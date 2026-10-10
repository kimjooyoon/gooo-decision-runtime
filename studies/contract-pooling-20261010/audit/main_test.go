package main

import (
	"path/filepath"
	"testing"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
)

func TestCompleteRecordedPoolingStudy(t *testing.T) {
	result, err := audit("../result", "../../contract-goals-20261010/result")
	if err != nil || result.Improved+result.Regressed+result.Unchanged != 196 || result.Pairs["extreme/all"].Count != 97 {
		t.Fatal(result, err)
	}
	for key, want := range map[string]int{"extreme/all": 1582, "saved_model/all": 1533, "saved_deterministic/all": 1333} {
		g := result.Groups[key]
		if g.FirstOutputPassed != want || g.FirstOutputTotal != 2048 || g.FirstConditionPassed != 588 || g.FirstConditionTotal != 588 {
			t.Fatalf("%s: incorrect partial-completeness counts: %+v", key, g)
		}
	}
}

func rejected(t *testing.T, check func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("altered evidence accepted")
		}
	}()
	check()
}

func TestExactRecordedOutputsAndRankingMutations(t *testing.T) {
	frozen := filepath.Join("..", "..", "contract-goals-20261010", "result")
	x := lines[r.Source](load(frozen, "sources.jsonl.gz"))[0]
	row := lines[r.Search](load("../result", "searches.jsonl.gz"))[0]
	for _, mutation := range []string{"actual", "actual_expected", "rank", "source_goal"} {
		t.Run(mutation, func(t *testing.T) {
			a := decode[r.Source](r.Encode(x))
			b := decode[r.Search](r.Encode(row))
			switch mutation {
			case "actual":
				b.Progress[1].NewAttempts[0].Results[0].Actual++
			case "actual_expected":
				b.Progress[1].NewAttempts[0].Results[0].Actual++
				b.Progress[1].NewAttempts[0].Results[0].Expected++
			case "rank":
				b.Ranking.Proposed ^= 2
			case "source_goal":
				a.Document.TestCases[0].Expected++
			}
			rejected(t, func() {
				checkSource(a, x.Spec)
				checkSearch(b, a, "32de9753de5637c3575b188261c80e510212f17194d0710b040e885fc05d0f8d")
			})
		})
	}
}
