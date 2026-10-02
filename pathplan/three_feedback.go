package pathplan

import (
	"context"
	"errors"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
)

// ReconsiderThree ranks only remaining complete masks from actual committed
// failures. Busy calls never wait; cancellation does not partially change scores.
func (session *Session) ReconsiderThree(ctx context.Context, model *jointdecision.ThreeModel, ci *CIHint) (FeedbackReceipt, error) {
	if err := searchBounds(ctx, []TestCase{{}}, 1); err != nil {
		return FeedbackReceipt{}, err
	}
	if session == nil {
		return FeedbackReceipt{}, errors.New("three-choice session required")
	}
	if !session.lock.TryLock() {
		return FeedbackReceipt{}, ErrSessionBusy
	}
	defer session.lock.Unlock()
	if err := ci.Validate(); err != nil {
		return FeedbackReceipt{}, err
	}
	if !session.initialized || !session.joint || session.result.DeclaredCombinations != 8 || model == nil || model.MetadataSHA256() != session.result.Selection.MetadataSHA256 || model.WeightsSHA256() != session.result.Selection.WeightsSHA256 {
		return FeedbackReceipt{}, errors.New("three-choice feedback requires original initialized model")
	}
	if session.result.Status == "TRAINING_COMPLETE" || session.attempted == 8 || session.attempted <= session.feedbackAt || session.feedbackRounds >= 16 {
		return FeedbackReceipt{}, errors.New("three-choice feedback requires new partial progress and remaining masks")
	}
	r := session.newJointFeedback(ci)
	r.Schema = "gooo/typed-path-three-choice-feedback-judgment/v1"
	if 8-session.attempted == 1 {
		if err := ctx.Err(); err != nil {
			return session.finishJointFeedback(r, err)
		}
		r.RankingUnnecessary = true
		return session.finishJointFeedback(r, nil)
	}
	text, parts, err := jointdecision.FeedbackThreeWithParts(session.jointInput, session.jointFeedbackPrefix(r))
	if err != nil {
		return session.finishJointFeedback(r, err)
	}
	r3 := threeReceipt(text, 3, model)
	for i, part := range parts {
		r3.PartSHA[i] = hash([]byte(part))
	}
	r.Three = &r3
	var features [jointdecision.ThreeFeatureDim]float32
	if err = jointdecision.FeaturesIntoThree(text, &features); err != nil {
		r3.Declined, r3.Error = true, err.Error()
		r.ContextDeclined, r.DeclinedDecision, r.DeclinedBytes = true, "three-choice", len(text)
		r.DeclinedInputSHA, r.DeclinedIntentSHA = hash([]byte(text)), hash([]byte(session.jointInput))
		return session.finishJointFeedback(r, ErrFeedbackContextBound)
	}
	if err = ctx.Err(); err != nil {
		return session.finishJointFeedback(r, err)
	}
	var workspace jointdecision.ThreeWorkspace
	var prediction jointdecision.ThreePrediction
	start := time.Now()
	err = model.PredictInto(text, &workspace, &prediction)
	r3.PredictNS, r3.Calls = time.Since(start).Nanoseconds(), 1
	r.ModelCalls = 1
	session.feedbackCalls++
	session.result.Selection.ModelCalls++
	r3.Probabilities, r3.Proposed, r3.Sampled, r3.PredictionValid = prediction.Probabilities, prediction.Mask, prediction.Mask, err == nil
	if err != nil {
		return session.finishJointFeedback(r, err)
	}
	if err = session.rescoreMaskDistribution(ctx, prediction.Probabilities[:]); err != nil {
		return session.finishJointFeedback(r, err)
	}
	r.Applied = true
	return session.finishJointFeedback(r, nil)
}
