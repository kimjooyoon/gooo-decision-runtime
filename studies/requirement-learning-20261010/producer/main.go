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
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/requirement-learning-20261010/record"
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

func collect(ctx context.Context, frozen r.FrozenSource) (r.Source, *pathplan.PreparedPlan) {
	s, source := frozen.Spec, frozen.Gooo
	require(source == r.Gooo(s), "frozen source differs from protocol")
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
	for i := range input.ConditionCount() {
		row, err := input.ConditionFeatures(i)
		must(err)
		x.ConditionRows = append(x.ConditionRows, row)
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

func search(ctx context.Context, x r.Source, p *pathplan.PreparedPlan, choice *contractdecision.ChoiceModel, requirements *contractdecision.RequirementModel, mode string) r.Search {
	start := time.Now()
	var s *pathplan.ContractSession
	var err error
	if requirements != nil {
		s, err = p.NewRequirementContractSession(ctx, requirements, x.Document.TestCases)
	} else if choice != nil {
		s, err = p.NewChoiceContractSession(ctx, choice, x.Document.TestCases)
	} else {
		s, err = p.NewContractSession(ctx, nil, x.Document.TestCases)
	}
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

func summarize(report *Report, row r.Search) {
	first := row.Progress[1].NewAttempts[0]
	last := row.Progress[len(row.Progress)-1]
	keys := []string{row.Mode + "/all", row.Mode + "/" + row.Split, row.Mode + "/family-" + row.Family}
	if row.Split != "train" && row.Split != "contradiction" {
		keys = append(keys, row.Mode+"/heldout_satisfiable")
	}
	for _, key := range keys {
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
	require(len(os.Args) == 4, "usage: requirement-learning COMPILER_CHECKOUT FROZEN_CORPUS_GZIP FRESH_OUTPUT")
	producer := identity()
	b, err := exec.Command("git", "-C", os.Args[1], "rev-parse", "HEAD").Output()
	must(err)
	require(strings.TrimSpace(string(b)) == r.Compiler, "compiler revision")
	b, err = exec.Command("git", "-C", os.Args[1], "status", "--porcelain").Output()
	must(err)
	require(len(b) == 0, "clean compiler")
	out := os.Args[3]
	must(os.Mkdir(out, 0700))
	save(out, "started.json", map[string]string{"producer": producer, "compiler": r.Compiler, "time": time.Now().UTC().Format(time.RFC3339Nano)})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	report := Report{Schema: "gooo/requirement-learning-study/v1", Producer: producer, Compiler: r.Compiler, CorpusSHA: r.CorpusSHA, Options: contractdecision.FitOptions{Epochs: 2000, LearningRate: .3, L2: .0001, Seed: 17}, Groups: map[string]r.Group{}, Models: map[string]ModelReport{}, InputAudits: map[string]*contractdecision.InputAudit{}}
	f := newRows(out, "sources.jsonl")
	var rows []r.Source
	var plans []*pathplan.PreparedPlan
	var training []r.Training
	var samples []contractdecision.Sample
	var requirements []contractdecision.RequirementSample
	var auditSamples []contractdecision.Sample
	var auditRequirements []contractdecision.RequirementSample
	for _, frozen := range frozenSources(os.Args[2]) {
		spec := frozen.Spec
		x, p := collect(ctx, frozen)
		must(json.NewEncoder(f).Encode(x))
		rows = append(rows, x)
		plans = append(plans, p)
		report.Sources++
		report.OracleCandidates += 4
		report.OracleCompileCalls += 8
		report.OracleOutputEvaluations += 4 * len(x.Document.TestCases)
		report.OracleConditionObservations += 4 * len(x.Document.Plan.ConditionCases)
		input, err := p.InitialContractInput(x.Document.TestCases)
		must(err)
		masks := []uint16{0, 1, 2, 3}
		sample := contractdecision.Sample{Inputs: x.Inputs, Cases: input, Masks: masks, Acceptable: x.Acceptable}
		required := contractdecision.RequirementSample{Sample: sample, Conditions: input}
		if x.Acceptable != 0 {
			auditSamples = append(auditSamples, sample)
			auditRequirements = append(auditRequirements, required)
			report.AuditedIDs = append(report.AuditedIDs, spec.ID)
		}
		if spec.Split == "train" {
			samples = append(samples, sample)
			requirements = append(requirements, required)
			training = append(training, r.Training{ID: spec.ID, PlanSHA: x.PlanSHA, CaseSHA: x.CaseSHA, Inputs: x.Inputs, Cases: x.CaseRows, Conditions: x.ConditionRows, Masks: masks, Acceptable: x.Acceptable})
		}
	}
	must(f.Close())
	report.RecordsSHA = fileSHA(filepath.Join(out, "sources.jsonl"))
	report.TrainingRows = len(samples)
	require(report.Sources == 172 && report.TrainingRows == 32 && report.OracleOutputEvaluations == 5504 && report.OracleConditionObservations == 4032, "fixed collection scope")
	report.InputAudits["choice"], err = contractdecision.AuditInputs(ctx, auditSamples)
	must(err)
	report.InputAudits["requirements"], err = contractdecision.AuditRequirements(ctx, auditRequirements)
	must(err)
	require(len(report.AuditedIDs) == 168, "satisfiable-only input audit")
	save(out, "training.json", training)
	report.TrainingSHA = fileSHA(filepath.Join(out, "training.json"))
	start := time.Now()
	baseline, history, err := contractdecision.FitChoiceConditioned(ctx, samples, report.Options, contractdecision.MeanPooling)
	must(err)
	recordModel(out, "choice", baseline, history, time.Since(start).Nanoseconds(), contractdecision.ChoiceParameterCount, &report)
	start = time.Now()
	model, history, err := contractdecision.FitRequirementConditioned(ctx, requirements, report.Options, contractdecision.MeanPooling)
	must(err)
	recordModel(out, "requirements", model, history, time.Since(start).Nanoseconds(), contractdecision.RequirementParameterCount, &report)
	report.Fits = 2
	f = newRows(out, "searches.jsonl")
	times := map[string][]int64{}
	for i, x := range rows {
		for mode := range 3 {
			choice, requirements := baseline, model
			name := "requirements"
			if mode == 0 {
				choice, requirements = nil, nil
				name = "deterministic"
			}
			if mode == 1 {
				requirements = nil
				name = "choice"
			}
			row := search(ctx, x, plans[i], choice, requirements, name)
			must(json.NewEncoder(f).Encode(row))
			summarize(&report, row)
			if mode != 0 {
				times[name] = append(times[name], row.Ranking.PredictNS)
			}
		}
	}
	must(f.Close())
	require(report.Searches == 516 && report.ModelCalls == 344, "fixed inference scope")
	for name, latencies := range times {
		slices.Sort(latencies)
		summary := report.Models[name]
		summary.PredictionMedianNS = latencies[len(latencies)/2]
		summary.PredictionP95NS = latencies[(len(latencies)*95+99)/100-1]
		report.Models[name] = summary
	}
	save(out, "report.json", report)
	save(out, "completed.json", map[string]any{"time": time.Now().UTC().Format(time.RFC3339Nano), "sources": report.Sources, "fits": report.Fits, "model_calls": report.ModelCalls})
	fmt.Printf("%d Gooo documents, %d training rows, two CPU fits, %d model calls, %d finite searches.\n", report.Sources, report.TrainingRows, report.ModelCalls, report.Searches)
}
