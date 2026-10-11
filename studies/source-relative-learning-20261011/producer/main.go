package main

import (
	"context"
	"fmt"
	"math/bits"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	relative "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/model"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/record"
)

func main() {
	require(len(os.Args) == 4, "usage: relative-study COMPILER_CHECKOUT FROZEN_CORPUS_GZIP FRESH_OUTPUT")
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
	report := r.Report{Schema: "gooo/source-relative-learning-study/v1", Producer: producer, Compiler: r.Compiler, SDKBase: r.SDKBase,
		CorpusSHA: r.CorpusSHA, Options: options, Models: map[string]r.Model{}, Groups: map[string]r.Group{}}
	sourceFile := newRows(out, "sources.jsonl")
	defer sourceFile.Close()
	var sources []r.Source
	var prepared []*pathplan.PreparedPlan
	training := map[string][]contractdecision.InteractionRequirementSample{}
	var records []r.TrainingRecord
	for _, item := range frozen {
		phase = "collect/" + item.Spec.ID
		x, p := collect(ctx, item)
		_, err := sourceFile.Write(r.Encode(x))
		must(err)
		sources, prepared = append(sources, x), append(prepared, p)
		if x.Spec.Split != "train" {
			continue
		}
		require(bits.OnesCount64(x.Acceptable) == 1, "training source must have one authored accepted mask")
		records = append(records, r.TrainingRecord{ID: x.Spec.ID, PlanSHA: x.PlanSHA, CasesSHA: x.CasesSHA,
			CanonicalInputs: x.CanonicalInputs, RelativeInputs: x.RelativeInputs, Outputs: x.OutputRows, Conditions: x.ConditionRows, Acceptable: x.Acceptable})
		for _, name := range []string{"baseline", "relative"} {
			input := x.CanonicalInputs
			if name == "relative" {
				input = x.RelativeInputs
			}
			training[name] = append(training[name], contractdecision.InteractionRequirementSample{Inputs: input, Cases: r.Rows(x.OutputRows),
				Conditions: r.Rows(x.ConditionRows), Masks: []uint16{0, 1, 2, 3}, Acceptable: x.Acceptable})
		}
	}
	require(len(sources) == 288 && len(records) == 64, "frozen populations differ")
	report.Sources, report.TrainingRows = len(sources), len(records)
	save(out, "training.json", records)
	models := map[string]predictor{}
	for _, name := range []string{"baseline", "relative"} {
		phase = "fit-" + name
		save(out, name+"-fit-started.json", map[string]any{"rows": len(training[name]), "options": options, "pooling": contractdecision.MeanPooling})
		var model storedModel
		var history []contractdecision.Epoch
		start := time.Now()
		if name == "baseline" {
			model, history, err = contractdecision.FitCanonicalInteractionRequirementConditioned(ctx, training[name], options, contractdecision.MeanPooling)
		} else {
			model, history, err = relative.Fit(ctx, training[name], options)
		}
		ns := time.Since(start).Nanoseconds()
		save(out, "history-"+name+".json", history)
		if err != nil {
			save(out, name+"-fit-failure.json", map[string]any{"error": err.Error(), "fit_ns": ns})
			panic(err)
		}
		raw, info := recordModel(out, name, model, history, ns)
		report.Models[name] = info
		if name == "baseline" {
			models[name], err = contractdecision.DecodeCanonicalInteractionRequirementConditioned(raw)
		} else {
			models[name], err = relative.Decode(raw)
		}
		must(err)
		report.Fits++
	}
	searches, events := newRows(out, "searches.jsonl"), newRows(out, "events.jsonl")
	defer searches.Close()
	defer events.Close()
	event := func(value any) { _, err := events.Write(r.Encode(value)); must(err) }
	for i, x := range sources {
		for _, mode := range []string{"fixed", "rule", "baseline", "relative"} {
			phase = "construction/" + x.Spec.ID + "/" + mode
			row := search(ctx, x, prepared[i], mode, models, event)
			_, err := searches.Write(r.Encode(row))
			must(err)
			summarize(&report, x, row)
		}
	}
	require(report.Fits == 2 && report.Searches == 1152 && report.ModelCalls == 576, "original execution counts differ")
	save(out, "report.json", report)
	save(out, "completed.json", map[string]string{"producer": producer, "time": time.Now().UTC().Format(time.RFC3339Nano)})
}
