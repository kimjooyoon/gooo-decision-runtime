package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type source struct {
	ID, Group, Form, Split, Source, SourceSHA, PlanSHA string
	Document                                           pathplan.Document
	V4, V5                                             [][384]float32
	V4SHA, V5SHA                                       string
	Acceptable                                         uint64
	Contexts                                           []pathplan.SemanticBranchContext
}
type prepared struct {
	source
	plan  *pathplan.PreparedPlan
	V6    [][384]float32
	V6SHA string
}
type fitRecord struct {
	FeatureVersion, ModelSHA, Fingerprint, TrainingSHA string
	ArtifactBytes                                      int
	TrainingNS, MedianNS, P95NS                        int64
	FirstLoss, LastPreUpdateLoss                       float64
}
type group struct{ Rows, Valid, OutputPassed, OutputTotal, ConditionPassed, ConditionTotal int }
type searchGroup struct {
	Programs, Complete, Attempts, ModelCalls, FeedbackCalls int
	NS                                                      int64
}
type report struct {
	Schema, Producer, DatasetSHA, Go, OS, Arch                                           string
	Sources, TrainingRows, Fits, Judgments, Searches, SearchModelCalls, NativeExecutions int
	Parameters, WeightBytes                                                              int
	Options                                                                              flowdecision.FitOptions
	Models                                                                               map[string]fitRecord
	Groups                                                                               map[string]group
	SearchGroups                                                                         map[string]searchGroup
	PairedDistributionMatches                                                            map[string]int
	BaselineReportSHA                                                                    string
}
type selected struct {
	Mask                   uint16
	GoooBody, GoSHA, Error string
	Outputs                []pathplan.TestResult
	Conditions             []pathplan.ConditionResult
	Acceptable             bool
}
type judgment struct {
	ID, Group, Form, Split, Model, SourceSHA, InputSHA string
	Acceptable                                         uint64
	Prediction                                         flowdecision.Prediction
	NS                                                 int64
	Selected                                           selected
}
type searchRow struct {
	ID, Form, Split, Mode string
	NS                    int64
	Result                pathplan.SearchResult
	Progress              []pathplan.ConditionProgress
	Feedback              []pathplan.ConditionRanking
	GoooBody, Error       string
}

func load(root string) ([]prepared, string) {
	raw, digest := gzipBytes(filepath.Join(root, "records.jsonl.gz"))
	require(digest == "05fa80975f0e6ea56e0e653fc1207d5894805957cd7a4a54091b67d0e1a7ed40", "exact original dataset")
	rows := decodeLines[source](raw)
	require(len(rows) == 162, "complete frozen source collection")
	var result []prepared
	counts := map[string]int{}
	ids := map[string]bool{}
	for _, row := range rows {
		require(!ids[row.ID] && hash([]byte(row.Source)) == row.SourceSHA, "unique exact source")
		ids[row.ID] = true
		plan, err := row.Document.Prepare()
		must(err)
		require(plan.PlanSHA256() == row.PlanSHA && len(row.Document.Plan.Decisions) == 2, "source plan")
		require(len(row.V4) == 2 && len(row.V5) == 2 && featureSHA(row.V4, 384) == row.V4SHA && featureSHA(row.V5, 384) == row.V5SHA, "frozen inputs")
		require(row.Acceptable != 0 && row.Acceptable>>4 == 0, "candidate labels")
		if row.Split == "future_train" {
			require(row.Form == "direct", "only direct training")
		}
		counts[row.Split]++
		input := relational(row)
		result = append(result, prepared{source: row, plan: plan, V6: input, V6SHA: featureSHA(input, 384)})
	}
	require(counts["future_train"] == 16 && counts["representation_transfer"] == 64 && counts["constant_transfer"] == 80 && counts["control"] == 2, "fixed split")
	return result, digest
}

func relational(row source) [][384]float32 {
	require(len(row.Contexts) == len(row.V5), "frozen source contexts")
	input := slices.Clone(row.V5)
	for i, view := range row.Contexts {
		if view.Normalized {
			must(decision.RelationalFlowFeaturesInto(row.V5[i], view.Flow, &input[i]))
		}
	}
	return input
}

