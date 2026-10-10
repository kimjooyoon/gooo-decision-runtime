package main

import (
	"context"
	"testing"
)

func TestSourceSpecificationsLowerWithoutTraining(t *testing.T) {
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, spec := range specifications() {
		if seen[spec.id] {
			t.Fatal("duplicate source id")
		}
		seen[spec.id] = true
		counts[spec.split]++
		r, err := lower(spec)
		if err != nil {
			t.Fatal(spec.id, err)
		}
		if r.SHA != hash([]byte(r.Source)) || len(r.Document.Plan.Decisions) != len(spec.choices) || len(r.Document.TestCases) != len(spec.cases) {
			t.Fatal("source lowering shape", spec.id)
		}
	}
	for name, want := range map[string]int{"train": 8, "wording": 16, "structure": 8, "both": 16, "nested": 8, "literal_collision": 4} {
		if counts[name] != want {
			t.Fatal(name, counts[name])
		}
	}
}

func TestAuthoredSourceFamiliesHaveCompleteTypedCandidates(t *testing.T) {
	for _, spec := range []specification{minmax("max", -10, true, false, 0), minmax("min", 10, false, true, 0), clamp(10, false, false), sign(false, 0)} {
		r, err := lower(spec)
		if err != nil {
			t.Fatal(err)
		}
		p, err := r.Document.Prepare()
		if err != nil {
			t.Fatal(err)
		}
		accepted := 0
		for mask := range 1 << len(r.Document.Plan.Decisions) {
			choices := map[string]string{}
			for i, choice := range r.Document.Plan.Decisions {
				choices[choice.ID] = choice.Options[mask>>i&1].Label
			}
			program, err := p.Compile(choices)
			if err != nil {
				t.Fatal(err)
			}
			conditions, err := p.CheckDeclaredConditions(context.Background(), choices)
			if err != nil {
				t.Fatal(err)
			}
			valid := true
			for _, c := range conditions {
				valid = valid && c.Passed
			}
			for _, c := range r.Document.TestCases {
				value, err := program.Evaluate(c.Input)
				if err != nil {
					t.Fatal(err)
				}
				valid = valid && value.Int == c.Expected
			}
			if valid {
				accepted++
			}
		}
		if accepted != 1 {
			t.Fatal(spec.id, "source candidate count", accepted)
		}
	}
}
