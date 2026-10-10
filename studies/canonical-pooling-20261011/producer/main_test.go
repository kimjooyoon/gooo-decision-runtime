package main

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	previous "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-pooling-20261011/record"
)

// This preflight lowers source and reads features; it never collects masks,
// calls a model, fits weights, or executes a candidate on the frozen corpus.
func TestFrozenPoolingInputsWithoutPredictions(t *testing.T) {
	rows := frozenSources("../corpus.jsonl.gz")
	counts := map[string]int{}
	canonical := map[string][][528]float32{}
	pairs := 0
	for _, x := range rows {
		s := x.Spec
		counts[s.Split]++
		d, p := lower(context.Background(), s.ID, x.Gooo)
		if !reflect.DeepEqual(d.TestCases, previous.Cases(s)) || !reflect.DeepEqual(d.Plan.ConditionCases, previous.Conditions(s)) {
			t.Fatal("authored requirements changed", s.ID)
		}
		input, err := p.InitialContractInput(d.TestCases)
		must(err)
		var features [][528]float32
		for _, c := range d.Plan.Decisions {
			var row [528]float32
			must(input.CanonicalOrderedSourceFeaturesInto(c.ID, &row))
			features = append(features, row)
		}
		if len(features) != 2 || input.CaseCount() != 8 || input.ConditionCount() != 3 || len(x.Probes) != 16 {
			t.Fatal("input dimensions", s.ID)
		}
		key := strings.Replace(s.ID, "-assignment-", "-direct-", 1)
		if s.Form == "assignment" {
			if !reflect.DeepEqual(features, canonical[key]) {
				t.Fatal("canonical source pair", s.ID)
			}
			pairs++
		} else {
			canonical[key] = features
		}
		for _, probe := range x.Probes {
			for _, visible := range d.TestCases {
				if probe.Input == visible.Input {
					t.Fatal("probe output overlaps construction input", s.ID)
				}
			}
		}
	}
	if pairs != 96 || !reflect.DeepEqual(counts, map[string]int{"train": 64, "in_family": 128, "unseen_family": 64}) {
		t.Fatal(pairs, counts)
	}
}

func TestBothPoolingRoutesOnNonStudyFixture(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	s := r.Spec{ID: "unit-pooling", Family: "distance", Form: "assignment", Split: "unit", K: 5,
		Reverse: 1, ConditionGoal: 1}
	x, p := collect(ctx, r.Freeze(s))
	if x.Acceptable != 1<<1 {
		t.Fatal("unexpected unit-fixture mask", x.Acceptable)
	}
	for _, recipe := range []struct{ mode, pool string }{{"fixed", ""}, {"mean", contractdecision.MeanPooling}, {"extreme", contractdecision.ExtremePooling}} {
		var model *contractdecision.CanonicalInteractionRequirementModel
		if recipe.mode != "fixed" {
			var err error
			model, err = contractdecision.NewCanonicalInteractionRequirementConditioned([contractdecision.InteractionRequirementParameterCount]float32{}, recipe.pool)
			must(err)
		}
		row := search(ctx, x, p, recipe.mode, model)
		stage := row.Stages[0]
		last := stage.Progress[len(stage.Progress)-1]
		if last.Status != "TRAINING_COMPLETE" || last.Attempted > 4 || !r.Passed(stage.Probe) {
			t.Fatal("finite selection", recipe.mode, last.Status, last.Attempted)
		}
		calls := 1
		if recipe.mode == "fixed" {
			calls = 0
		}
		if stage.Ranking.Calls != calls || stage.Progress[0].Attempted != 0 {
			t.Fatal("initial prediction", recipe.mode)
		}
		for _, progress := range stage.Progress {
			if progress.PredictionsThisAdvance != 0 {
				t.Fatal("late prediction", recipe.mode)
			}
		}
		report := r.Report{Groups: map[string]r.Group{}}
		summarize(&report, x, row)
		g := report.Groups[recipe.mode+"/all"]
		if g.Programs != 1 || g.InitialDeclaredComplete != 1 || g.InitialProbeComplete != 1 || g.ModelCalls != calls || report.InitialSearches != 1 {
			t.Fatal("summary lost observations", recipe.mode, g)
		}
	}
}
