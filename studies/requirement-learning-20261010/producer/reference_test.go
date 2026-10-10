package main

import (
	"testing"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/requirement-learning-20261010/record"
)

func reference(s r.Spec, mask uint16, x int64) (int64, bool) {
	v := x
	if s.Form == "rare" && x >= 0 {
		v = s.K
	}
	condition := v < s.K
	if mask>>(s.Order)&1 != 0 {
		condition = s.K < v
	}
	first := condition
	if (s.Reverse == 1) != (mask>>(1-s.Order)&1 != 0) {
		first = !first
	}
	a, b := x, s.K
	switch s.Family {
	case "distance":
		a, b = x-s.K, s.K-x
	case "negative_bound":
		a, b = -x, -s.K
	}
	if first {
		return a, condition
	}
	return b, condition
}

// An arithmetic reference checks the corpus authoring claim before fitting or
// executing compiled candidates. The labels used for fitting still come from
// Gooo's complete four-candidate oracle during the recorded producer run.
func TestFrozenReferenceSatisfiability(t *testing.T) {
	for _, row := range frozenSources("../corpus.jsonl.gz") {
		s := row.Spec
		valid := 0
		for mask := range uint16(4) {
			pass := true
			for _, c := range r.Cases(s) {
				x, _ := reference(s, mask, c.Input)
				pass = pass && x == c.Expected
			}
			for _, c := range r.Conditions(s) {
				_, condition := reference(s, mask, c.Input)
				pass = pass && condition == c.Expected
			}
			if pass {
				valid++
			}
		}
		want := 1
		if s.Form == "empty" {
			want = 2
		}
		if s.Form == "contradiction" {
			want = 0
		}
		if valid != want {
			t.Fatalf("%s: reference accepts %d paths, want %d", s.ID, valid, want)
		}
	}
}
