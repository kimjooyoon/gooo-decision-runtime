package pathplan

import (
	"container/heap"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime"
)

var ErrFeedbackContextBound = errors.New("feedback context exceeds the model input bound")

// CIHint is caller-supplied context, never permission to edit semantic state.
// SourceSHA records what the caller claims was checked; this API does not verify CI.
type CIHint struct {
	SourceSHA string `json:"source_sha"`
	Status    string `json:"status"`
}

func (hint *CIHint) Validate() error {
	if hint == nil {
		return nil
	}
	raw, err := hex.DecodeString(hint.SourceSHA)
	if err != nil || len(raw) != 20 || hint.SourceSHA != hex.EncodeToString(raw) ||
		(hint.Status != "PASS" && hint.Status != "FAIL" && hint.Status != "UNKNOWN") {
		return errors.New("CI hint requires a lowercase source SHA and PASS, FAIL or UNKNOWN")
	}
	return nil
}

type FeedbackJudgment struct {
	DecisionID string     `json:"decision_id"`
	Input      string     `json:"input"`
	InputSHA   string     `json:"input_sha256"`
	Prediction Receipt    `json:"prediction"`
	Eligible   [2]float64 `json:"eligible_probabilities"`
}

type FeedbackReceipt struct {
	Schema             string             `json:"schema"`
	Round              int                `json:"round"`
	PreviousSHA        string             `json:"previous_feedback_sha256,omitempty"`
	SHA                string             `json:"feedback_sha256,omitempty"`
	FromProgressSHA    string             `json:"from_progress_sha256,omitempty"`
	PlanSHA            string             `json:"plan_sha256"`
	CaseSHA            string             `json:"finite_cases_sha256"`
	MetadataSHA        string             `json:"model_metadata_sha256"`
	WeightsSHA         string             `json:"model_weights_sha256"`
	Attempted          int                `json:"prior_attempts"`
	Passed             int                `json:"prior_selected_passed"`
	Cases              int                `json:"finite_cases"`
	TypeRejected       int                `json:"prior_type_rejections"`
	FirstFailure       *TestResult        `json:"first_selected_failure,omitempty"`
	CI                 *CIHint            `json:"caller_ci_hint,omitempty"`
	CIIsAuthority      bool               `json:"ci_hint_is_authority"`
	Judgments          []FeedbackJudgment `json:"judgments,omitempty"`
	ModelCalls         int                `json:"new_local_model_predictions"`
	CumulativeCalls    int                `json:"cumulative_local_model_predictions"`
	Applied            bool               `json:"frontier_ranking_applied"`
	AddedMask          bool               `json:"new_proposal_scheduled"`
	Error              string             `json:"error,omitempty"`
	ContextDeclined    bool               `json:"context_declined,omitempty"`
	DeclinedDecision   string             `json:"declined_decision_id,omitempty"`
	DeclinedBytes      int                `json:"declined_input_bytes,omitempty"`
	DeclinedInputSHA   string             `json:"declined_input_sha256,omitempty"`
	DeclinedIntentSHA  string             `json:"declined_intent_sha256,omitempty"`
	RankingUnnecessary bool               `json:"ranking_unnecessary,omitempty"`
	Scope              string             `json:"scope"`
}

