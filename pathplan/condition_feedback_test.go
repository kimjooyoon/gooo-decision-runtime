package pathplan

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
)

func TestConditionFailureIsPassedToFrozenModelFeedback(t *testing.T) {
	ctx := sessionContext(t)
	p := conditionContextPlan(t, false)
	model := zeroPathModel(t)
	s, err := p.NewSession(ctx, model, []TestCase{{Input: -1, Expected: -1}}, "")
	if err != nil {
		t.Fatal(err)
	}
	progress, _, err := s.Advance(ctx, 1)
	if !errors.Is(err, ErrNoConditionCandidate) || progress.NewAttempts[0].Passed != 1 {
		t.Fatal("need output-correct condition failure", progress, err)
	}
	r, err := s.Reconsider(ctx, model, &CIHint{SourceSHA: strings.Repeat("a", 40), Status: "FAIL"})
	if err != nil {
		t.Fatal(err)
	}
	if r.ModelCalls != 2 || len(r.Judgments) != 2 || r.FirstConditionFailure == nil {
		t.Fatal("feedback did not make the declared predictions", r)
	}
	for _, judgment := range r.Judgments {
		for _, part := range []string{"condition_rejected=1", "condition_input=-9007199254740995", "condition_expected=false", "condition_actual=true", "condition_status=MISMATCH", "ci=FAIL"} {
			if !strings.Contains(judgment.Input, part) {
				t.Fatalf("model did not receive %q: %s", part, judgment.Input)
			}
		}
		if judgment.InputSHA != hash([]byte(judgment.Input)) {
			t.Fatal("input identity does not include condition evidence")
		}
	}
	raw, _ := json.Marshal(r)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["prior_condition_rejections"]) != "1" || len(fields["first_condition_failure"]) == 0 {
		t.Fatal("feedback lost condition evidence")
	}
	want := *r.FirstConditionFailure
	r.FirstConditionFailure.Result.Case.Input = 9
	progress.NewAttempts[0].Conditions[0].Case.Input = 10
	var copied FeedbackReceipt
	s.bindConditionFeedback(&copied)
	if *copied.FirstConditionFailure != want {
		t.Fatal("returned evidence mutated the retained counterexample")
	}
}

func TestConditionCounterexampleReachesJointAndThreeModelFeatures(t *testing.T) {
	for _, three := range []bool{false, true} {
		t.Run(map[bool]string{false: "joint", true: "three"}[three], func(t *testing.T) {
			ctx := sessionContext(t)
			p := conditionContextPlan(t, three)
			cases := []TestCase{{Input: -1, Expected: -1}}
			var s *Session
			var reconsider func() (FeedbackReceipt, error)
			var err error
			if three {
				m := testThreeModel(t)
				s, err = p.NewThreeSession(ctx, m, cases, "")
				reconsider = func() (FeedbackReceipt, error) { return s.ReconsiderThree(ctx, m, nil) }
			} else {
				m := testJointModel(t)
				s, err = p.NewJointSession(ctx, m, cases, "")
				reconsider = func() (FeedbackReceipt, error) { return s.ReconsiderJoint(ctx, m, nil) }
			}
			if err != nil {
				t.Fatal(err)
			}
			first, _, err := s.Advance(ctx, 1)
			if !errors.Is(err, ErrNoConditionCandidate) || first.ConditionRejected != 1 {
				t.Fatal(first, err)
			}
			r, err := reconsider()
			if err != nil || r.ModelCalls != 1 || !r.Applied || r.FirstConditionFailure == nil {
				t.Fatal(r, err)
			}
			var parts []string
			if three {
				decoded, err := jointdecision.ThreeParts(r.Three.Input)
				if err != nil {
					t.Fatal(err)
				}
				parts = decoded[:]
				for i, part := range parts {
					if r.Three.PartSHA[i] != hash([]byte(part)) {
						t.Fatal("three-part identity lost failure context")
					}
				}
			} else {
				decoded, err := jointdecision.Parts(r.Joint.Input)
				if err != nil {
					t.Fatal(err)
				}
				parts = decoded[:]
			}
			for i, part := range parts {
				if !strings.HasPrefix(part, p.plan.Decisions[i].Intent+"\nfeedback:") ||
					!strings.Contains(part, "condition_input=-9007199254740995") ||
					!strings.Contains(part, "condition_status=MISMATCH") {
					t.Fatal("model did not receive original intent and condition failure", part)
				}
				var initial, next [decision.FeatureDim]float32
				if err := decision.SemanticContextFeaturesInto(p.plan.Decisions[i].Intent, &initial); err != nil {
					t.Fatal(err)
				}
				if err := decision.SemanticContextFeaturesInto(part, &next); err != nil ||
					!reflect.DeepEqual(initial[:64], next[:64]) || reflect.DeepEqual(initial[64:], next[64:]) {
					t.Fatal("feedback changed source features or never reached the natural channel", err)
				}
			}
		})
	}
}

