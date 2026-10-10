package main

import (
	"reflect"
	"testing"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-pooling-20261011/record"
)

func TestRetainedPoolingStudy(t *testing.T) {
	got, err := audit("../result")
	if err != nil {
		t.Fatal(err)
	}
	if got.Documents != 256 || got.Fits != 2 || got.Sessions != 768 || got.Calls != 512 ||
		!reflect.DeepEqual(got.EquivalentFormPairs, map[string]int{"mean": 96, "extreme": 96}) {
		t.Fatal(got)
	}
	for mode, correct := range map[string]int{"fixed": 32, "mean": 32, "extreme": 36} {
		g := got.Groups[mode+"/in_family"]
		if g.Programs != 128 || g.FirstProbeComplete != correct || g.InitialProbeComplete != 128 {
			t.Fatal(mode, g)
		}
	}
}

func TestAuditRejectsChangedSession(t *testing.T) {
	sources := rows[r.Source](inflated("../result/sources.jsonl.gz"))
	saved := rows[r.Search](inflated("../result/searches.jsonl.gz"))
	for name, change := range map[string]func(*r.Search){
		"late prediction": func(r *r.Search) { r.Stages[0].Progress[1].PredictionsThisAdvance = 1 },
		"first mask":      func(r *r.Search) { r.Stages[0].Ranking.Proposed = 7 },
		"counts":          func(r *r.Search) { r.Stages[0].Progress[1].NewAttempts[0].Passed = 99 },
		"source":          func(r *r.Search) { r.Stages[0].SourceSHA = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			row := decode[r.Search](r.Encode(saved[0]))
			change(&row)
			rejected := false
			func() {
				defer func() { rejected = recover() != nil }()
				validateSession(sources[0], row)
			}()
			if !rejected {
				t.Fatal("changed record accepted")
			}
		})
	}
}