// Reconsider re-ranks only unattempted paths with the original frozen model.
// It changes neither cases, intent, source, best body nor committed attempts.
// One call per newly committed batch and at most 16 rounds prevent retry loops.
// Cancellation leaves ranking unchanged but records predictions already made.
func (session *Session) Reconsider(ctx context.Context, model *decision.Model, ci *CIHint) (FeedbackReceipt, error) {
	if err := searchBounds(ctx, []TestCase{{}}, 1); err != nil {
		return FeedbackReceipt{}, err
	}
	if session == nil {
		return FeedbackReceipt{}, errors.New("path session is required")
	}
	if !session.lock.TryLock() {
		return FeedbackReceipt{}, ErrSessionBusy
	}
	defer session.lock.Unlock()
	if err := ci.Validate(); err != nil {
		return FeedbackReceipt{}, err
	}
	if !session.initialized || model == nil || !session.ranked || model.Schema() != decision.PathMetadataSchema ||
		model.MetadataSHA256() != session.result.Selection.MetadataSHA256 || model.WeightsSHA256() != session.result.Selection.WeightsSHA256 {
		return FeedbackReceipt{}, errors.New("feedback requires the successfully initialized original structural model")
	}
	if session.result.Status == "TRAINING_COMPLETE" || session.attempted == session.result.DeclaredCombinations ||
		session.attempted <= session.feedbackAt || session.feedbackRounds >= 16 {
		return FeedbackReceipt{}, errors.New("feedback requires a new partial batch, remaining paths and fewer than 16 rounds")
	}
	receipt := FeedbackReceipt{Schema: "gooo/typed-path-feedback-judgment/v1", Round: session.feedbackRounds + 1,
		PreviousSHA: session.feedbackSHA, FromProgressSHA: session.previous, PlanSHA: session.prepared.sha, CaseSHA: session.caseSHA,
		MetadataSHA: model.MetadataSHA256(), WeightsSHA: model.WeightsSHA256(), Attempted: session.attempted,
		Passed: session.result.SelectedTrainingPassed, Cases: len(session.cases), TypeRejected: session.result.TypeRejected,
		Scope: "Finite observed failures condition the original frozen model; ranking hints are not acceptance, online learning, general language correctness or semantic edit authority."}
	if ci != nil {
		copy := *ci
		receipt.CI = &copy
	}
	for _, result := range session.bestCases {
		if !result.Passed {
			copy := result
			receipt.FirstFailure = &copy
			break
		}
	}
	prefix := fmt.Sprintf("feedback: tried=%d passed=%d/%d rejected=%d remaining=%d", session.attempted,
		receipt.Passed, receipt.Cases, receipt.TypeRejected, session.result.DeclaredCombinations-session.attempted)
	if receipt.FirstFailure != nil {
		prefix += fmt.Sprintf(" mismatch=%d:%d:%d", receipt.FirstFailure.Input, receipt.FirstFailure.Actual, receipt.FirstFailure.Expected)
	}
	if ci != nil {
		prefix += " ci=" + ci.Status
	}
	finish := func(failure error) (FeedbackReceipt, error) {
		if failure != nil {
			receipt.Error = failure.Error()
		}
		receipt.CumulativeCalls = session.result.Selection.ModelCalls
		if receipt.ModelCalls == 0 && !receipt.ContextDeclined && !receipt.RankingUnnecessary {
			return receipt, failure
		}
		raw, err := json.Marshal(receipt)
		if err != nil {
			return receipt, err
		}
		receipt.SHA = hash(raw)
		session.feedbackRounds, session.feedbackAt, session.feedbackSHA = receipt.Round, session.attempted, receipt.SHA
		return receipt, failure
	}
	// With one unattempted declared path, probabilities cannot change which path
	// is evaluated next. Preserve the observed failure and original model binding
	// without inference or a frontier rewrite; this observation consumes a round.
	if session.result.DeclaredCombinations-session.attempted == 1 {
		if err := ctx.Err(); err != nil {
			return finish(err)
		}
		receipt.RankingUnnecessary = true
		return finish(nil)
	}
	// A representation decline is observed once per new partial batch. It leaves
	// the frontier untouched and counts toward the round budget without inference.
	var inputs [16]string
	for i, choice := range session.prepared.plan.Decisions {
		inputs[i] = prefix + " selected=" + session.result.Selection.Choices[choice.ID] + "\nintent: " + choice.Intent
		if len(inputs[i]) > decision.InputMaxBytes {
			receipt.ContextDeclined, receipt.DeclinedDecision, receipt.DeclinedBytes = true, choice.ID, len(inputs[i])
			receipt.DeclinedInputSHA, receipt.DeclinedIntentSHA = hash([]byte(inputs[i])), hash([]byte(choice.Intent))
			return finish(fmt.Errorf("%w; original intent was not truncated", ErrFeedbackContextBound))
		}
	}
	var workspace decision.Workspace
	var weights [16][2]float64
	var proposal uint16
	for i, choice := range session.prepared.plan.Decisions {
		if err := ctx.Err(); err != nil {
			return finish(err)
		}
		var prediction decision.Prediction
		started := time.Now()
		err := model.PredictInto(inputs[i], &workspace, &prediction)
		receipt.ModelCalls++
		session.feedbackCalls++
		session.result.Selection.ModelCalls++
		if err != nil {
			return finish(errors.New("feedback structural prediction failed"))
		}
		probabilities := eligibleProbabilities(choice, prediction, model.Temperature())
		selected := 0
		if probabilities[1] > probabilities[0] {
			selected = 1
		}
		if selected == 1 {
			proposal |= 1 << i
		}
		for j, p := range probabilities {
			weights[i][j] = math.Log(math.Max(p, 1e-12))
		}
		if prediction.Abstained {
			session.result.ModelAbstentionsObserved++
		}
		receipt.Judgments = append(receipt.Judgments, FeedbackJudgment{DecisionID: choice.ID, Input: inputs[i], InputSHA: hash([]byte(inputs[i])), Eligible: probabilities,
			Prediction: Receipt{ID: choice.ID, Kind: choice.Kind, IntentSHA256: hash([]byte(choice.Intent)), Mode: "feedback_probability_rank_before_finite_validation",
				Proposed: model.PredictLabel(&prediction), Selected: choice.Options[selected].Label, Confidence: prediction.Confidence, Probabilities: prediction.Probabilities, PredictNS: time.Since(started).Nanoseconds()}})
	}
	// Build a temporary heap so cancellation cannot leave a partly re-scored queue.
	queue := make(searchHeap, len(session.queue), min(session.result.DeclaredCombinations, max(cap(session.queue), len(session.queue)+1)))
	score := func(mask uint16) float64 {
		var sum float64
		for i := range session.prepared.plan.Decisions {
			sum += weights[i][int(mask>>i&1)]
		}
		return sum
	}
	for i, node := range session.queue {
		if i%64 == 0 {
			if err := ctx.Err(); err != nil {
				return finish(err)
			}
		}
		queue[i] = searchNode{node.mask, score(node.mask)}
	}
	word, bit := int(proposal)/64, uint64(1)<<(proposal%64)
	if session.scheduled[word]&bit == 0 {
		queue = append(queue, searchNode{proposal, score(proposal)})
		receipt.AddedMask = true
	}
	heap.Init(&queue)
	if err := ctx.Err(); err != nil {
		receipt.AddedMask = false
		return finish(err)
	}
	session.logWeights, session.queue = weights, queue
	if receipt.AddedMask {
		session.scheduled[word] |= bit
	}
	receipt.Applied = true
	return finish(nil)
}
