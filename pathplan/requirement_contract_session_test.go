package pathplan

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func TestRequirementContractRetainsFailedConditionsAndDeterministicAbsence(t *testing.T) {
	ctx := sessionContext(t)
	plan := executionMaxPlan()
	plan.Base.Expressions[1].Int = 0
	plan.Base.Statements[0].Expr = 1
	plan.Decisions = plan.Decisions[:1]
	plan.ConditionCases[0].Expected = false
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{-9007199254740995, 0}, {9007199254740993, 0}}
	old, err := prepared.NewContractSession(ctx, nil, cases)
	if err != nil {
		t.Fatal(err)
	}
	absent, err := prepared.NewRequirementContractSession(ctx, nil, cases)
	if err != nil || old.Ranking() != absent.Ranking() {
		t.Fatal("deterministic ranking changed", err)
	}
	a, ab, ae := old.Advance(ctx, 2)
	b, bb, be := absent.Advance(ctx, 2)
	if ae != nil || be != nil || a.Status != b.Status || a.Attempted != b.Attempted ||
		!reflect.DeepEqual(a.BestCases, b.BestCases) || ab == nil || bb == nil || ab.GoSource() != bb.GoSource() {
		t.Fatal("deterministic continuation changed", ae, be)
	}
	model, err := contractdecision.NewRequirementConditioned([contractdecision.RequirementParameterCount]float32{}, contractdecision.MeanPooling)
	if err != nil {
		t.Fatal(err)
	}
	s, err := prepared.NewRequirementContractSession(ctx, model, cases)
	if err != nil {
		t.Fatal(err)
	}
	r := s.Ranking()
	initial, err := s.Observe()
	if err != nil || initial.Attempted != 0 || initial.RankingSHA != r.SHA || r.Calls != 1 || !r.Applied || r.Proposed != 0 {
		t.Fatal("initial prediction scope", initial, r, err)
	}
	copyRank := r
	copyRank.SHA = ""
	raw, _ := json.Marshal(copyRank)
	if hash(raw) != r.SHA {
		t.Fatal("condition metadata was added outside the ranking digest")
	}
	first, body, err := s.Advance(ctx, 1)
	if !errors.Is(err, ErrNoConditionCandidate) || body != nil || first.Status != "PARTIAL" || first.Attempted != 1 || first.Selection.ModelCalls != 1 {
		t.Fatal("output-correct condition failure was accepted", first, err)
	}
	last, body, err := s.Advance(ctx, 1)
	if err != nil || body == nil || last.Status != "TRAINING_COMPLETE" || last.Attempted != 2 ||
		last.Selection.ModelCalls != 1 || last.FeedbackPredictions != 0 || s.Ranking() != r ||
		last.Selection.ModelVariant != "requirement_conditioned_contract_fp32" {
		t.Fatal("condition continuation lost its committed ranking", last, err)
	}
	for _, c := range cases {
		value, err := body.Evaluate(c.Input)
		if err != nil || value.Int != c.Expected {
			t.Fatal("constructed output differs", value, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if session, err := prepared.NewRequirementContractSession(cancelled, model, cases); err == nil || session != nil {
		t.Fatal("cancelled initialization exposed session")
	}
}
