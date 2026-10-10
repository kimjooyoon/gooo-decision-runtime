package main

import (
	"path/filepath"
	"testing"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
)

func TestCompleteRecordedSourceLiteralStudy(t *testing.T) {
	result, err := audit("../result", "../../contract-goals-20261010/result", "../../contract-pooling-20261010/result")
	if err != nil || result.Improved+result.Regressed+result.Unchanged != 196 || result.Pairs["source_literals/all"].Count != 97 {
		t.Fatal(result, err)
	}
	if result.Improved != 0 || result.Regressed != 23 || result.Unchanged != 173 || result.Groups["source_literals/all"].FirstValid != 109 || result.Groups["source_literals/all"].Attempts != 287 || result.Groups["source_literals/all"].Complete != 194 || result.Pairs["source_literals/all"].SameProposal != 85 || result.Pairs["source_literals/all"].BothValid != 12 || result.Pairs["source_literals/all"].BothInvalid != 0 {
		t.Fatal("original gains/regressions or finite completeness changed", result)
	}
	for key, want := range map[string]int{"source_literals/all": 1417, "saved_signed_max_abs/all": 1582} {
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
	sources := lines[r.Source](load(frozen, "sources.jsonl.gz"))
	searches := lines[r.Search](load("../result", "searches.jsonl.gz"))
	for _, mutation := range []string{"actual", "actual_expected", "positive_actual_expected", "rank", "source_goal"} {
		t.Run(mutation, func(t *testing.T) {
			x, row := sources[0], searches[0]
			if mutation == "positive_actual_expected" {
				x, row = sources[1], searches[1]
				value := row.Progress[1].NewAttempts[0].Results[5]
				if value.Actual <= 1<<53 || value.Expected <= 1<<53 {
					t.Fatal("positive mutation must change actual large output values", value)
				}
			}
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
				checkSearch(b, a, "0a3b35e90e878beab8f1fb7cb20bd3caf75831e27d3e0216f65ad33c44588153")
			})
		})
	}
}

func TestLargeOracleOutputMutationsWithoutDigestFailure(t *testing.T) {
	frozen := filepath.Join("..", "..", "contract-goals-20261010", "result")
	sources := lines[r.Source](load(frozen, "sources.jsonl.gz"))
	for _, example := range []struct{ source, mask, output int }{{0, 0, 0}, {1, 2, 5}} {
		source := sources[example.source]
		for _, coordinated := range []bool{false, true} {
			altered := decode[r.Source](r.Encode(source))
			value := &altered.Candidates[example.mask].Outputs[example.output]
			if value.Actual >= -(1<<53) && value.Actual <= 1<<53 {
				t.Fatal("oracle mutation requires a large integer", *value)
			}
			value.Actual++
			if coordinated {
				value.Expected++
			}
			// Candidate output arrays are checked arithmetically by checkSource;
			// they are outside the source text, plan and case digest payloads.
			rejected(t, func() { checkSource(altered, source.Spec) })
		}
	}
}
