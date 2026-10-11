package main

import (
	"context"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/record"
	"reflect"
	"testing"
)

// Source-only preflight: no corpus labels, model calls, fits or candidate runs.
func TestFrozenInputsWithoutPredictions(t *testing.T) {
	rows := frozenSources("../corpus.jsonl.gz")
	counts := map[string]int{}
	seen := map[string]bool{}
	bySpec := map[r.Spec]inputRows{}
	pairs := 0
	for _, x := range rows {
		s := x.Spec
		counts[s.Split]++
		if seen[s.ID] {
			t.Fatal("duplicate identity", s.ID)
		}
		seen[s.ID] = true
		d, p := lower(context.Background(), s.ID, x.Gooo)
		inputs := prepareRows(p, d, true)
		conditions := 3
		if s.Diagnostic == "missing_conditions" {
			conditions = 0
		}
		if len(inputs.canonical) != 2 || len(inputs.relative) != 2 || len(inputs.outputs) != 8 || len(inputs.conditions) != conditions || len(x.Probes) != 16 || len(x.ProbeConditions) != 3 {
			t.Fatal("dimensions", s.ID)
		}
		for _, view := range inputs.relations {
			if !view.Available || view.Count != conditions {
				t.Fatal("relation count", s.ID)
			}
		}
		key := s
		key.ID = ""
		if s.Form == "assignment" {
			key.Form = "direct"
			other, ok := bySpec[key]
			if !ok || !reflect.DeepEqual(inputs.canonical, other.canonical) || !reflect.DeepEqual(inputs.relative, other.relative) {
				t.Fatal("equivalent form features", s.ID)
			}
			pairs++
		} else {
			bySpec[key] = inputs
		}
		for _, probe := range x.Probes {
			for _, visible := range d.TestCases {
				if probe.Input == visible.Input {
					t.Fatal("output probe overlap", s.ID)
				}
			}
		}
	}
	if pairs != 112 || !reflect.DeepEqual(counts, map[string]int{"train": 64, "in_family": 128, "unseen_family": 64, "missing_conditions": 16, "unsatisfiable_conditions": 16}) {
		t.Fatal(pairs, counts)
	}
}