func TestUnreachedConditionFeedbackAndCanceledCandidate(t *testing.T) {
	plan := conditionObservationPlan()
	plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "int", Int: -10},
		bodyplan.Expr{Kind: "binary", Operation: "less_than", Left: 0, Right: 5})
	plan.Base.Statements[2].Then = []int{3}
	plan.Base.Statements = append(plan.Base.Statements,
		bodyplan.Stmt{Kind: "if", Expr: 6, Then: []int{0}}, bodyplan.Stmt{Kind: "return", Expr: 0})
	plan.Base.Root = []int{2, 4}
	plan.Decisions = []Choice{plan.Decisions[2], {ID: "nested", Kind: BranchLayout, Target: 3,
		Intent: "Observe nested condition.", Fallback: "layout_forward",
		Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}}}
	plan.ConditionCases = []ConditionCase{{ChoiceID: "nested", Input: 1, Expected: false}}
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	ctx, m := sessionContext(t), zeroPathModel(t)
	s, err := p.NewSession(ctx, m, []TestCase{{1, 1}}, "")
	if err != nil {
		t.Fatal(err)
	}
	progress, _, err := s.Advance(&interruptedSessionContext{Context: ctx}, 1)
	if !errors.Is(err, context.Canceled) || progress.Attempted != 0 || s.firstConditionFailure != nil ||
		s.result.ConditionRejected != 0 {
		t.Fatal("interrupted candidate supplied feedback evidence", progress, err)
	}
	progress, _, err = s.Advance(ctx, 1)
	if !errors.Is(err, ErrNoConditionCandidate) || progress.NewAttempts[0].Conditions[0].Status != "NOT_REACHED" {
		t.Fatal(progress, err)
	}
	r, err := s.Reconsider(ctx, m, nil)
	if err != nil || r.ModelCalls != 2 {
		t.Fatal(r, err)
	}
	for _, judgment := range r.Judgments {
		if !strings.Contains(judgment.Input, "condition_actual=UNOBSERVED condition_status=NOT_REACHED") {
			t.Fatal("unreached condition was turned into a Boolean", judgment.Input)
		}
	}
}

func TestConditionFeedbackAbsentAndOversizedContext(t *testing.T) {
	empty := FeedbackReceipt{Attempted: 1, Passed: 0, Cases: 1, TypeRejected: 0}
	if got := feedbackPrefix(empty, 3); got != "feedback: tried=1 passed=0/1 rejected=0 remaining=3" {
		t.Fatal("old feedback prefix changed", got)
	}
	raw, err := json.Marshal(empty)
	if err != nil || strings.Contains(string(raw), "condition") {
		t.Fatal("absent condition evidence changed serialization", string(raw), err)
	}
	plan := conditionContractPlan()
	for i := range plan.Decisions {
		plan.Decisions[i].Intent = strings.Repeat("x", 364)
	}
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	ctx, m := sessionContext(t), zeroPathModel(t)
	s, err := p.NewSession(ctx, m, []TestCase{{-1, -1}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Advance(ctx, 1); !errors.Is(err, ErrNoConditionCandidate) {
		t.Fatal(err)
	}
	queue, weights := append(searchHeap(nil), s.queue...), s.logWeights
	r, err := s.Reconsider(ctx, m, nil)
	if !errors.Is(err, ErrFeedbackContextBound) || !r.ContextDeclined || r.ModelCalls != 0 ||
		r.FirstConditionFailure == nil || r.DeclinedBytes <= decision.InputMaxBytes || r.DeclinedInputSHA == "" ||
		!reflect.DeepEqual(queue, s.queue) || weights != s.logWeights {
		t.Fatal("oversized condition context inferred, truncated or changed frontier", r, err)
	}
}
