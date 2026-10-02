package pathplan

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

func TestThreeUnsupportedFourthDecisionIsRetainedAndNeverPredicted(t *testing.T) {
	plan := threePlan()
	plan.Base.Expressions = append(plan.Base.Expressions,
		bodyplan.Expr{Kind: bodyplan.ExprInt, Int: 2},
		bodyplan.Expr{Kind: bodyplan.ExprBinary, Operation: "subtract", Left: 8, Right: 9})
	plan.Base.Statements[2].Expr = 10
	plan.Decisions = append(plan.Decisions, Choice{ID: "fourth", Kind: OperandOrder, Target: 10,
		Intent: "Preserve fourth complete intention.", Fallback: "layout_forward",
		Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}})
	base, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range plan.Decisions {
		fields, e := base.SourceFeatures(c.ID)
		if e != nil {
			t.Fatal(e)
		}
		plan.Decisions[i].Intent, e = decision.EncodeSemanticContextInput(fields, c.Intent)
		if e != nil {
			t.Fatal(e)
		}
	}
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	ctx := threeContext(t)
	cases := []TestCase{{Input: 3, Expected: 999}}
	s, err := p.NewThreeSession(ctx, testThreeModel(t), cases, "")
	if err != nil {
		t.Fatal(err)
	}
	initial, _ := s.Observe()
	decline := initial.Selection.Three
	if decline == nil || !decline.Declined || decline.Decisions != 4 || initial.Declared != 16 || initial.Selection.ModelCalls != 0 || !strings.HasPrefix(decline.Input, "gooo;unsupported3;count=4|") {
		t.Fatal("fourth decision silently dropped or inferred")
	}
	for _, choice := range plan.Decisions {
		if !strings.Contains(decline.Input, choice.Intent) {
			t.Fatal("complete unsupported input missing", choice.ID)
		}
	}
	baseline, _ := p.NewSession(ctx, nil, cases, "")
	actual, _, actualErr := s.Advance(ctx, 16)
	want, _, expectedErr := baseline.Advance(ctx, 16)
	if !reflect.DeepEqual(actual.NewAttempts, want.NewAttempts) || !errors.Is(actualErr, expectedErr) || actual.Attempted != 16 || actual.Selection.ModelCalls != 0 {
		t.Fatal("unsupported arity changed finite deterministic continuation")
	}
}

func TestThreeCanceledOrInvalidRescoringIsAtomic(t *testing.T) {
	p, ctx := threePrepared(t, false), threeContext(t)
	s, err := p.NewThreeSession(ctx, testThreeModel(t), []TestCase{{Input: 3, Expected: 999}}, "")
	if err != nil {
		t.Fatal(err)
	}
	s.Advance(ctx, 1)
	queue := append(searchHeap(nil), s.queue...)
	weights, calls, attempted := s.jointLogWeights, s.result.Selection.ModelCalls, s.attempted
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	probabilities := []float32{0.01, 0.02, 0.03, 0.04, 0.05, 0.06, 0.09, 0.7}
	if err = s.rescoreMaskDistribution(canceled, probabilities); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled rescoring committed")
	}
	for _, invalid := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		probabilities[0] = invalid
		if s.rescoreMaskDistribution(ctx, probabilities) == nil {
			t.Fatal("invalid mask probability accepted")
		}
	}
	if s.rescoreMaskDistribution(ctx, make([]float32, 8)) == nil || s.rescoreMaskDistribution(ctx, []float32{1, 0, 0, 0}) == nil {
		t.Fatal("empty mass or partial mask space accepted")
	}
	if weights != s.jointLogWeights || !reflect.DeepEqual(queue, s.queue) || calls != s.result.Selection.ModelCalls || attempted != s.attempted {
		t.Fatal("failed rescore changed committed progress")
	}
	if _, err = p.NewThreeSession(canceled, testThreeModel(t), []TestCase{{}}, ""); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled construction inferred")
	}
}

func TestThreeConcurrentSessionsShareOnlyImmutableModel(t *testing.T) {
	p, m, ctx := threePrepared(t, false), testThreeModel(t), threeContext(t)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			r, body, _, _, err := p.SearchThreeFeedbackBatches(ctx, m,
				[]TestCase{{Input: 3, Expected: 4}, {Input: 4, Expected: 3}}, 8, 1, "", 7, nil)
			if err != nil || body == nil || r.Status != "TRAINING_COMPLETE" || len(r.Attempts) != 8 || r.Selection.ModelCalls != 7 {
				t.Error("concurrent request lost progress or shared mutable state", err)
			}
		})
	}
	wg.Wait()
}
