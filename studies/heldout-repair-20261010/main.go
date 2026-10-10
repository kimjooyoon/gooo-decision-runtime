package main

import (
	"context"
	"encoding/json"
	"math/bits"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type source struct {
	ID, Group, Form, Split, Source, SourceSHA, PlanSHA string
	Document                                           pathplan.Document
	Acceptable                                         uint64
}
type initial struct {
	ID, SourceSHA, InputSHA string
	Acceptable              uint64
	Prediction              flowdecision.Prediction
	Explanation             flowdecision.Explanation
}
type modelRecord struct {
	Name, SHA, Fingerprint, InitialSHA string
	Model                              *flowdecision.Model `json:"-"`
	Initial                            map[string]initial  `json:"-"`
}
type observation struct {
	ID, SourceSHA, PlanSHA, CaseSHA, InputSHA string
	Mask                                      uint16
	NS                                        int64
	ConditionPresent, OutputPresent           bool
	Condition                                 pathplan.ConditionFailure
	Output                                    pathplan.OutputFailure
	Inputs                                    [][384]float32
}
type outcome struct {
	Mask        uint16
	Body, GoSHA string
	Outputs     []pathplan.TestResult
	Conditions  []pathplan.ConditionResult
	Valid       bool
}
type judgment struct {
	ID, Split, Form, Model, InputSHA            string
	ObservedMask, InitialRemaining              uint16
	InitialRemainingValid, RawValid, RawRepeats bool
	Prediction                                  flowdecision.Prediction
	Explanation                                 flowdecision.Explanation
	Selected                                    outcome
	NS                                          int64
}
type group struct {
	Rows, RawValid, RawRepeats, NextValid, InitialRemainingValid int
	FeedbackImproved, FeedbackRegressed, FeedbackUnchanged       int
	OutputPassed, OutputTotal, ConditionPassed, ConditionTotal   int
}
type report struct {
	Schema, Producer, DatasetSHA, Go, OS, Arch                                     string
	Sources, Observations, ModelCalls, SelectedEvaluations, Fits, NativeExecutions int
	ObservationNS                                                                  int64
	Models                                                                         []modelRecord
	Groups                                                                         map[string]group
	MedianNS, P95NS                                                                map[string]int64
}

func loadSources(root string) ([]source, string) {
	b, sha := gzipBytes(filepath.Join(root, "records.jsonl.gz"))
	require(sha == "05fa80975f0e6ea56e0e653fc1207d5894805957cd7a4a54091b67d0e1a7ed40", "frozen source collection")
	all := decodeLines[source](b)
	require(len(all) == 162, "complete collection")
	var rows []source
	counts := map[string]int{}
	for _, s := range all {
		if s.Split == "future_train" {
			continue
		}
		require(hash([]byte(s.Source)) == s.SourceSHA && bits.OnesCount64(s.Acceptable) == 1 && s.Acceptable < 16, "source and one accepted mask")
		require(len(s.Document.Plan.Decisions) == 2, "two declared choices")
		rows = append(rows, s)
		counts[s.Split]++
	}
	require(len(rows) == 146 && counts["representation_transfer"] == 64 && counts["constant_transfer"] == 80 && counts["control"] == 2, "evaluation sources only")
	return rows, sha
}

func loadModel(root, name, modelFile, modelSHA, initialSHA string) modelRecord {
	b, err := os.ReadFile(filepath.Join(root, modelFile))
	must(err)
	require(hash(b) == modelSHA, "frozen model bytes")
	m, err := flowdecision.Decode(b)
	must(err)
	b, sha := gzipBytes(filepath.Join(root, "judgments.jsonl.gz"))
	require(sha == initialSHA, "saved initial predictions")
	r := modelRecord{Name: name, SHA: modelSHA, Fingerprint: m.Fingerprint(), InitialSHA: sha, Model: m, Initial: map[string]initial{}}
	for _, row := range decodeLines[initial](b) {
		_, exists := r.Initial[row.ID]
		require(!exists, "unique initial record")
		r.Initial[row.ID] = row
	}
	require(len(r.Initial) == 162, "all saved first judgments")
	return r
}

func choices(s source, mask uint16) map[string]string {
	m := map[string]string{}
	for i, c := range s.Document.Plan.Decisions {
		m[c.ID] = c.Options[mask>>i&1].Label
	}
	return m
}

func observe(ctx context.Context, s source, p *pathplan.PreparedPlan, mask uint16) observation {
	start := time.Now()
	input, err := p.ObserveExecutionInput(ctx, choices(s, mask), s.Document.TestCases)
	ns := time.Since(start).Nanoseconds()
	must(err)
	o := observation{ID: s.ID, SourceSHA: s.SourceSHA, PlanSHA: input.PlanSHA256(), CaseSHA: input.CaseSHA256(), Mask: mask, NS: ns, Inputs: make([][384]float32, 2)}
	o.Condition, o.ConditionPresent = input.Failure()
	o.Output, o.OutputPresent = input.OutputFailure()
	require(o.ConditionPresent || o.OutputPresent, "forced rejected start actually fails")
	for i, c := range s.Document.Plan.Decisions {
		must(input.ExecutionRelationalFlowFeaturesInto(c.ID, &o.Inputs[i]))
	}
	o.InputSHA = featureSHA(o.Inputs, 384)
	return o
}

// Choose from all three untried configurations. Ties use ascending mask order.
// The acceptable set is deliberately absent from this decision function.
func next(scores [64]float64, rejected uint16) uint16 {
	best := uint16(4)
	for mask := range uint16(4) {
		if mask != rejected && (best == 4 || scores[mask] > scores[best]) {
			best = mask
		}
	}
	return best
}

func evaluate(ctx context.Context, s source, p *pathplan.PreparedPlan, mask uint16) outcome {
	b, err := p.Compile(choices(s, mask))
	must(err)
	o := outcome{Mask: mask, Body: b.GoooBody(), GoSHA: hash([]byte(b.GoSource())), Valid: true}
	o.Conditions, err = p.CheckDeclaredConditions(ctx, choices(s, mask))
	must(err)
	for _, c := range o.Conditions {
		o.Valid = o.Valid && c.Passed
	}
	for _, c := range s.Document.TestCases {
		v, err := b.Evaluate(c.Input)
		must(err)
		pass := v.Int == c.Expected
		o.Outputs = append(o.Outputs, pathplan.TestResult{Input: c.Input, Expected: c.Expected, Actual: v.Int, Passed: pass})
		o.Valid = o.Valid && pass
	}
	require(o.Valid == (s.Acceptable>>mask&1 != 0), "selected actual body agrees with frozen finite acceptance")
	return o
}

func judge(ctx context.Context, s source, p *pathplan.PreparedPlan, o observation, model modelRecord) judgment {
	var work flowdecision.Workspace
	j := judgment{ID: s.ID, Split: s.Split, Form: s.Form, Model: model.Name, InputSHA: o.InputSHA, ObservedMask: o.Mask}
	start := time.Now()
	must(model.Model.ExplainInto(o.Inputs, []uint16{0, 1, 2, 3}, &work, &j.Prediction, &j.Explanation))
	j.NS = time.Since(start).Nanoseconds()
	// Execute this exact untried selection before calling the next model.
	j.Selected = evaluate(ctx, s, p, next(j.Explanation.CandidateScores, o.Mask))
	j.RawValid, j.RawRepeats = s.Acceptable>>j.Prediction.Selected&1 != 0, j.Prediction.Selected == o.Mask
	prior, ok := model.Initial[s.ID]
	require(ok && prior.SourceSHA == s.SourceSHA && prior.Acceptable == s.Acceptable, "same initial source identity")
	j.InitialRemaining = next(prior.Explanation.CandidateScores, o.Mask)
	j.InitialRemainingValid = s.Acceptable>>j.InitialRemaining&1 != 0
	return j
}

func addGroup(g group, j judgment) group {
	g.Rows++
	if j.RawValid {
		g.RawValid++
	}
	if j.RawRepeats {
		g.RawRepeats++
	}
	if j.Selected.Valid {
		g.NextValid++
	}
	if j.InitialRemainingValid {
		g.InitialRemainingValid++
	}
	switch {
	case !j.InitialRemainingValid && j.Selected.Valid:
		g.FeedbackImproved++
	case j.InitialRemainingValid && !j.Selected.Valid:
		g.FeedbackRegressed++
	default:
		g.FeedbackUnchanged++
	}
	for _, o := range j.Selected.Outputs {
		g.OutputTotal++
		if o.Passed {
			g.OutputPassed++
		}
	}
	for _, c := range j.Selected.Conditions {
		g.ConditionTotal++
		if c.Passed {
			g.ConditionPassed++
		}
	}
	return g
}

func main() {
	require(len(os.Args) == 5, "usage: heldout-repair FROZEN_SOURCE LEAKY_RESULT FEEDBACK_RESULT NEW_OUTPUT")
	revision := producer()
	rows, dataset := loadSources(os.Args[1])
	models := []modelRecord{
		loadModel(os.Args[2], "initial_trained", "model-leaky_v6.json", "d5f9c4ffb204852f4e1b3b1b1c491c7d1fb0753d0df2c1e0f0ffead80d9ac47e", "4ad76a785a122b478128b88edce4a056b90e9f867c7e7d631d5a3cf05de61acf"),
		loadModel(os.Args[3], "observation_trained", "model-feedback_v6.json", "b81317ce44943549c58237eb9b87e797eca56c181a24086780f7f08a13d4ad59", "4aba2c54b83efd939fb3b2054ab03b87ed7bc6a44a640c743fbc2242a66242c8"),
	}
	out := os.Args[4]
	must(os.Mkdir(out, 0700))
	save(out, "started.json", map[string]string{"producer": revision, "time": time.Now().UTC().Format(time.RFC3339)})
	r := report{Schema: "gooo/heldout-repair/v1", Producer: revision, DatasetSHA: dataset, Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH, Sources: len(rows), Models: models, Groups: map[string]group{}, MedianNS: map[string]int64{}, P95NS: map[string]int64{}}
	observations, judgments := rowFile(out, "observations.jsonl"), rowFile(out, "judgments.jsonl")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	times := map[string][]int64{}
	for _, s := range rows {
		p, err := s.Document.Prepare()
		must(err)
		require(p.PlanSHA256() == s.PlanSHA, "source plan")
		for mask := range uint16(4) {
			if s.Acceptable>>mask&1 != 0 {
				continue
			}
			o := observe(ctx, s, p, mask)
			appendRow(observations, o)
			r.Observations++
			r.ObservationNS += o.NS
			for _, model := range models {
				j := judge(ctx, s, p, o, model)
				appendRow(judgments, j)
				r.ModelCalls++
				r.SelectedEvaluations++
				for _, key := range []string{model.Name + "/all", model.Name + "/" + s.Split, model.Name + "/form/" + s.Form} {
					r.Groups[key] = addGroup(r.Groups[key], j)
				}
				times[model.Name] = append(times[model.Name], j.NS)
			}
		}
	}
	must(observations.Close())
	must(judgments.Close())
	for name, ns := range times {
		slices.Sort(ns)
		r.MedianNS[name] = ns[len(ns)/2]
		r.P95NS[name] = ns[(len(ns)*95-1)/100]
	}
	require(r.Observations == 438 && r.ModelCalls == 876 && r.SelectedEvaluations == 876, "fixed protocol counts")
	save(out, "report.json", r)
	save(out, "completed.json", map[string]any{"observations": r.Observations, "model_calls": r.ModelCalls, "time": time.Now().UTC().Format(time.RFC3339)})
	must(json.NewEncoder(os.Stdout).Encode(r))
}
