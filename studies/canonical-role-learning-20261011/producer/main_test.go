package main

import (
	"context"
	"reflect"
	"strings"
	"testing"

	previous "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
)

// This preflight lowers source and reads features; it never collects masks,
// calls a model, fits weights, or executes a candidate on the frozen corpus.
func TestFrozenRoleInputsWithoutPredictions(t *testing.T) {
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
