package pathplan

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestFeedbackKeepsCasesBestBodyAndNeverRepeatsCommittedMasks(t *testing.T) {
	ctx := sessionContext(t)
	prepared, err := Prepare(interactingPlan())
	if err != nil {
		t.Fatal(err)
	}
	model := zeroPathModel(t)
	session, err := prepared.NewSession(ctx, model, []TestCase{{Input: 3, Expected: 999}}, "")
	if err != nil {
		t.Fatal(err)
	}
	initial, _ := session.Observe()
	first, best, err := session.Advance(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	before := best.GoSource()
	hint := &CIHint{SourceSHA: strings.Repeat("a", 40), Status: "FAIL"}
	receipt, err := session.Reconsider(ctx, model, hint)
	if err != nil || !receipt.Applied || receipt.ModelCalls != 2 || receipt.CumulativeCalls != 4 || receipt.FromProgressSHA != first.SHA || receipt.CaseSHA != initial.CaseSHA || receipt.FirstFailure == nil || receipt.CIIsAuthority {
		t.Fatalf("feedback accounting differs: %+v %v", receipt, err)
	}
	digest := receipt.SHA
	copy := receipt
	copy.SHA = ""
	raw, _ := json.Marshal(copy)
	if digest != hash(raw) {
		t.Fatal("feedback digest does not bind the observation")
	}
	for _, judgment := range receipt.Judgments {
		if judgment.InputSHA != hash([]byte(judgment.Input)) || !strings.Contains(judgment.Input, "mismatch=3:16:999") || !strings.Contains(judgment.Input, "ci=FAIL") {
			t.Fatal("observed failure or CI hint was not passed to the model")
		}
	}
	hint.Status = "PASS"
	receipt.Judgments[0].Input = "mutated"
	receipt.FirstFailure.Expected = 16
	if _, err := session.Reconsider(ctx, model, nil); err == nil {
		t.Fatal("same failed batch was retried")
	}
	last, body, err := session.Advance(ctx, 3)
	if err != nil || !last.Exhausted || last.Status != "PARTIAL" || last.SelectedPassed != 0 || last.Cases != 1 ||
		last.FeedbackPredictions != 2 || last.FeedbackRounds != 1 || last.LatestFeedbackSHA != digest || last.Selection.ModelCalls != 4 || last.PredictionsThisAdvance != 0 || body.GoSource() != before {
		t.Fatal("feedback changed cases, best partial body, call accounting or completeness")
	}
	seen := map[uint16]bool{first.NewAttempts[0].Mask: true}
	for _, attempt := range last.NewAttempts {
		if seen[attempt.Mask] {
			t.Fatal("feedback repeated a committed mask")
		}
		seen[attempt.Mask] = true
	}
	if len(seen) != 4 {
		t.Fatal("feedback lost finite alternatives")
	}
	if _, err := session.Reconsider(ctx, model, nil); err == nil {
		t.Fatal("exhausted session performed feedback")
	}
}

func TestCanceledFeedbackAccountsCallsWithoutChangingFrontier(t *testing.T) {
	ctx := sessionContext(t)
	prepared, _ := Prepare(interactingPlan())
	model := zeroPathModel(t)
	session, err := prepared.NewSession(ctx, model, []TestCase{{Input: 3, Expected: 999}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = session.Advance(ctx, 1); err != nil {
		t.Fatal(err)
	}
	queue := append(searchHeap(nil), session.queue...)
	weights := session.logWeights
	scheduled := append([]uint64(nil), session.scheduled...)
	receipt, err := session.Reconsider(&interruptedSessionContext{Context: ctx}, model, nil)
	if !errors.Is(err, context.Canceled) || receipt.ModelCalls != 2 || receipt.Applied || receipt.AddedMask || receipt.SHA == "" || receipt.Error == "" {
		t.Fatalf("cancelled calls were hidden or committed: %+v %v", receipt, err)
	}
	if !reflect.DeepEqual(queue, session.queue) || weights != session.logWeights || !reflect.DeepEqual(scheduled, session.scheduled) {
		t.Fatal("cancelled feedback mutated candidate ranking")
	}
	if _, err = session.Reconsider(ctx, model, nil); err == nil {
		t.Fatal("cancelled predictions were silently retried")
	}
	progress, _, err := session.Advance(ctx, 1)
	if err != nil || progress.FeedbackPredictions != 2 || progress.LatestFeedbackSHA != receipt.SHA {
		t.Fatal("cancelled feedback could not resume ordinary search")
	}
	next, err := session.Reconsider(ctx, model, nil)
	if err != nil || next.PreviousSHA != receipt.SHA || next.Round != 2 {
		t.Fatal("new committed batch could not be reconsidered")
	}
}

func TestFeedbackBoundsRejectBeforeCallingModel(t *testing.T) {
	ctx := sessionContext(t)
	model := zeroPathModel(t)
	prepared, _ := Prepare(interactingPlan())
	session, _ := prepared.NewSession(ctx, model, []TestCase{{Input: 3, Expected: 999}}, "")
	if _, err := session.Reconsider(ctx, model, nil); err == nil {
		t.Fatal("no observations were accepted as feedback")
	}
	if _, _, err := session.Advance(ctx, 1); err != nil {
		t.Fatal(err)
	}
	for _, hint := range []*CIHint{{SourceSHA: strings.Repeat("a", 40), Status: "approved"}, {SourceSHA: strings.Repeat("A", 40), Status: "PASS"}, {SourceSHA: "short", Status: "FAIL"}} {
		if _, err := session.Reconsider(ctx, model, hint); err == nil {
			t.Fatal("invalid CI context accepted")
		}
	}
	session.lock.Lock()
	_, err := session.Reconsider(ctx, model, nil)
	session.lock.Unlock()
	if !errors.Is(err, ErrSessionBusy) {
		t.Fatal("feedback waited on another caller")
	}
	if _, err := session.Reconsider(context.Background(), model, nil); err == nil {
		t.Fatal("unbounded context accepted")
	}
	if _, err := session.Reconsider(ctx, nil, nil); err == nil {
		t.Fatal("missing model accepted")
	}
	session.result.Selection.MetadataSHA256 = "changed"
	if _, err := session.Reconsider(ctx, model, nil); err == nil {
		t.Fatal("changed model identity accepted")
	}
	if session.feedbackCalls != 0 || session.feedbackRounds != 0 {
		t.Fatal("invalid feedback called the model")
	}
	plan := interactingPlan()
	plan.Decisions[0].Intent = strings.Repeat("a", 512)
	prepared, err = Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	session, err = prepared.NewSession(ctx, model, []TestCase{{Input: 3, Expected: 999}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = session.Advance(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = session.Reconsider(ctx, model, nil); err == nil || session.feedbackCalls != 0 {
		t.Fatal("oversized feedback truncated intent or called the model")
	}
}

func TestFeedbackHasSixteenRoundCapAndNoModelFreeSideEffects(t *testing.T) {
	ctx := sessionContext(t)
	model := zeroPathModel(t)
	prepared, _ := Prepare(chainedDecisionPlan(7))
	session, err := prepared.NewSession(ctx, model, []TestCase{{Input: 3, Expected: 999}}, "")
	if err != nil {
		t.Fatal(err)
	}
	for range 16 {
		if _, _, err = session.Advance(ctx, 1); err != nil {
			t.Fatal(err)
		}
		if _, err = session.Reconsider(ctx, model, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err = session.Advance(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = session.Reconsider(ctx, model, nil); err == nil || session.feedbackCalls != 112 {
		t.Fatal("unbounded feedback loop accepted")
	}
	offline, _ := prepared.NewSession(ctx, nil, []TestCase{{Input: 3, Expected: 999}}, "")
	if _, _, err = offline.Advance(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = offline.Reconsider(ctx, model, nil); err == nil || offline.result.Selection.ModelCalls != 0 {
		t.Fatal("offline session gained model calls")
	}
}
