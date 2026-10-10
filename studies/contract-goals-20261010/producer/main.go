package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
	"github.com/kimjooyoon/meta-ontology-go/internal/bodycodegen"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func save(root, name string, v any) {
	f, e := os.OpenFile(filepath.Join(root, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(e)
	_, e = f.Write(r.Encode(v))
	must(e)
	must(f.Close())
}
func newRows(root, name string) *os.File {
	f, e := os.OpenFile(filepath.Join(root, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(e)
	return f
}
func fileSHA(path string) string { b, e := os.ReadFile(path); must(e); return r.Hash(b) }
func identity() string {
	b, ok := debug.ReadBuildInfo()
	require(ok && b.GoVersion == "go1.27.2", "Go1.27.2 producer required")
	rev := ""
	clean := false
	for _, s := range b.Settings {
		if s.Key == "vcs.revision" {
			rev = s.Value
		}
		if s.Key == "vcs.modified" {
			clean = s.Value == "false"
		}
	}
	require(clean && len(rev) == 40, "clean committed producer required")
	return rev
}

func collect(ctx context.Context, s r.Spec) (r.Source, *pathplan.PreparedPlan) {
	source := r.Gooo(s)
	d, err := bodycodegen.DecodeSourcePathDocument(ctx, s.ID+".gooo", []byte(source), "Choose", nil)
	must(err)
	p, err := d.Prepare()
	must(err)
	input, err := p.InitialContractInput(d.TestCases)
	must(err)
	x := r.Source{Spec: s, Gooo: source, SourceSHA: r.Hash([]byte(source)), PlanSHA: p.PlanSHA256(), CaseSHA: input.CaseSHA256(), Document: d}
	require(len(d.Plan.Decisions) == 2, "two source choices")
	for i, c := range d.Plan.Decisions {
		var row [384]float32
		must(input.RelationalSourceFeaturesInto(c.ID, &row))
		x.Inputs = append(x.Inputs, row)
		x.InputSHA[i] = r.FeatureSHA(row)
	}
	for i := range input.CaseCount() {
		row, err := input.CaseFeatures(i)
		must(err)
		x.CaseRows = append(x.CaseRows, row)
	}
	for mask := range 4 {
		choices := map[string]string{}
		for i, c := range d.Plan.Decisions {
			choices[c.ID] = c.Options[mask>>i&1].Label
		}
		body, err := p.Compile(choices)
		must(err)
		c := r.Candidate{Mask: uint16(mask), GoooSource: body.GoooSource(), GoSource: body.GoSource(), Acceptable: true}
		for _, test := range d.TestCases {
			v, err := body.Evaluate(test.Input)
			must(err)
			passed := v.Int == test.Expected
			c.Outputs = append(c.Outputs, pathplan.TestResult{Input: test.Input, Expected: test.Expected, Actual: v.Int, Passed: passed})
			c.Acceptable = c.Acceptable && passed
		}
		c.Conditions, err = p.CheckConditions(ctx, choices, d.Plan.ConditionCases)
		must(err)
		for _, v := range c.Conditions {
			c.Acceptable = c.Acceptable && v.Passed
		}
		if c.Acceptable {
			x.Acceptable |= 1 << mask
		}
		x.Candidates = append(x.Candidates, c)
	}
	require((x.Acceptable == 0) == (s.Split == "contradiction"), "declared satisfiability "+s.ID)
	return x, p
}

func search(ctx context.Context, x r.Source, p *pathplan.PreparedPlan, m *contractdecision.Model, mode string) r.Search {
	start := time.Now()
	s, err := p.NewContractSession(ctx, m, x.Document.TestCases)
	must(err)
	row := r.Search{ID: x.Spec.ID, Split: x.Spec.Split, Family: x.Spec.Family, Mode: mode, Ranking: s.Ranking()}
	initial, err := s.Observe()
	must(err)
	row.Progress = append(row.Progress, initial)
	for range 4 {
		progress, body, err := s.Advance(ctx, 1)
		if err != nil && !errors.Is(err, pathplan.ErrNoConditionCandidate) && !errors.Is(err, pathplan.ErrNoTypedCandidate) {
			panic(err)
		}
		require(progress.Schema != "" && len(progress.NewAttempts) == 1, "one immediate committed candidate")
		row.Progress = append(row.Progress, progress)
		if body != nil {
			row.GoooBody, row.GoSource = body.GoooBody(), body.GoSource()
		}
		if progress.Status == "TRAINING_COMPLETE" || progress.Exhausted {
			break
		}
	}
	row.NS = time.Since(start).Nanoseconds()
	return row
}

func summarize(report *r.Report, row r.Search) {
	first := row.Progress[1].NewAttempts[0]
	last := row.Progress[len(row.Progress)-1]
	for _, key := range []string{row.Mode + "/" + row.Split, row.Mode + "/family-" + row.Family} {
		g := report.Groups[key]
		g.Programs++
		g.Attempts += last.Attempted
		g.ModelCalls += row.Ranking.Calls
		g.NS += row.NS
		g.PredictNS += row.Ranking.PredictNS
		if first.Passed == first.Total && pathplan.ConditionsPassed(first.Conditions) {
			g.FirstValid++
		}
		if last.Status == "TRAINING_COMPLETE" {
			g.Complete++
		}
		g.FirstOutputPassed += first.Passed
		g.FirstOutputTotal += first.Total
		g.FirstConditionTotal += len(first.Conditions)
		for _, c := range first.Conditions {
			if c.Passed {
				g.FirstConditionPassed++
			}
		}
		report.Groups[key] = g
	}
	report.Searches++
	report.ModelCalls += row.Ranking.Calls
}

func main() {
	require(len(os.Args) == 3, "usage: contract-goals COMPILER_CHECKOUT FRESH_OUTPUT")
	producer := identity()
	b, err := exec.Command("git", "-C", os.Args[1], "rev-parse", "HEAD").Output()
	must(err)
	require(strings.TrimSpace(string(b)) == r.Compiler, "compiler revision")
	b, err = exec.Command("git", "-C", os.Args[1], "status", "--porcelain").Output()
	must(err)
	require(len(b) == 0, "clean compiler")
	out := os.Args[2]
	must(os.Mkdir(out, 0700))
	save(out, "started.json", map[string]string{"producer": producer, "compiler": r.Compiler, "time": time.Now().UTC().Format(time.RFC3339Nano)})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	report := r.Report{Schema: "gooo/contract-goals-study/v1", Producer: producer, Compiler: r.Compiler, Parameters: contractdecision.ParameterCount, WeightBytes: contractdecision.ParameterCount * 4, Options: contractdecision.FitOptions{Epochs: 600, LearningRate: 0.3, L2: 0.0001, Seed: 17}, Groups: map[string]r.Group{}}
	f := newRows(out, "sources.jsonl")
	var rows []r.Source
	var plans []*pathplan.PreparedPlan
	var training []r.Training
	var samples []contractdecision.Sample
	for _, spec := range r.Specs() {
		x, p := collect(ctx, spec)
		must(json.NewEncoder(f).Encode(x))
		rows = append(rows, x)
		plans = append(plans, p)
		report.Sources++
		report.OracleCandidates += 4
		report.OracleCompileCalls += 8
		report.OracleOutputEvaluations += 4 * len(x.Document.TestCases)
		report.OracleConditionObservations += 4 * len(x.Document.Plan.ConditionCases)
		if spec.Split == "train" {
			input, err := p.InitialContractInput(x.Document.TestCases)
			must(err)
			masks := []uint16{0, 1, 2, 3}
			samples = append(samples, contractdecision.Sample{Inputs: x.Inputs, Cases: input, Masks: masks, Acceptable: x.Acceptable})
			training = append(training, r.Training{ID: spec.ID, PlanSHA: x.PlanSHA, CaseSHA: x.CaseSHA, Inputs: x.Inputs, Cases: x.CaseRows, Masks: masks, Acceptable: x.Acceptable})
		}
	}
	must(f.Close())
	report.RecordsSHA = fileSHA(filepath.Join(out, "sources.jsonl"))
	report.TrainingRows = len(samples)
	require(report.Sources == 196 && report.TrainingRows == 24 && report.OracleOutputEvaluations == 8192, "fixed collection scope")
	save(out, "training.json", training)
	report.TrainingSHA = fileSHA(filepath.Join(out, "training.json"))
	start := time.Now()
	model, history, err := contractdecision.Fit(ctx, samples, report.Options)
	must(err)
	report.TrainingNS = time.Since(start).Nanoseconds()
	report.Fits = 1
	raw, err := model.Marshal()
	must(err)
	mf := newRows(out, "model.json")
	_, err = mf.Write(raw)
	must(err)
	must(mf.Close())
	report.ModelSHA, report.Fingerprint, report.ArtifactBytes = r.Hash(raw), model.Fingerprint(), len(raw)
	report.FirstLoss, report.LastLoss = history[0].Loss, history[len(history)-1].Loss
	save(out, "history.json", history)
	f = newRows(out, "searches.jsonl")
	var times []int64
	for i, x := range rows {
		for mode := range 2 {
			m := model
			name := "model"
			if mode == 0 {
				m = nil
				name = "deterministic"
			}
			row := search(ctx, x, plans[i], m, name)
			must(json.NewEncoder(f).Encode(row))
			summarize(&report, row)
			if mode == 1 {
				times = append(times, row.Ranking.PredictNS)
			}
		}
	}
	must(f.Close())
	require(report.Searches == 392 && report.ModelCalls == 196, "fixed inference scope")
	slices.Sort(times)
	report.PredictionMedianNS = times[len(times)/2]
	report.PredictionP95NS = times[(len(times)*95+99)/100-1]
	save(out, "report.json", report)
	save(out, "completed.json", map[string]any{"time": time.Now().UTC().Format(time.RFC3339Nano), "sources": report.Sources, "fits": report.Fits, "model_calls": report.ModelCalls})
	fmt.Printf("%d Gooo documents, %d training rows, one CPU fit, %d model calls, %d finite searches.\n", report.Sources, report.TrainingRows, report.ModelCalls, report.Searches)
}
