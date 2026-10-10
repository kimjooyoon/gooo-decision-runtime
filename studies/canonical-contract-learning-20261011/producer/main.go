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
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
)

func main() {
	require(len(os.Args) == 4, "usage: canonical-contract-learning COMPILER_CHECKOUT FROZEN_CORPUS_GZIP FRESH_OUTPUT")
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	options := contractdecision.FitOptions{Epochs: 2000, LearningRate: .3, L2: .0001, Seed: 31}
	report := r.Report{Schema: "gooo/canonical-contract-learning-study/v1", Producer: producer, Compiler: r.Compiler, SDKBase: r.SDKBase, CorpusSHA: r.CorpusSHA,
		Options: options, Models: map[string]r.Model{}, Groups: map[string]r.Group{}, InputAudits: map[string]*contractdecision.InputAudit{}}
	sourceFile := newRows(out, "sources.jsonl")
	var sources []r.Source
	var prepared []*pathplan.PreparedPlan
	var trainingLegacy, trainingCanonical []contractdecision.InteractionRequirementSample
	var allLegacy, allCanonical, sparseLegacy, sparseCanonical []contractdecision.OrderedRequirementSample
	var trainRecords []trainingRecord
	for _, f := range frozen {
		x, p := collect(ctx, f)
		_, err := sourceFile.Write(r.Encode(x))
		must(err)
		sources, prepared = append(sources, x), append(prepared, p)
		a := contractdecision.OrderedRequirementSample{Inputs: x.LegacyInputs, Cases: r.Rows(x.OutputRows), Conditions: r.Rows(x.ConditionRows), Masks: []uint16{0, 1, 2, 3}, Acceptable: x.Acceptable}
		b := a
		b.Inputs = x.CanonicalInputs
		allLegacy, allCanonical = append(allLegacy, a), append(allCanonical, b)
		if x.Spec.Split == "train" {
			trainingLegacy, trainingCanonical = append(trainingLegacy, a), append(trainingCanonical, b)
			trainRecords = append(trainRecords, trainingRecord{x.Spec.ID, x.PlanSHA, x.CasesSHA, x.LegacyInputs, x.CanonicalInputs, x.OutputRows, x.ConditionRows, x.Acceptable})
		}
		if x.Spec.Split == "sparse" {
			a.Acceptable, b.Acceptable = x.ProbeAcceptable, x.ProbeAcceptable
			sparseLegacy, sparseCanonical = append(sparseLegacy, a), append(sparseCanonical, b)
		}
	}
	must(sourceFile.Close())
	require(len(sources) == 320 && len(trainingLegacy) == 64 && len(sparseLegacy) == 64, "frozen cohort sizes")
	report.Sources, report.TrainingRows = len(sources), len(trainingLegacy)
	report.SourcesSHA = fileSHA(filepath.Join(out, "sources.jsonl"))
	save(out, "training.json", trainRecords)
	report.InputAudits["legacy_declared"], err = contractdecision.AuditOrderedRequirements(ctx, allLegacy)
	must(err)
	report.InputAudits["canonical_declared"], err = contractdecision.AuditCanonicalRequirements(ctx, allCanonical)
	must(err)
	report.InputAudits["legacy_sparse_probes"], err = contractdecision.AuditOrderedRequirements(ctx, sparseLegacy)
	must(err)
	report.InputAudits["canonical_sparse_probes"], err = contractdecision.AuditCanonicalRequirements(ctx, sparseCanonical)
	must(err)
	// Both fits use identical authored training rows/settings and the same seed.
	// Neither probe labels nor any evaluation document enters either fit.
	var model models
	save(out, "legacy-fit-started.json", map[string]any{"options": options, "rows": 64})
	start := time.Now()
	legacy, history, err := contractdecision.FitInteractionRequirementConditioned(ctx, trainingLegacy, options, contractdecision.MeanPooling)
	fitNS := time.Since(start).Nanoseconds()
	must(err)
	raw := recordModel(out, "legacy", legacy, history, fitNS, &report)
	model.legacy, err = contractdecision.DecodeInteractionRequirementConditioned(raw)
	must(err)
	save(out, "canonical-fit-started.json", map[string]any{"options": options, "rows": 64})
	start = time.Now()
	canonical, history, err := contractdecision.FitCanonicalInteractionRequirementConditioned(ctx, trainingCanonical, options, contractdecision.MeanPooling)
	fitNS = time.Since(start).Nanoseconds()
	must(err)
	raw = recordModel(out, "canonical", canonical, history, fitNS, &report)
	model.canonical, err = contractdecision.DecodeCanonicalInteractionRequirementConditioned(raw)
	must(err)
	report.Fits = 2
	searchFile := newRows(out, "searches.jsonl")
	for i, x := range sources {
		for _, mode := range []string{"fixed", "legacy", "canonical"} {
			row := search(ctx, x, prepared[i], mode, model)
			_, err := searchFile.Write(r.Encode(row))
			must(err)
			summarize(&report, x, row)
		}
	}
	must(searchFile.Close())
	report.SearchesSHA = fileSHA(filepath.Join(out, "searches.jsonl"))
	save(out, "report.json", report)
	save(out, "completed.json", map[string]string{"producer": producer, "time": time.Now().UTC().Format(time.RFC3339Nano)})
}
