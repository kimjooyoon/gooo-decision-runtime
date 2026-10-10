package main

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
	"github.com/kimjooyoon/meta-ontology-go/internal/bodycodegen"
)

// Source preflight: no candidate evaluation, prediction, labels or fitting.
func TestFrozenSourceAndBothInputFormats(t *testing.T) {
	rows := frozenSources("../corpus.jsonl.gz")
	counts := map[string]int{}
	seen := map[string]bool{}
	canonical := map[string][][528]float32{}
	for _, x := range rows {
		s := x.Spec
		if seen[s.ID] {
			t.Fatal("duplicate source", s.ID)
		}
		seen[s.ID] = true
		counts[s.Split]++
		d, err := bodycodegen.DecodeSourcePathDocument(context.Background(), s.ID+".gooo", []byte(x.Gooo), "Choose", nil)
		if err != nil {
			t.Fatal(s.ID, err)
		}
		if !reflect.DeepEqual(d.TestCases, r.Cases(s)) || !reflect.DeepEqual(d.Plan.ConditionCases, r.Conditions(s)) {
			t.Fatal("authored cases", s.ID)
		}
		if len(d.Plan.Decisions) != 2 || d.Plan.Decisions[0].ID != "comparison" || d.Plan.Decisions[1].ID != "branches" {
			t.Fatal("choice order", s.ID)
		}
		p, err := d.Prepare()
		if err != nil {
			t.Fatal(s.ID, err)
		}
		input, err := p.InitialContractInput(d.TestCases)
		if err != nil {
			t.Fatal(s.ID, err)
		}
		var features [][528]float32
		for _, c := range d.Plan.Decisions {
			var a, b [528]float32
			if err := input.OrderedSourceFeaturesInto(c.ID, &a); err != nil {
				t.Fatal(s.ID, err)
			}
			if err := input.CanonicalOrderedSourceFeaturesInto(c.ID, &b); err != nil {
				t.Fatal(s.ID, err)
			}
			features = append(features, b)
		}
		key := strings.Replace(s.ID, "-assignment-", "-direct-", 1)
		if s.Form == "assignment" {
			if !reflect.DeepEqual(features, canonical[key]) {
				t.Fatal("paired canonical inputs", s.ID)
			}
		} else {
			canonical[key] = features
		}
		if len(x.Probes) != 16 || len(x.ProbeConditions) != 3 {
			t.Fatal("probe count", s.ID)
		}
		for _, probe := range x.Probes {
			for _, visible := range d.TestCases {
				if probe.Input == visible.Input {
					t.Fatal("probe output exposed in initial contract", s.ID)
				}
			}
		}
		if s.Split == "train" {
			if s.Form != "direct" || s.Wording > 1 || (s.K != 41 && s.K != -23) || s.Family == "affine" {
				t.Fatal("training scope", s.ID)
			}
		} else if s.Wording < 2 || s.K == 41 || s.K == -23 {
			t.Fatal("evaluation leakage", s.ID)
		}
		if s.Split == "sparse" && (len(d.TestCases) != 1 || len(d.Plan.ConditionCases) != 1 || d.TestCases[0].Expected != 0 || d.Plan.ConditionCases[0].Expected) {
			t.Fatal("sparse contract", s.ID)
		}
	}
	if !reflect.DeepEqual(counts, map[string]int{"train": 64, "in_family": 128, "unseen_family": 64, "sparse": 64}) {
		t.Fatal(counts)
	}
}

func TestFreshFeedbackFixtureKeepsVisibleAndProbeResultsSeparate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// This K=5 fixture is absent from the frozen corpus. No weight fit occurs.
	spec := r.Spec{ID: "unit-feedback", Family: "distance", Form: "assignment", Split: "sparse", K: 5, OutputGoal: 1}
	frozen := r.Frozen{Spec: spec, Gooo: r.Gooo(spec), Probes: r.Probes(spec), ProbeConditions: r.ProbeConditions(spec)}
	x, p := collect(ctx, frozen)
	legacy, err := contractdecision.NewInteractionRequirementConditioned([contractdecision.InteractionRequirementParameterCount]float32{}, contractdecision.MeanPooling)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := contractdecision.NewCanonicalInteractionRequirementConditioned([contractdecision.InteractionRequirementParameterCount]float32{}, contractdecision.MeanPooling)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"fixed", "legacy", "canonical"} {
		row := search(ctx, x, p, mode, models{legacy, canonical})
		if len(row.Stages) != 3 || row.Stages[0].NextOutput == nil || row.Stages[1].NextCondition == nil || r.Passed(row.Stages[0].Probe) || !r.Passed(row.Stages[2].Probe) {
			t.Fatal("bounded counterexample progression", mode, len(row.Stages))
		}
		if len(row.Stages[0].Document.TestCases) != 1 || len(row.Stages[1].Document.TestCases) != 2 || len(row.Stages[1].Document.Plan.ConditionCases) != 1 || len(row.Stages[2].Document.Plan.ConditionCases) != 2 || len(x.Document.TestCases) != 1 {
			t.Fatal("counterexamples changed an earlier source", mode)
		}
		for _, stage := range row.Stages {
			calls := 1
			if mode == "fixed" {
				calls = 0
			}
			last := stage.Progress[len(stage.Progress)-1]
			if stage.Ranking.Calls != calls || stage.Progress[0].Attempted != 0 || last.Status != "TRAINING_COMPLETE" || last.Attempted > 4 {
				t.Fatal("stage scope", mode, stage.Number)
			}
			for _, step := range stage.Progress {
				if step.PredictionsThisAdvance != 0 {
					t.Fatal("later inference")
				}
			}
		}
	}
}
