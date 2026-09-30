package pathplan

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime"
)

func TestTDDSearchRetainsPartialCompletenessAndTypeRejections(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	plan := interactingPlan()
	result, program, err := Search(ctx, plan, nil, []TestCase{{Input: 3, Expected: 17}}, 4, "")
	if err != nil || program == nil || result.Status != "TRAINING_COMPLETE" || result.Evaluated != 2 || result.Selection.ModelCalls != 0 || result.Unattempted != 2 {
		t.Fatalf("search: %+v %v", result, err)
	}
	partial, program, err := Search(ctx, plan, nil, []TestCase{{Input: 3, Expected: 999}}, 4, "")
	if err != nil || program == nil || partial.Status != "PARTIAL" || partial.TypeRejected != 1 || partial.Evaluated != 3 || len(partial.Attempts) != 4 || partial.Unattempted != 0 {
		t.Fatalf("partial: %+v %v", partial, err)
	}
	if _, _, err := Search(context.Background(), plan, nil, []TestCase{{Input: 3, Expected: 17}}, 4, ""); err == nil {
		t.Fatal("unbounded search accepted")
	}
	if _, _, err := Search(ctx, plan, nil, []TestCase{{Input: 3, Expected: 17}}, 65, ""); err == nil {
		t.Fatal("oversized search budget accepted")
	}
	if _, _, err := Search(ctx, plan, nil, []TestCase{{Input: 3, Expected: 17}}, 4, "seed"); err == nil {
		t.Fatal("model-free sampling accepted")
	}
}

func TestEligibleRankingSurvivesGlobalProbabilityUnderflow(t *testing.T) {
	choice := Choice{Options: []Option{{Label: "layout_reverse"}, {Label: "layout_forward"}}}
	prediction := decision.Prediction{}
	prediction.Logits[4], prediction.Logits[5], prediction.Logits[0] = -1000, -998, 1000
	weights := eligibleProbabilities(choice, prediction, 2)
	if math.Abs(weights[0]-1/(1+math.Exp(-1))) > 1e-12 || math.Abs(weights[0]+weights[1]-1) > 1e-12 {
		t.Fatalf("conditional weights: %v", weights)
	}
}
