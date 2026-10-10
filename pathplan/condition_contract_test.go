package pathplan

import (
	"context"
	"errors"
	"reflect"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func conditionContractPlan() Plan {
	p := conditionObservationPlan()
	p.Decisions = []Choice{p.Decisions[2], p.Decisions[3]}
	p.ConditionCases = []ConditionCase{{ChoiceID: "comparison", Input: -9007199254740995, Expected: false}}
	return p
}

func TestSourceConditionsSearchAndBatchesExcludeFinalOutputOnlySuccess(t *testing.T) {
	plan := conditionContractPlan()
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.ConditionCases[0].Expected = true
	changed, err := Prepare(plan)
	if err != nil || changed.PlanSHA256() == p.PlanSHA256() {
		t.Fatal("condition contract did not bind plan identity", err)
	}
	cases := []TestCase{{-1, -1}, {9007199254740995, 9007199254740995}}
	ctx := sessionContext(t)
	partial, body, err := p.Search(ctx, nil, cases, 1, "")
	if !errors.Is(err, ErrNoConditionCandidate) || body != nil || partial.Evaluated != 1 || partial.ConditionRejected != 1 ||
		partial.TypeRejected != 0 || partial.Attempts[0].Passed != 2 || partial.Attempts[0].Status != "CONDITION_REJECTED" {
		t.Fatal("final success bypassed source condition or lost finite observations", partial, err)
	}
	complete, body, err := p.Search(ctx, nil, cases, 4, "")
	if err != nil || body == nil || complete.Status != "TRAINING_COMPLETE" || complete.ConditionRejected != 1 ||
		complete.Selection.Choices["comparison"] != "layout_reverse" || !ConditionsPassed(complete.Selection.Conditions) {
		t.Fatal(complete, err)
	}
	batched, replay, progress, err := p.SearchBatches(ctx, nil, cases, 4, 1, "")
	if err != nil || replay.GoSource() != body.GoSource() || !reflect.DeepEqual(complete, batched) ||
		len(progress) != 3 || progress[1].ConditionRejected != 1 || progress[1].TypeRejected != 0 {
		t.Fatal("batch continuation differs or lost rejected condition", batched, progress, err)
	}
	selection, body, err := Choose(p.plan, nil, "")
	if !errors.Is(err, ErrNoConditionCandidate) || body != nil || ConditionsPassed(selection.Conditions) {
		t.Fatal("direct fallback ignored conditions", selection, err)
	}
}

func TestSourceConditionsOwnValidationAndInterruptedExecution(t *testing.T) {
	plan := conditionContractPlan()
	for _, cases := range [][]ConditionCase{
		{{ChoiceID: "missing", Input: 0, Expected: false}},
		{{ChoiceID: "comparison", Input: 0}, {ChoiceID: "branches", Input: 0}},
		make([]ConditionCase, 129),
	} {
		plan.ConditionCases = cases
		if _, err := Prepare(plan); err == nil {
			t.Fatal("invalid contract prepared", cases)
		}
		if _, err := Validate(plan); err == nil {
			t.Fatal("invalid contract validated", cases)
		}
	}
	p, err := Prepare(conditionContractPlan())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(sessionContext(t))
	cancel()
	if rows, err := p.CheckDeclaredConditions(ctx, p.Defaults()); !errors.Is(err, context.Canceled) || rows != nil {
		t.Fatal(rows, err)
	}
	if _, err := p.CheckDeclaredConditions(nil, p.Defaults()); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestSourceConditionsCannotFallBackWhenEveryCandidateViolatesThem(t *testing.T) {
	plan := conditionContractPlan()
	plan.ConditionCases = []ConditionCase{{ChoiceID: "comparison", Input: 0, Expected: true}}
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, batch := range []bool{false, true} {
		var result SearchResult
		if batch {
			r, selected, _, failure := p.SearchBatches(sessionContext(t), nil, []TestCase{{0, 0}}, 4, 1, "")
			result, err = r, failure
			if selected != nil {
				t.Fatal("violating body escaped batched search")
			}
		} else {
			r, selected, failure := p.Search(sessionContext(t), nil, []TestCase{{0, 0}}, 4, "")
			result, err = r, failure
			if selected != nil {
				t.Fatal("violating body escaped search")
			}
		}
		if !errors.Is(err, ErrNoConditionCandidate) || result.ConditionRejected != 4 || result.TypeRejected != 0 ||
			result.Evaluated != 4 || result.Unattempted != 0 || result.Status == "TRAINING_COMPLETE" {
			t.Fatal("source conditions were weakened into a best-score fallback", result, err)
		}
	}
}

func TestSourceConditionsConstrainProbesAndCachedResolution(t *testing.T) {
	p, err := Prepare(conditionContractPlan())
	if err != nil {
		t.Fatal(err)
	}
	ctx := sessionContext(t)
	cases := []TestCase{{0, 0}}
	r, err := p.RankProbes(ctx, cases, []int64{-1, 1}, 4)
	if err != nil || !reflect.DeepEqual(r.SurvivingMasks, []uint16{1, 3}) || r.ConditionRejected != 2 ||
		r.ConditionEvaluations != 4 || len(r.ConditionObservations) != 4 || r.CaseRejected != 0 {
		t.Fatal(r, err)
	}
	s, initial, err := p.StartProbeSession(ctx, cases, []int64{-1, 1}, 4)
	if err != nil || !reflect.DeepEqual(initial.Ranking, r) {
		t.Fatal(initial, err)
	}
	next, err := s.AppendObservation(ctx, TestCase{-1, -1})
	if err != nil || next.Ranking.ConditionRejected != 2 || next.Ranking.ConditionEvaluations != 0 ||
		next.Ranking.ReusedConditionEvaluations != 4 || next.TotalConditionEvaluations != 4 ||
		!reflect.DeepEqual(next.Ranking.SurvivingMasks, []uint16{1, 3}) {
		t.Fatal(next, err)
	}
	diagnosis, err := p.Diagnose(ctx, p.Defaults(), cases, []int64{-1}, 4)
	if err != nil || len(diagnosis.Candidates) != 4 || ConditionsPassed(diagnosis.Candidates[0].Conditions) {
		t.Fatal(diagnosis, err)
	}
}

func conditionContextPlan(t *testing.T, three bool) *PreparedPlan {
	t.Helper()
	plan := conditionContractPlan()
	if three {
		plan.Decisions = append(plan.Decisions, conditionObservationPlan().Decisions[0])
	}
	original, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range plan.Decisions {
		fields, err := original.SourceFeatures(c.ID)
		if err != nil {
			t.Fatal(err)
		}
		plan.Decisions[i].Intent, err = decision.EncodeSemanticContextInput(fields, c.Intent)
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSourceConditionsApplyToModelFeedbackJointAndThreeSessions(t *testing.T) {
	ctx := sessionContext(t)
	cases := []TestCase{{-1, -1}, {1, 1}}
	p := conditionContextPlan(t, false)
	model := zeroPathModel(t)
	s, err := p.NewSession(ctx, model, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	first, body, err := s.Advance(ctx, 1)
	if !errors.Is(err, ErrNoConditionCandidate) || body != nil || first.ConditionRejected != 1 {
		t.Fatal(first, err)
	}
	if _, err = s.Reconsider(ctx, model, nil); err != nil {
		t.Fatal(err)
	}
	last, body, err := s.Advance(ctx, 3)
	if err != nil || body == nil || !ConditionsPassed(last.Selection.Conditions) || last.Selection.ModelCalls != 4 {
		t.Fatal(last, err)
	}
	joint, body, _, _, err := p.SearchJointFeedbackBatches(ctx, testJointModel(t), cases, 4, 1, "", 1, nil)
	if err != nil || body == nil || joint.ConditionRejected == 0 || !ConditionsPassed(joint.Selection.Conditions) {
		t.Fatal(joint, err)
	}
	p = conditionContextPlan(t, true)
	three, body, _, _, err := p.SearchThreeFeedbackBatches(ctx, testThreeModel(t), cases, 8, 1, "", 1, nil)
	if err != nil || body == nil || three.ConditionRejected == 0 || !ConditionsPassed(three.Selection.Conditions) {
		t.Fatal(three, err)
	}
}