func training(rows []prepared) []flowdecision.Sample {
	var result []flowdecision.Sample
	for _, row := range rows {
		if row.Split != "future_train" {
			continue
		}
		result = append(result, flowdecision.Sample{Inputs: slices.Clone(row.V6), Masks: []uint16{0, 1, 2, 3}, Acceptable: row.Acceptable})
	}
	return result
}

func evaluate(ctx context.Context, s prepared, mask uint16) selected {
	r := selected{Mask: mask}
	choices := map[string]string{}
	for i, c := range s.Document.Plan.Decisions {
		choices[c.ID] = c.Options[mask>>i&1].Label
	}
	b, err := s.plan.Compile(choices)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.GoooBody, r.GoSHA = b.GoooBody(), hash([]byte(b.GoSource()))
	r.Conditions, err = s.plan.CheckDeclaredConditions(ctx, choices)
	must(err)
	r.Acceptable = true
	for _, test := range s.Document.TestCases {
		v, err := b.Evaluate(test.Input)
		must(err)
		passed := v.Int == test.Expected
		r.Outputs = append(r.Outputs, pathplan.TestResult{Input: test.Input, Expected: test.Expected, Actual: v.Int, Passed: passed})
		r.Acceptable = r.Acceptable && passed
	}
	for _, c := range r.Conditions {
		r.Acceptable = r.Acceptable && c.Passed
	}
	return r
}

func judge(ctx context.Context, rows []prepared, m *flowdecision.Model, name string, f *os.File, r *report) {
	var times []int64
	byID := map[string]flowdecision.Prediction{}
	direct := map[string]flowdecision.Prediction{}
	for _, s := range rows {
		input, sha := s.V6, s.V6SHA
		var work flowdecision.Workspace
		var p flowdecision.Prediction
		start := time.Now()
		must(m.PredictInto(input, []uint16{0, 1, 2, 3}, &work, &p))
		ns := time.Since(start).Nanoseconds()
		// Construct/evaluate this exact selection before making another prediction.
		body := evaluate(ctx, s, p.Selected)
		require(body.Acceptable == (s.Acceptable>>p.Selected&1 != 0), "frozen acceptance agrees")
		appendRow(f, judgment{s.ID, s.Group, s.Form, s.Split, name, s.SourceSHA, sha, s.Acceptable, p, ns, body})
		byID[s.ID] = p
		if s.Form == "direct" {
			direct[s.Group] = p
		}
		for _, key := range []string{name + "/" + s.Split, name + "/form/" + s.Form} {
			g := r.Groups[key]
			g.Rows++
			if body.Acceptable {
				g.Valid++
			}
			for _, o := range body.Outputs {
				g.OutputTotal++
				if o.Passed {
					g.OutputPassed++
				}
			}
			for _, c := range body.Conditions {
				g.ConditionTotal++
				if c.Passed {
					g.ConditionPassed++
				}
			}
			r.Groups[key] = g
		}
		r.Judgments++
		times = append(times, ns)
	}
	for _, s := range rows {
		if s.Form != "direct" && s.Split != "control" && byID[s.ID] == direct[s.Group] {
			r.PairedDistributionMatches[name]++
		}
	}
	slices.Sort(times)
	fit := r.Models[name]
	fit.MedianNS = times[len(times)/2]
	fit.P95NS = times[(len(times)*95-1)/100]
	r.Models[name] = fit
}

func search(ctx context.Context, rows []prepared, m *flowdecision.Model, f *os.File, r *report) {
	names := [2]string{"v6_initial", "v6_feedback"}
	for _, s := range rows {
		for mode, name := range names {
			rounds := 0
			if mode == 1 {
				rounds = 4
			}
			start := time.Now()
			result, body, progress, feedback, err := s.plan.SearchFlowBatches(ctx, m, s.Document.TestCases, 4, 1, "", rounds)
			row := searchRow{ID: s.ID, Form: s.Form, Split: s.Split, Mode: name, NS: time.Since(start).Nanoseconds(), Result: result, Progress: progress, Feedback: feedback}
			if err != nil {
				row.Error = err.Error()
			}
			if body != nil {
				row.GoooBody = body.GoooBody()
			}
			require(len(progress) > 0, "runtime input")
			for i, f := range s.V6 {
				require(progress[0].Ranking.FeatureSHA[i] == choiceSHA(f), "exact runtime input")
			}
			seen := map[uint16]bool{}
			for _, a := range result.Attempts {
				require(!seen[a.Mask], "candidate reexecution")
				seen[a.Mask] = true
			}
			appendRow(f, row)
			g := r.SearchGroups[name+"/"+s.Split]
			g.Programs++
			if result.Status == "TRAINING_COMPLETE" {
				g.Complete++
			}
			g.Attempts += len(result.Attempts)
			g.ModelCalls += result.Selection.ModelCalls
			g.NS += row.NS
			for _, fb := range feedback {
				g.FeedbackCalls += fb.Calls
			}
			r.SearchGroups[name+"/"+s.Split] = g
			r.Searches++
			r.SearchModelCalls += result.Selection.ModelCalls
		}
	}
}

