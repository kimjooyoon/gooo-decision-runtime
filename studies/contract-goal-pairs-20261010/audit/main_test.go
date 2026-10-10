package main

import (
	"path/filepath"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
)

func TestCompleteRecordedGoalPairStudy(t *testing.T) {
	result, err := audit("../result", "../../contract-goals-20261010/result", "../../contract-pooling-20261010/result")
	if err != nil || result.Improved+result.Regressed+result.Unchanged != 196 || result.Pairs["paired/all"].Count != 97 {
		t.Fatal(result, err)
	}
	if result.Improved != 10 || result.Regressed != 14 || result.Unchanged != 172 || result.Groups["paired/all"].FirstValid != 128 || result.Groups["paired/all"].Attempts != 268 || result.Groups["paired/all"].Complete != 194 || result.Pairs["paired/all"].SameProposal != 58 || result.Pairs["paired/all"].BothValid != 35 || result.Pairs["paired/all"].BothInvalid != 4 {
		t.Fatal("original gains/regressions or finite completeness changed", result)
	}
	for key, want := range map[string]int{"paired/all": 1548, "saved_signed_max_abs/all": 1582} {
		g := result.Groups[key]
		if g.FirstOutputPassed != want || g.FirstOutputTotal != 2048 || g.FirstConditionPassed != 588 || g.FirstConditionTotal != 588 {
			t.Fatalf("%s: incorrect partial-completeness counts: %+v", key, g)
		}
	}
}

func TestTrainingPairMutationsAreRejected(t *testing.T) {
	training := decode[[]r.Training](load("../result", "training.json.gz"))
	pairs := decode[[]contractdecision.GoalPair](load("../result", "pairs.json"))
	for _, mutation := range []string{"index", "label", "input"} {
		t.Run(mutation, func(t *testing.T) {
			a := decode[[]r.Training](r.Encode(training))
			b := append([]contractdecision.GoalPair(nil), pairs...)
			switch mutation {
			case "index":
				b[0].Second = 3
			case "label":
				a[1].Acceptable = a[0].Acceptable
			case "input":
				a[1].Inputs[0][0]++
			}
			rejected(t, func() { checkPairs(a, b) })
		})
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
	for _, mutation := range []string{"actual", "actual_expected", "positive_actual_expected", "rank", "source_goal"} {
		t.Run(mutation, func(t *testing.T) {
			a := decode[r.Source](r.Encode(x))
			b := decode[r.Search](r.Encode(row))
			switch mutation {
			case "actual":
				b.Progress[1].NewAttempts[0].Results[0].Actual++
			case "actual_expected":
				b.Progress[1].NewAttempts[0].Results[0].Actual++
				b.Progress[1].NewAttempts[0].Results[0].Expected++
			case "positive_actual_expected":
				b.Progress[1].NewAttempts[0].Results[5].Actual++
				b.Progress[1].NewAttempts[0].Results[5].Expected++
			case "rank":
				b.Ranking.Proposed ^= 2
			case "source_goal":
				a.Document.TestCases[0].Expected++
			}
			rejected(t, func() {
				checkSource(a, x.Spec)
				checkSearch(b, a, "e6dc12548759daa3ef77244ae155d88ce25f83d8372a821debafb520c4c260e2")
			})
		})
	}
}
