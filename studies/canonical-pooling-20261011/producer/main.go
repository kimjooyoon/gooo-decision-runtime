package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-pooling-20261011/record"
)

func main() {
	require(len(os.Args) == 4, "usage: canonical-pooling COMPILER_CHECKOUT FROZEN_CORPUS_GZIP FRESH_OUTPUT")
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
	save(out, "started.json", map[string]string{"producer": producer, "compiler": r.Compiler, "sdk_base": r.SDKBase,
		"corpus": r.CorpusSHA, "time": time.Now().UTC().Format(time.RFC3339Nano)})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	options := contractdecision.FitOptions{Epochs: 2000, LearningRate: .3, L2: .0001, Seed: 37}
	report := r.Report{Schema: "gooo/canonical-pooling-study/v1", Producer: producer, Compiler: r.Compiler,
		SDKBase: r.SDKBase, CorpusSHA: r.CorpusSHA, Options: options, Models: map[string]r.Model{}, Groups: map[string]r.Group{}}
	sourceFile := newRows(out, "sources.jsonl")
	var sources []r.Source
	var prepared []*pathplan.PreparedPlan
	var training, all []contractdecision.InteractionRequirementSample
	var records []r.TrainingRecord
	for _, frozen := range frozen {
		x, p := collect(ctx, frozen)
		_, err := sourceFile.Write(r.Encode(x))
		must(err)
		sources, prepared = append(sources, x), append(prepared, p)
		sample := contractdecision.InteractionRequirementSample{Inputs: x.CanonicalInputs,
			Cases: r.Rows(x.OutputRows), Conditions: r.Rows(x.ConditionRows), Masks: []uint16{0, 1, 2, 3}, Acceptable: x.Acceptable}
		all = append(all, sample)
		if x.Spec.Split == "train" {
			training = append(training, sample)
			records = append(records, r.TrainingRecord{ID: x.Spec.ID, PlanSHA: x.PlanSHA, CasesSHA: x.CasesSHA,
				CanonicalInputs: x.CanonicalInputs, Outputs: x.OutputRows, Conditions: x.ConditionRows, Acceptable: x.Acceptable})
		}
	}
	must(sourceFile.Close())
	require(len(sources) == 256 && len(training) == 64, "frozen cohorts")
	report.Sources, report.TrainingRows = len(sources), len(training)
	report.SourcesSHA = fileSHA(filepath.Join(out, "sources.jsonl"))
	save(out, "training.json", records)
	report.InputAudit, err = contractdecision.AuditCanonicalRequirements(ctx, all)
	must(err)
	models := map[string]*contractdecision.CanonicalInteractionRequirementModel{}
	for _, recipe := range []struct{ name, pooling string }{{"mean", contractdecision.MeanPooling}, {"extreme", contractdecision.ExtremePooling}} {
		save(out, recipe.name+"-fit-started.json", map[string]any{"options": options, "rows": len(training), "pooling": recipe.pooling})
		start := time.Now()
		model, history, err := contractdecision.FitCanonicalInteractionRequirementConditioned(ctx, training, options, recipe.pooling)
		fitNS := time.Since(start).Nanoseconds()
		// Preserve partial losses if a fit fails. No retry path exists.
		save(out, "history-"+recipe.name+".json", history)
		if err != nil {
			save(out, recipe.name+"-fit-failure.json", map[string]any{"error": err.Error(), "fit_ns": fitNS})
			panic(err)
		}
		raw, info := recordModel(out, recipe.name, model, history, fitNS)
		report.Models[recipe.name] = info
		models[recipe.name], err = contractdecision.DecodeCanonicalInteractionRequirementConditioned(raw)
		must(err)
		require(models[recipe.name].Pooling() == recipe.pooling, "reloaded pooling mode")
		report.Fits++
	}
	searchFile := newRows(out, "searches.jsonl")
	for i, x := range sources {
		for _, mode := range []string{"fixed", "mean", "extreme"} {
			row := search(ctx, x, prepared[i], mode, models[mode])
			_, err := searchFile.Write(r.Encode(row))
			must(err)
			summarize(&report, x, row)
		}
	}
	must(searchFile.Close())
	require(report.Fits == 2 && report.InitialSearches == 768 && report.ModelCalls == 512, "original execution counts")
	report.SearchesSHA = fileSHA(filepath.Join(out, "searches.jsonl"))
	save(out, "report.json", report)
	save(out, "completed.json", map[string]string{"producer": producer, "time": time.Now().UTC().Format(time.RFC3339Nano)})
}
