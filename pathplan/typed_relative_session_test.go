package pathplan

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func TestTypedSourceRelativeSessionRanksOnceThenChecksSourceRequirements(t *testing.T) {
	plan := orderedArithmeticPlan()
	plan.Base.Expressions = []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 487},
		{Kind: "int", Int: 17}, {Kind: "int", Int: 31}, {Kind: "binary", Operation: "less_than", Left: 0, Right: 1},
		{Kind: "binary", Operation: "multiply", Left: 0, Right: 2}, {Kind: "binary", Operation: "subtract", Left: 0, Right: 3}}
	plan.Base.Statements[0].Expr, plan.Base.Statements[1].Expr, plan.Base.Statements[2].Expr = 5, 6, 4
	plan.Decisions[0].Target = 4
	plan.Decisions[0].Intent, plan.Decisions[1].Intent = "487보다 작은지 비교", "작으면 17배, 아니면 31을 뺀다"
	plan.ConditionCases = []ConditionCase{{plan.Decisions[0].ID, 486, true}, {plan.Decisions[0].ID, 487, false}, {plan.Decisions[0].ID, 488, false}}
	cases := []TestCase{{-5, -85}, {486, 8262}, {487, 456}, {901, 870}}
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	var weights [2][contractdecision.TypedSourceRelativeHeadParameterCount]float32
	// Synthetic biases deliberately favor the wrong comparison initially.
	weights[0][contractdecision.TypedSourceRelativeHeadParameterCount-1] = 2
	weights[1][contractdecision.TypedSourceRelativeHeadParameterCount-2] = 1
	m, err := contractdecision.NewTypedSourceRelative(weights)
	if err != nil {
		t.Fatal(err)
	}
	ctx := sessionContext(t)
	s, err := p.NewTypedSourceRelativeContractSession(ctx, m, cases)
	if err != nil {
		t.Fatal(err)
	}
	r := s.Ranking()
	if r.Calls != 1 || !r.Applied || r.Schema != "gooo/typed-source-relative-path-ranking/v1" || r.ModelFingerprint != m.Fingerprint() || r.Proposed != 1 {
		t.Fatal("typed ranking", r)
	}
	initial, err := s.Observe()
	if err != nil || initial.Attempted != 0 {
		t.Fatal("prediction executed candidate", err)
	}
	input, err := p.InitialContractInput(cases)
	if err != nil {
		t.Fatal(err)
	}
	for i, choice := range p.plan.Decisions {
		var row [528]float32
		if err := input.SourceRelativeV4FeaturesInto(choice.ID, &row); err != nil {
			t.Fatal(err)
		}
		var raw [528 * 4]byte
		for j, value := range row {
			binary.LittleEndian.PutUint32(raw[j*4:], math.Float32bits(value))
		}
		if r.FeatureSHA[i] != hash(raw[:]) {
			t.Fatal("typed receipt input differs")
		}
	}
	first, _, err := s.Advance(ctx, 1)
	if !errors.Is(err, ErrNoConditionCandidate) || first.Status != "PARTIAL" || first.ConditionRejected != 1 || first.PredictionsThisAdvance != 0 {
		t.Fatal("unchecked initial proposal", err, first)
	}
	progress, body, err := s.Advance(ctx, 3)
	if err != nil || body == nil || progress.Status != "TRAINING_COMPLETE" || progress.PredictionsThisAdvance != 0 || s.Ranking() != r || progress.Selection.ModelVariant != "typed_source_relative_contract_fp32" {
		t.Fatal("finite completion", err, progress)
	}
	for _, test := range cases {
		value, err := body.Evaluate(test.Input)
		if err != nil || value.Int != test.Expected {
			t.Fatal("source requirement", err, test)
		}
	}
	plain, err := p.NewContractSession(ctx, nil, cases)
	if err != nil {
		t.Fatal(err)
	}
	without, err := p.NewTypedSourceRelativeContractSession(ctx, nil, cases)
	if err != nil || !reflect.DeepEqual(plain.Ranking(), without.Ranking()) {
		t.Fatal("deterministic omission", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := p.NewTypedSourceRelativeContractSession(cancelled, m, cases); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled session", err)
	}
	plan.Base.Expressions[6].Left = 5
	unsupported, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	declined, err := unsupported.NewTypedSourceRelativeContractSession(ctx, m, cases)
	if err != nil || !declined.Ranking().Declined || declined.Ranking().Calls != 0 || declined.Ranking().Error == "" {
		t.Fatal("unsupported source called model", err)
	}
}
