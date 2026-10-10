package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-role-learning-20261011/record"
)

func main() {
	require(len(os.Args) == 4, "usage: canonical-role-study COMPILER_CHECKOUT FROZEN_CORPUS_GZIP FRESH_OUTPUT")
	producer := identity()
	head, err := exec.Command("git", "-C", os.Args[1], "rev-parse", "HEAD").Output()
	must(err)
	require(strings.TrimSpace(string(head)) == r.Compiler, "pinned compiler required")
	status, err := exec.Command("git", "-C", os.Args[1], "status", "--porcelain").Output()
	must(err)
	require(len(status) == 0, "clean compiler required")
	frozen := frozenSources(os.Args[2])
	out := os.Args[3]
	must(os.Mkdir(out, 0700))
	save(out, "started.json", map[string]string{"producer": producer, "compiler": r.Compiler, "sdk_base": r.SDKBase, "corpus": r.CorpusSHA, "time": time.Now().UTC().Format(time.RFC3339Nano)})
	phase := "collect"
	defer func() {
		if failure := recover(); failure != nil {
			save(out, "failure.json", map[string]string{"phase": phase, "error": fmt.Sprint(failure)})
			panic(failure)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	options := contractdecision.FitOptions{Epochs: 2000, LearningRate: .3, L2: .0001, Seed: 37}
	report := r.Report{Schema: "gooo/canonical-role-study/v1", Producer: producer, Compiler: r.Compiler, SDKBase: r.SDKBase, CorpusSHA: r.CorpusSHA, Options: options, Models: map[string]r.Model{}, Groups: map[string]r.Group{}}
	sourceFile := newRows(out, "sources.jsonl")
	defer sourceFile.Close()
	var sources []r.Source
	var prepared []*pathplan.PreparedPlan
	training := map[string][]contractdecision.InteractionRequirementSample{}
	var records []r.TrainingRecord
	for _, frozen := range frozen {
		x, p := collect(ctx, frozen)
		_, err := sourceFile.Write(r.Encode(x))
		must(err)
		sources = append(sources, x)
		prepared = append(prepared, p)
		if x.Spec.Split != "train" {
			continue
		}
		cmp, err := r.ProjectRole(x.Acceptable, 0)
		must(err)
		branch, err := r.ProjectRole(x.Acceptable, 1)
		must(err)
		records = append(records, r.TrainingRecord{ID: x.Spec.ID, PlanSHA: x.PlanSHA, CasesSHA: x.CasesSHA, CanonicalInputs: x.CanonicalInputs, Outputs: x.OutputRows, Conditions: x.ConditionRows, Joint: x.Acceptable, Comparison: cmp, Branches: branch})
		for _, role := range []struct {
			name       string
			acceptable uint64
		}{{"joint", x.Acceptable}, {"comparison", cmp}, {"branches", branch}} {
			training[role.name] = append(training[role.name], contractdecision.InteractionRequirementSample{Inputs: x.CanonicalInputs, Cases: r.Rows(x.OutputRows), Conditions: r.Rows(x.ConditionRows), Masks: []uint16{0, 1, 2, 3}, Acceptable: role.acceptable})
		}
	}
	require(len(sources) == 256 && len(records) == 64, "frozen populations")
	report.Sources, report.TrainingRows = len(sources), len(records)
	save(out, "training.json", records)
	models := map[string]predictor{}
	for _, name := range []string{"joint", "comparison", "branches"} {
		phase = "fit-" + name
		save(out, name+"-fit-started.json", map[string]any{"rows": len(training[name]), "options": options, "pooling": contractdecision.MeanPooling})
		start := time.Now()
		model, history, err := contractdecision.FitCanonicalInteractionRequirementConditioned(ctx, training[name], options, contractdecision.MeanPooling)
		ns := time.Since(start).Nanoseconds()
		save(out, "history-"+name+".json", history)
		if err != nil {
			save(out, name+"-fit-failure.json", map[string]any{"error": err.Error(), "fit_ns": ns})
			panic(err)
		}
		raw, info := recordModel(out, name, model, history, ns)
		report.Models[name] = info
		models[name], err = contractdecision.DecodeCanonicalInteractionRequirementConditioned(raw)
		must(err)
		report.Fits++
	}
	save(out, "split-policy.json", map[string]any{"schema": "gooo/canonical-role-research-policy/v1", "comparison": report.Models["comparison"], "branches": report.Models["branches"], "model_calls_before_first_candidate": 2, "parameters": 2 * contractdecision.InteractionRequirementParameterCount, "weight_bytes": 8 * contractdecision.InteractionRequirementParameterCount, "scope": "research producer only; no public compiler loader for this two-model policy"})
	phase = "construction"
	searches := newRows(out, "searches.jsonl")
	defer searches.Close()
	events := newRows(out, "events.jsonl")
	defer events.Close()
	event := func(value any) { _, err := events.Write(r.Encode(value)); must(err) }
	for i, x := range sources {
		for _, mode := range []string{"fixed", "joint", "split"} {
			phase = "construction/" + x.Spec.ID + "/" + mode
			row := search(ctx, x, prepared[i], mode, models, event)
			_, err := searches.Write(r.Encode(row))
			must(err)
			summarize(&report, x, row)
		}
	}
	require(report.Fits == 3 && report.Searches == 768 && report.ModelCalls == 768, "original execution counts")
	save(out, "report.json", report)
	save(out, "completed.json", map[string]string{"producer": producer, "time": time.Now().UTC().Format(time.RFC3339Nano)})
}
