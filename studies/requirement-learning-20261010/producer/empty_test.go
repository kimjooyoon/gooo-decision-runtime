package main

import (
	"context"
	"math/bits"
	"testing"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/requirement-learning-20261010/record"
)

func TestEmptyConditionCollection(t *testing.T) {
	for _, s := range r.Specs() {
		if s.Form != "empty" {
			continue
		}
		x, _ := collect(context.Background(), r.FrozenSource{Spec: s, Gooo: r.Gooo(s)})
		if len(x.ConditionRows) != 0 || len(x.Candidates) != 4 || bits.OnesCount64(x.Acceptable) != 2 {
			t.Fatal("empty condition suite changed output requirements", s.ID)
		}
		for _, c := range x.Candidates {
			if len(c.Conditions) != 0 || len(c.Outputs) != 8 {
				t.Fatal("empty condition collection", s.ID)
			}
		}
	}
}