func fit(ctx context.Context, rows []prepared, out string, r *report) *flowdecision.Model {
	samples := training(rows)
	require(len(samples) == 16, "only16 initial training rows")
	trainingBytes := encoded(samples)
	saveRaw(out, "training-v6.json", trainingBytes)
	start := time.Now()
	m, history, err := flowdecision.FitForFeatures(ctx, samples, r.Options, decision.RelationalFlowFeatureVersion)
	ns := time.Since(start).Nanoseconds()
	must(err)
	r.Fits++
	require(len(history) == 400, "fixed epochs")
	b, err := m.Marshal()
	must(err)
	saveRaw(out, "model-v6.json", b)
	save(out, "history-v6.json", history)
	loaded, err := flowdecision.Decode(b)
	must(err)
	require(loaded.Fingerprint() == m.Fingerprint() && loaded.Weights() == m.Weights(), "saved model identity")
	r.Models["v6"] = fitRecord{FeatureVersion: decision.RelationalFlowFeatureVersion, ModelSHA: hash(b), Fingerprint: loaded.Fingerprint(), TrainingSHA: hash(trainingBytes), ArtifactBytes: len(b), TrainingNS: ns, FirstLoss: history[0].Loss, LastPreUpdateLoss: history[399].Loss}
	return loaded
}

func baseline(root string) ([]byte, string) {
	raw, err := os.ReadFile(filepath.Join(root, "report.json"))
	must(err)
	sha := hash(raw)
	require(sha == "83ab9f2b87845e519e9f7d0d6bbc474d2dc87fcf568df91be972460412e9f4ac", "immutable prior baseline report")
	return raw, sha
}

func main() {
	require(len(os.Args) == 4, "usage: relational-flow-learning FROZEN_DATASET BASELINE_RESULT_DIRECTORY NEW_OUTPUT_DIRECTORY")
	revision := producer()
	rows, sha := load(os.Args[1])
	prior, baselineSHA := baseline(os.Args[2])
	out := os.Args[3]
	must(os.Mkdir(out, 0700))
	save(out, "started.json", map[string]string{"producer": revision, "time": time.Now().UTC().Format(time.RFC3339)})
	saveRaw(out, "baseline-report.json", prior)
	r := report{Schema: "gooo/relational-flow-learning/v1", Producer: revision, DatasetSHA: sha, BaselineReportSHA: baselineSHA, Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH, Sources: 162, TrainingRows: 16, Parameters: flowdecision.ParameterCount, WeightBytes: flowdecision.ParameterCount * 4, Options: flowdecision.FitOptions{Epochs: 400, LearningRate: .25, L2: .0001, Seed: 17}, Models: map[string]fitRecord{}, Groups: map[string]group{}, SearchGroups: map[string]searchGroup{}, PairedDistributionMatches: map[string]int{}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	model := fit(ctx, rows, out, &r)
	f := rowFile(out, "judgments.jsonl")
	judge(ctx, rows, model, "v6", f, &r)
	must(f.Close())
	f = rowFile(out, "searches.jsonl")
	search(ctx, rows, model, f, &r)
	must(f.Close())
	require(r.Fits == 1 && r.Judgments == 162 && r.Searches == 324, "whole fixed protocol")
	save(out, "report.json", r)
	save(out, "completed.json", map[string]any{"fits": r.Fits, "judgments": r.Judgments, "searches": r.Searches, "time": time.Now().UTC().Format(time.RFC3339)})
	must(json.NewEncoder(os.Stdout).Encode(map[string]any{"fits": r.Fits, "judgments": r.Judgments, "searches": r.Searches}))
	fmt.Fprintln(os.Stdout, "No previous experiment was rerun.")
}
