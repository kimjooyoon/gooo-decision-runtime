package pathplan

import (
	"math"
	"sync"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

func relationInput(t *testing.T, p Plan) *ContractInput {
	t.Helper()
	prepared, err := Prepare(p)
	if err != nil {
		t.Fatal(err)
	}
	input, err := prepared.InitialContractInput([]TestCase{{0, 0}})
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func TestSourceRelativeConditionsExactIntegersFormsAndOrientation(t *testing.T) {
	for _, k := range []int64{167, -97, 9007199254740993, math.MinInt64, math.MaxInt64} {
		for opIndex, operation := range []string{"less_than", "less_equal", "equal"} {
			for fallback := range 4 {
				var views [2][2]SourceRelativeConditionContext
				for form, p := range []Plan{orderedArithmeticPlan(), canonicalAssignmentPlan()} {
					p.Base.Expressions[1].Int, p.Base.Expressions[2].Operation = k, operation
					p.ConditionCases = nil
					seen := map[int64]bool{}
					for _, x := range []int64{math.MinInt64, k, math.MaxInt64} {
						if seen[x] {
							continue
						}
						seen[x] = true
						p.ConditionCases = append(p.ConditionCases, ConditionCase{p.Decisions[0].ID, x, x == k})
					}
					for j := range p.Decisions {
						p.Decisions[j].Fallback = p.Decisions[j].Options[fallback>>j&1].Label
					}
					input := relationInput(t, p)
					for choice, c := range p.Decisions {
						got, err := input.SourceRelativeConditions(c.ID)
						if err != nil || !got.Available || got.Count != len(seen) || got.PlanSHA256 != input.PlanSHA256() {
							t.Fatal(got, err)
						}
						var want [3][2]uint16
						for x := range seen {
							relation, goal := 1, 0
							if x < k {
								relation = 0
							} else if x > k {
								relation = 2
							} else {
								goal = 1
							}
							if fallback&1 != 0 {
								relation = 2 - relation
							}
							want[relation][goal]++
						}
						if got.Buckets != want || got.Features[15] != 0 || got.Features[4+choice] != 1 {
							t.Fatal("relation or reserved/kind cells differ", k, fallback, got)
						}
						reversed := float32(fallback >> choice & 1)
						if got.Features[1+opIndex] != 1 || got.Features[6] != reversed || got.Features[7] != 1-reversed || got.Features[0] != 1 {
							t.Fatal("operator/option orientation changed", got)
						}
						for relation, goals := range want {
							for goal, count := range goals {
								if got.Features[8+2*relation+goal] != float32(count)/float32(len(seen)) {
									t.Fatal("normalized count differs", got)
								}
							}
						}
						views[form][choice] = got
					}
					if views[form][0].Buckets != views[form][1].Buckets {
						t.Fatal("same if target changed across choice aliases")
					}
				}
				for choice := range 2 {
					if views[0][choice].Features != views[1][choice].Features {
						t.Fatal("equivalent variable form changed relations")
					}
				}
			}
		}
	}
}

func TestSourceRelativeConditionsDeclineBooleanAtoms(t *testing.T) {
	p := orderedArithmeticPlan()
	p.Base.Expressions = []bodyplan.Expr{
		{Kind: "bool", Bool: true}, {Kind: "bool"}, {Kind: "binary", Operation: "equal", Left: 0, Right: 1},
		{Kind: "input", Name: "input"}, {Kind: "int", Int: 10},
		{Kind: "binary", Operation: "subtract", Left: 3, Right: 4},
		{Kind: "binary", Operation: "subtract", Left: 4, Right: 3},
	}
	p.Base.Statements[0].Expr, p.Base.Statements[1].Expr = 5, 6
	got, err := relationInput(t, p).SourceRelativeConditions(p.Decisions[0].ID)
	if err != nil || got.Available || got.Reason != "INTEGER_COMPARISON_ATOMS_REQUIRED" || got.Features != ([16]float32{}) {
		t.Fatal("Boolean atoms were treated as integers", got, err)
	}
}

func TestSourceRelativeConditionsRetainChangedGoalWithEqualRawMeans(t *testing.T) {
	var views [2]SourceRelativeConditionContext
	var rawSums [2][32]float32
	var sourceRows [2][528]float32
	for goal := range 2 {
		p := orderedArithmeticPlan()
		p.Base.Expressions[1].Int = 167
		p.ConditionCases = nil
		for _, x := range []int64{166, 167, 168} {
			p.ConditionCases = append(p.ConditionCases, ConditionCase{p.Decisions[0].ID, x, x == []int64{166, 168}[goal]})
		}
		input := relationInput(t, p)
		var err error
		views[goal], err = input.SourceRelativeConditions(p.Decisions[0].ID)
		if err != nil || input.CanonicalOrderedSourceFeaturesInto(p.Decisions[0].ID, &sourceRows[goal]) != nil {
			t.Fatal(err)
		}
		for i := range input.ConditionCount() {
			row, err := input.ConditionFeatures(i)
			if err != nil {
				t.Fatal(err)
			}
			for j, value := range row {
				rawSums[goal][j] += value
			}
		}
	}
	if rawSums[0] != rawSums[1] || sourceRows[0] != sourceRows[1] || views[0].Features == views[1].Features {
		t.Fatal("regression does not isolate relation/goal association")
	}
	if views[0].Buckets != ([3][2]uint16{{0, 1}, {1, 0}, {1, 0}}) || views[1].Buckets != ([3][2]uint16{{1, 0}, {1, 0}, {0, 1}}) {
		t.Fatal("expected source relation counts differ", views)
	}
}

func TestSourceRelativeConditionsBoundsSnapshotsAndDeclines(t *testing.T) {
	p := orderedArithmeticPlan()
	p.Base.Expressions[1].Int = 1000
	p.ConditionCases = nil
	for i := range 128 {
		p.ConditionCases = append(p.ConditionCases, ConditionCase{p.Decisions[i%2].ID, int64(i), i%2 == 0})
	}
	input := relationInput(t, p)
	id := p.Decisions[0].ID
	want, err := input.SourceRelativeConditions(id)
	if err != nil || want.Count != 128 || want.Buckets != ([3][2]uint16{{64, 64}}) || want.Features[14] != 1 {
		t.Fatal(want, err)
	}
	p.Base.Expressions[1].Int = -1000
	p.ConditionCases[0].Expected = false
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			if got, err := input.SourceRelativeConditions(id); err != nil || got != want {
				t.Error("source snapshot changed", got, err)
			}
		})
	}
	wg.Wait()
	p.ConditionCases = nil
	empty, err := relationInput(t, p).SourceRelativeConditions(id)
	if err != nil || !empty.Available || empty.Count != 0 || empty.Features[0] != 0 || empty.Buckets != ([3][2]uint16{}) {
		t.Fatal("empty requirements fabricated an observation", empty, err)
	}
	p.Base.Expressions[4].Left = 3
	unsupported, err := relationInput(t, p).SourceRelativeConditions(id)
	if err != nil || unsupported.Available || unsupported.Reason == "" || unsupported.Features != ([16]float32{}) {
		t.Fatal("unsupported source fabricated relations", unsupported, err)
	}
	var absent *ContractInput
	if _, err := absent.SourceRelativeConditions(id); err == nil {
		t.Fatal("nil source accepted")
	}
	if _, err := input.SourceRelativeConditions("absent"); err == nil {
		t.Fatal("undeclared choice accepted")
	}
}
