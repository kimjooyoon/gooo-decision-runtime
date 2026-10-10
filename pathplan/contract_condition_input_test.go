package pathplan

import (
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestContractConditionInputOwnsAllRowsAndTargets(t *testing.T) {
	plan := executionMaxPlan()
	plan.ConditionCases = make([]ConditionCase, 128)
	for i := range plan.ConditionCases {
		plan.ConditionCases[i] = ConditionCase{ChoiceID: plan.Decisions[i%2].ID,
			Input: -9007199254740995 + int64(i), Expected: i%2 == 0}
	}
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	input, err := prepared.InitialContractInput([]TestCase{{0, 10}})
	if err != nil || input.ConditionCount() != 128 || input.ConditionFeatureVersion() != decision.DeclaredConditionFeatureVersion {
		t.Fatal("complete condition channel missing", input, err)
	}
	owned := append([]ConditionCase(nil), plan.ConditionCases...)
	clear(plan.ConditionCases)
	plan.Decisions[0].ID = "caller-mutated"
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for i, condition := range owned {
				got, err := input.ConditionCase(i)
				if err != nil || got != condition {
					t.Error("source condition changed", i, got, err)
				}
				var want [decision.DeclaredConditionFeatureDim]float32
				_ = decision.DeclaredConditionFeaturesInto(decision.DeclaredConditionFeatureInput{
					Input: condition.Input, Expected: condition.Expected, TargetChoice: i % 2,
					ChoiceCount: 2, Index: i, Count: 128}, &want)
				row, err := input.ConditionFeatures(i)
				if err != nil || row != want {
					t.Error("source row lost or target detached", i, row, err)
				}
			}
		})
	}
	wg.Wait()
}

func TestContractConditionInputEmptyAndInvalidReadsPreserveDestination(t *testing.T) {
	plan := executionMaxPlan()
	plan.ConditionCases = nil
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := prepared.InitialContractInput([]TestCase{{0, 10}})
	if err != nil || empty.ConditionCount() != 0 {
		t.Fatal("empty conditions acquired a row", err)
	}
	plan = executionMaxPlan()
	prepared, err = Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	input, err := prepared.InitialContractInput([]TestCase{{0, 10}})
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range []*ContractInput{nil, {}, empty, input} {
		for _, index := range []int{-1, 128} {
			var row [decision.DeclaredConditionFeatureDim]float32
			row[0], row[31] = 99, 42
			before := row
			if view.ConditionFeaturesInto(index, &row) == nil || row != before {
				t.Fatal("invalid read changed destination")
			}
		}
	}
	if input.ConditionFeaturesInto(0, nil) == nil {
		t.Fatal("nil destination accepted")
	}
	if _, err := empty.ConditionCase(0); err == nil {
		t.Fatal("empty channel fabricated a condition")
	}
}
