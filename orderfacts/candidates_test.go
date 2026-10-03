package orderfacts

import (
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func threeFixture() pathplan.Plan {
	p := fixture()
	for i, target := range []int{3, 5} {
		p.Decisions = append(p.Decisions, pathplan.Choice{ID: []string{"first", "second"}[i],
			Kind: pathplan.OperandOrder, Target: target, Intent: "Keep operands.",
			Options:  []pathplan.Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}},
			Fallback: "layout_forward"})
	}
	return p
}

func TestCandidatesComposeInteractingChoices(t *testing.T) {
	p := threeFixture()
	p.Base.Expressions[3].Operation = "subtract"
	facts, err := Candidates(p)
	if err != nil {
		t.Fatal(err)
	}
	if facts[0] == facts[1] || facts[0] == facts[2] || facts[1] == facts[3] {
		t.Fatal("root or subtraction operand choice was lost")
	}
	for mask := range 4 {
		if facts[mask] != facts[mask|4] {
			t.Fatal("multiplication swap changed canonical facts")
		}
	}
	if facts[0][8] != 2 || facts[1][28] != 2 || facts[2][9] == facts[0][9] {
		t.Fatal("composed operation positions or subtraction leaves differ")
	}
	// Reordering declarations remaps mask bits, not candidate meaning.
	p.Decisions[0], p.Decisions[2] = p.Decisions[2], p.Decisions[0]
	reordered, err := Candidates(p)
	if err != nil {
		t.Fatal(err)
	}
	for mask := range 8 {
		mapped := ((mask & 1) << 2) | (mask & 2) | ((mask & 4) >> 2)
		if reordered[mask] != facts[mapped] {
			t.Fatal("mask used a hardcoded decision order")
		}
	}
}

func TestCandidateFailureReturnsNoPartialFacts(t *testing.T) {
	p := threeFixture()
	p.Decisions[2].Options[1].Reverse = false
	if facts, err := Candidates(p); err == nil || facts != ([8]Signature{}) {
		t.Fatal("invalid plan produced partial candidate descriptors")
	}
	p = threeFixture()
	p.Base.Expressions[5].Left = 3 // valid nested expression, outside this descriptor.
	if facts, err := Candidates(p); err == nil || facts != ([8]Signature{}) {
		t.Fatal("unsupported nested expression accepted")
	}
}
