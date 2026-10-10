package pathplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
	"time"
)

func feedbackPrefix(r FeedbackReceipt, remaining int) string {
	text := fmt.Sprintf("feedback: tried=%d passed=%d/%d rejected=%d remaining=%d", r.Attempted, r.Passed, r.Cases, r.TypeRejected, remaining)
	if r.FirstFailure != nil {
		c := r.FirstFailure
		text += fmt.Sprintf(" mismatch=%d:%d:%d", c.Input, c.Actual, c.Expected)
	}
	if r.CI != nil {
		text += " ci=" + r.CI.Status
	}
	return text + conditionFeedbackSuffix(r)
}
func (session *Session) newJointFeedback(ci *CIHint) FeedbackReceipt {
	r := FeedbackReceipt{Schema: "gooo/typed-path-joint-feedback-judgment/v1", Round: session.feedbackRounds + 1, PreviousSHA: session.feedbackSHA, FromProgressSHA: session.previous, PlanSHA: session.prepared.sha, CaseSHA: session.caseSHA, MetadataSHA: session.result.Selection.MetadataSHA256, WeightsSHA: session.result.Selection.WeightsSHA256, Attempted: session.attempted, Passed: session.result.SelectedTrainingPassed, Cases: len(session.cases), TypeRejected: session.result.TypeRejected, Scope: "Observed finite failures condition the original joint model; tests decide acceptance; no online learning or semantic edit authority."}
	if ci != nil {
		copy := *ci
		r.CI = &copy
	}
	session.bindConditionFeedback(&r)
	for _, c := range session.bestCases {
		if !c.Passed {
			copy := c
			r.FirstFailure = &copy
			break
		}
	}
	return r
}
func (session *Session) finishJointFeedback(r FeedbackReceipt, failure error) (FeedbackReceipt, error) {
	if failure != nil {
		r.Error = failure.Error()
	}
	r.CumulativeCalls = session.result.Selection.ModelCalls
	if r.ModelCalls == 0 && !r.ContextDeclined && !r.RankingUnnecessary {
		return r, failure
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	r.SHA = hash(raw)
	session.feedbackRounds, session.feedbackAt, session.feedbackSHA = r.Round, session.attempted, r.SHA
	return r, failure
}
func (session *Session) jointFeedbackBounds(model *jointdecision.Model) error {
	if !session.initialized || !session.joint || session.result.DeclaredCombinations != 4 || model == nil || model.MetadataSHA256() != session.result.Selection.MetadataSHA256 || model.WeightsSHA256() != session.result.Selection.WeightsSHA256 {
		return errors.New("joint feedback requires original initialized model")
	}
	if session.result.Status == "TRAINING_COMPLETE" || session.attempted == 4 || session.attempted <= session.feedbackAt || session.feedbackRounds >= 16 {
		return errors.New("joint feedback requires new partial progress and remaining paths")
	}
	return nil
}

// ReconsiderJoint makes one real call for the full path distribution. Source
// headers/intents are preserved, failed masks stay excluded, and cancellation
// commits no partial frontier. A sole remaining mask needs zero predictions.
func (session *Session) ReconsiderJoint(ctx context.Context, model *jointdecision.Model, ci *CIHint) (FeedbackReceipt, error) {
	if err := searchBounds(ctx, []TestCase{{}}, 1); err != nil {
		return FeedbackReceipt{}, err
	}
	if session == nil {
		return FeedbackReceipt{}, errors.New("joint session required")
	}
	if !session.lock.TryLock() {
		return FeedbackReceipt{}, ErrSessionBusy
	}
	defer session.lock.Unlock()
	if err := ci.Validate(); err != nil {
		return FeedbackReceipt{}, err
	}
	if err := session.jointFeedbackBounds(model); err != nil {
		return FeedbackReceipt{}, err
	}
	r := session.newJointFeedback(ci)
	if 4-session.attempted == 1 {
		if err := ctx.Err(); err != nil {
			return session.finishJointFeedback(r, err)
		}
		r.RankingUnnecessary = true
		return session.finishJointFeedback(r, nil)
	}
	text, err := jointdecision.Feedback(session.jointInput, session.jointFeedbackPrefix(r))
	if err != nil {
		return session.finishJointFeedback(r, err)
	}
	rj := jointReceipt(text)
	r.Joint = &rj
	var features [jointdecision.FeatureDim]float32
	if err = jointdecision.FeaturesInto(text, &features); err != nil {
		rj.Declined, rj.Error = true, err.Error()
		r.ContextDeclined, r.DeclinedDecision, r.DeclinedBytes = true, "joint", len(text)
		r.DeclinedInputSHA, r.DeclinedIntentSHA = hash([]byte(text)), hash([]byte(session.jointInput))
		return session.finishJointFeedback(r, ErrFeedbackContextBound)
	}
	if err = ctx.Err(); err != nil {
		return session.finishJointFeedback(r, err)
	}
	var workspace jointdecision.Workspace
	var prediction jointdecision.Prediction
	start := time.Now()
	err = model.PredictInto(text, &workspace, &prediction)
	rj.PredictNS, rj.Calls = time.Since(start).Nanoseconds(), 1
	r.ModelCalls = 1
	session.feedbackCalls++
	session.result.Selection.ModelCalls++
	rj.Probabilities, rj.Proposed, rj.Sampled = prediction.Probabilities, prediction.Mask, prediction.Mask
	if err != nil {
		return session.finishJointFeedback(r, err)
	}
	if err = session.rescoreJoint(ctx, prediction.Probabilities); err != nil {
		return session.finishJointFeedback(r, err)
	}
	r.Applied = true
	return session.finishJointFeedback(r, nil)
}
