package pathplan

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestFeedbackBatchesRetainBodyAfterBoundedContextDeclines(t *testing.T) {
	ctx := sessionContext(t)
	plan := interactingPlan()
	plan.Decisions[0].Intent = strings.Repeat("a", 480)
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	model := zeroPathModel(t)
	cases := []TestCase{{Input: 3, Expected: 999}}
	baseline, before, _, err := prepared.SearchBatches(ctx, model, cases, 4, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	result, body, progress, declines, err := prepared.SearchFeedbackBatches(ctx, model, cases, 4, 1, "", 2, nil)
	if err != nil || body == nil || body.GoSource() != before.GoSource() || result.SelectedTrainingPassed != baseline.SelectedTrainingPassed ||
		len(result.Attempts) != 4 || result.Selection.ModelCalls != 2 || len(declines) != 2 || len(progress) != 7 {
		t.Fatal("recoverable representation bound prevented continued finite construction", err)
	}
	seen := map[uint16]bool{}
	for _, attempt := range result.Attempts {
		if seen[attempt.Mask] {
			t.Fatal("decline repeated candidate")
		}
		seen[attempt.Mask] = true
	}
	for i, decline := range declines {
		if !decline.ContextDeclined || decline.ModelCalls != 0 || decline.Applied || decline.SHA == "" || decline.Round != i+1 || decline.CumulativeCalls != 2 || decline.DeclinedInputSHA == "" {
			t.Fatal("decline lost bound, call or receipt accounting")
		}
	}
	last := progress[len(progress)-1]
	if last.Interrupted || last.FeedbackPredictions != 0 || last.FeedbackRounds != 2 || last.LatestFeedbackSHA != declines[1].SHA {
		t.Fatal("decline interrupted owned progress")
	}
}

func TestFeedbackBatchesPreserveZeroRoundContractAndBoundNewPredictions(t *testing.T) {
	ctx := sessionContext(t)
	prepared, err := Prepare(interactingPlan())
	if err != nil {
		t.Fatal(err)
	}
	model := zeroPathModel(t)
	cases := []TestCase{{Input: 3, Expected: 999}}
	old, oldBody, oldProgress, err := prepared.SearchBatches(ctx, nil, cases, 4, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	value, body, progress, feedback, err := prepared.SearchFeedbackBatches(ctx, nil, cases, 4, 1, "", 0, nil)
	if err != nil || !reflect.DeepEqual(old, value) || !reflect.DeepEqual(oldProgress, progress) || oldBody.GoSource() != body.GoSource() || len(feedback) != 0 {
		t.Fatal("zero-round helper changed offline behavior")
	}
	value, body, progress, feedback, err = prepared.SearchFeedbackBatches(ctx, model, cases, 4, 1, "", 2, nil)
	if err != nil || body == nil || len(feedback) != 2 || len(value.Attempts) != 4 || value.SelectedTrainingPassed != 0 || value.TrainingTotal != 1 || value.Unattempted != 0 || value.Selection.ModelCalls != 6 {
		t.Fatal("feedback helper lost finite or actual-call accounting")
	}
	seen := map[uint16]bool{}
	previous := ""
	for i, record := range progress {
		if record.Sequence != i+1 || record.PreviousSHA != previous {
			t.Fatal("broken observation chain")
		}
		previous = record.SHA
		for _, attempt := range record.NewAttempts {
			if seen[attempt.Mask] {
				t.Fatal("repeated committed mask")
			}
			seen[attempt.Mask] = true
		}
	}
	if len(seen) != 4 || progress[len(progress)-1].FeedbackPredictions != 4 || progress[len(progress)-1].LatestFeedbackSHA != feedback[1].SHA {
		t.Fatal("feedback linkage differs")
	}
	for _, budget := range []struct{ total, step, rounds int }{{65, 1, 1}, {4, 0, 1}, {4, 65, 1}, {4, 1, 17}, {4, 1, -1}} {
		if _, _, _, _, err = prepared.SearchFeedbackBatches(ctx, model, cases, budget.total, budget.step, "", budget.rounds, nil); err == nil {
			t.Fatal("invalid helper bounds accepted")
		}
	}
	if _, _, _, _, err = prepared.SearchFeedbackBatches(ctx, nil, cases, 4, 1, "", 1, nil); err == nil {
		t.Fatal("feedback without a model accepted")
	}
}

type canceledFeedbackBatchContext struct {
	context.Context
	checks int
}

func (c *canceledFeedbackBatchContext) Err() error {
	c.checks++
	if c.checks >= 13 {
		return context.Canceled
	}
	return nil
}
func TestFeedbackBatchesKeepCanceledPredictionEvidenceAndPartialBody(t *testing.T) {
	ctx := sessionContext(t)
	prepared, _ := Prepare(interactingPlan())
	model := zeroPathModel(t)
	// The deterministic fixture cancels after two feedback predictions and before
	// committing its temporary heap. This is cancellation accounting, not timing.
	value, body, progress, feedback, err := prepared.SearchFeedbackBatches(&canceledFeedbackBatchContext{Context: ctx}, model, []TestCase{{Input: 3, Expected: 999}}, 4, 1, "", 2, nil)
	if !errors.Is(err, context.Canceled) || body == nil || len(value.Attempts) != 1 || len(feedback) != 1 || feedback[0].Applied || feedback[0].ModelCalls != 2 || value.Selection.ModelCalls != 4 || len(progress) != 3 || !progress[2].Interrupted || progress[2].LatestFeedbackSHA != feedback[0].SHA || progress[2].FeedbackPredictions != 2 {
		t.Fatalf("canceled feedback hidden: %v %+v", err, value)
	}
}
