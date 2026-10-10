package pathplan

import (
	"encoding/json"
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestDeclaredContractDistinguishesGoalsWithSameInitialSource(t *testing.T) {
	p, err := Prepare(executionMaxPlan())
	if err != nil {
		t.Fatal(err)
	}
	maxCases := []TestCase{{-9007199254740995, 10}, {9007199254740993, 9007199254740993}}
	minCases := []TestCase{{-9007199254740995, -9007199254740995}, {9007199254740993, 10}}
	a, err := p.InitialContractInput(maxCases)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.InitialContractInput(minCases)
	if err != nil {
		t.Fatal(err)
	}
	if a.PlanSHA256() != b.PlanSHA256() || a.CaseSHA256() == b.CaseSHA256() {
		t.Fatal("source/goal identity separation")
	}
	legacy, err := p.InitialExecutionInput(maxCases)
	if err != nil {
		t.Fatal(err)
	}
	for _, choice := range p.plan.Decisions {
		var first, second, old [384]float32
		if a.RelationalSourceFeaturesInto(choice.ID, &first) != nil || b.RelationalSourceFeaturesInto(choice.ID, &second) != nil || legacy.ExecutionRelationalFlowFeaturesInto(choice.ID, &old) != nil {
			t.Fatal("source projection")
		}
		if first != second || first != old {
			t.Fatal("old source ABI changed")
		}
		for i, v := range first {
			if ((i >= 192 && i < 236) || (i >= 256 && i < 320)) && v != 0 {
				t.Fatal("declared goal became an execution failure")
			}
		}
	}
	for i := range maxCases {
		var first, second [32]float32
		if a.CaseFeaturesInto(i, &first) != nil || b.CaseFeaturesInto(i, &second) != nil || first == second {
			t.Fatal("different requirements remain indistinguishable")
		}
	}
}

func TestDeclaredContractOwnsAll128CasesAndConcurrentReaders(t *testing.T) {
	p, err := Prepare(executionMaxPlan())
	if err != nil {
		t.Fatal(err)
	}
	cases := make([]TestCase, 128)
	for i := range cases {
		cases[i] = TestCase{Input: int64(i), Expected: 9007199254740993 + int64(i)}
	}
	// Preserve duplicates and conflicting declarations for the owning verifier.
	cases[1].Input = cases[0].Input
	owned := append([]TestCase(nil), cases...)
	input, err := p.InitialContractInput(cases)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(owned)
	if input.CaseCount() != 128 || input.CaseSHA256() != hash(raw) {
		t.Fatal("complete declared suite identity")
	}
	clear(cases)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for i, c := range owned {
				var got, want [32]float32
				_ = decision.DeclaredCaseFeaturesInto(c.Input, c.Expected, &want)
				if input.CaseFeaturesInto(i, &got) != nil || got != want {
					t.Error("case lost, truncated or aliased", i)
				}
			}
		})
	}
	wg.Wait()
}

func TestDeclaredContractBoundsAndErrorsPreserveDestinations(t *testing.T) {
	p, err := Prepare(executionMaxPlan())
	if err != nil {
		t.Fatal(err)
	}
	for _, cases := range [][]TestCase{nil, make([]TestCase, 129)} {
		if input, err := p.InitialContractInput(cases); err == nil || input != nil {
			t.Fatal("case bound")
		}
	}
	var missing *PreparedPlan
	if input, err := missing.InitialContractInput([]TestCase{{0, 0}}); err == nil || input != nil {
		t.Fatal("nil source")
	}
	input, err := p.InitialContractInput([]TestCase{{0, 0}})
	if err != nil {
		t.Fatal(err)
	}
	var nilInput *ContractInput
	if nilInput.CaseCount() != 0 || nilInput.CaseSHA256() != "" || nilInput.PlanSHA256() != "" {
		t.Fatal("nil identity")
	}
	var original [32]float32
	original[0] = 99
	for _, view := range []*ContractInput{nilInput, {}, input} {
		for _, index := range []int{-1, 128} {
			destination := original
			if view.CaseFeaturesInto(index, &destination) == nil || destination != original {
				t.Fatal("case error mutated destination")
			}
		}
	}
	if input.CaseFeaturesInto(0, nil) == nil {
		t.Fatal("nil destination")
	}
	var source [384]float32
	source[0] = 99
	before := source
	if input.RelationalSourceFeaturesInto("missing", &source) == nil || source != before {
		t.Fatal("source error mutated destination")
	}
	if nilInput.RelationalSourceFeaturesInto("branches", &source) == nil || source != before {
		t.Fatal("nil source view")
	}
}
