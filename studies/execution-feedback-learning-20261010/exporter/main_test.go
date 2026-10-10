package main

import (
	"context"
	"testing"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestFixedSourcesLowerAndHaveCompleteTypedSolutions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	specs := specifications()
	if len(specs) != 128 {
		t.Fatal(len(specs))
	}
	counts := map[string]int{}
	seen := map[string]bool{}
	candidates, contexts := 0, 0
	for _, spec := range specs {
		if seen[spec.id] {
			t.Fatal("duplicate source", spec.id)
		}
		seen[spec.id] = true
		counts[spec.split]++
		r, err := lower(spec)
		if err != nil {
			t.Fatal(spec.id, err)
		}
		p, err := r.Document.Prepare()
		if err != nil {
			t.Fatal(spec.id, err)
		}
		input, err := p.InitialExecutionInput(r.Document.TestCases)
		if err != nil {
			t.Fatal(err)
		}
		for _, choice := range r.Document.Plan.Decisions {
			var features [decision.ExecutionFeatureDim]float32
			if err := input.ExecutionFeaturesInto(choice.ID, &features); err != nil {
				t.Fatal(spec.id, err)
			}
		}
		count := 1 << len(r.Document.Plan.Decisions)
		candidates += count
		contexts += count + 1
		accepted := 0
		for mask := range count {
			choices := p.Defaults()
			for i, c := range r.Document.Plan.Decisions {
				choices[c.ID] = c.Options[mask>>i&1].Label
			}
			body, err := p.Compile(choices)
			if err != nil {
				t.Fatal(spec.id, mask, err)
			}
			conditions, err := p.CheckDeclaredConditions(ctx, choices)
			if err != nil {
				t.Fatal(err)
			}
			ok := true
			for _, condition := range conditions {
				ok = ok && condition.Passed
			}
			for _, test := range r.Document.TestCases {
				got, err := body.Evaluate(test.Input)
				if err != nil {
					t.Fatal(err)
				}
				ok = ok && got.Int == test.Expected
			}
			if ok {
				accepted++
			}
			observed, err := p.ObserveExecutionInput(ctx, choices, r.Document.TestCases)
			if err != nil {
				t.Fatal(spec.id, err)
			}
			for _, choice := range r.Document.Plan.Decisions {
				var features [decision.ExecutionFeatureDim]float32
				if err := observed.ExecutionFeaturesInto(choice.ID, &features); err != nil {
					t.Fatal(spec.id, err)
				}
			}
		}
		if accepted == 0 {
			t.Fatal("no valid source selection", spec.id)
		}
	}
	if candidates != 576 || contexts != 704 {
		t.Fatal(candidates, contexts)
	}
	for name, want := range map[string]int{"train": 32, "wording": 32, "constants": 32, "assignment": 16, "new_family": 16} {
		if counts[name] != want {
			t.Fatal(name, counts[name])
		}
	}
}
