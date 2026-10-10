package pathplan

import (
	"container/heap"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"time"
	"unicode/utf8"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
)

// ConditionRanking records one source-bound ranking, including skipped calls.
// Scores are additive factors, not correctness probabilities. ModelFingerprint
// identifies semantic model content; a loader records the artifact file hash.
type ConditionRanking struct {
	Schema             string           `json:"schema"`
	SHA                string           `json:"ranking_sha256,omitempty"`
	PreviousSHA        string           `json:"previous_ranking_sha256,omitempty"`
	FromProgressSHA    string           `json:"from_progress_sha256,omitempty"`
	PlanSHA            string           `json:"plan_sha256"`
	CaseSHA            string           `json:"finite_cases_sha256"`
	ModelFingerprint   string           `json:"model_fingerprint,omitempty"`
	FeatureVersion     string           `json:"feature_version,omitempty"`
	Round              int              `json:"feedback_round"`
	Attempted          int              `json:"prior_attempts"`
	ChoiceCount        int              `json:"choice_count"`
	FeatureSHA         [16]string       `json:"feature_sha256"`
	Logits             [16][2]float32   `json:"choice_logits"`
	Proposed           uint16           `json:"proposed_mask"`
	ObservationAttempt int              `json:"observation_attempt,omitempty"`
	ObservedMask       uint16           `json:"observed_mask"`
	HasFailure         bool             `json:"has_condition_failure"`
	Failure            ConditionFailure `json:"condition_failure"`
	Calls              int              `json:"new_local_model_predictions"`
	CumulativeCalls    int              `json:"cumulative_local_model_predictions"`
	PredictNS          int64            `json:"predict_ns"`
	PredictionValid    bool             `json:"prediction_valid"`
	Applied            bool             `json:"frontier_ranking_applied"`
	AddedMask          bool             `json:"new_proposal_scheduled"`
	Reused             bool             `json:"unchanged_features_reused"`
	Unnecessary        bool             `json:"ranking_unnecessary"`
	Declined           bool             `json:"representation_declined"`
	Error              string           `json:"error,omitempty"`
}

// ConditionProgress binds the ordinary search record and its model receipt.
// Each returned value owns all mutable fields. Search.SHA retains its own chain.
type ConditionProgress struct {
	Schema  string           `json:"schema"`
	SHA     string           `json:"progress_sha256,omitempty"`
	Search  SessionProgress  `json:"search"`
	Ranking ConditionRanking `json:"ranking"`
}

// ConditionSession uses the existing finite frontier, type checks, source
// conditions and output cases. It retains no model pointer or old attempt log.
// Public operations never wait on another operation's mutex.
type ConditionSession struct {
	lock               sync.Mutex
	core               *Session
	latest             ConditionRanking
	applied            ConditionRanking
	input              ConditionInput
	observationAttempt int
	observedMask       uint16
	featureVersion     string
}

// NewConditionSession ranks 1..16 declared binary choices with one neural call.
// A missing model uses the unchanged declared-fallback search. A source shape
// outside the feature representation also falls back, with a decline receipt.
func (prepared *PreparedPlan) NewConditionSession(ctx context.Context, model *conditiondecision.Model, cases []TestCase, seed string) (*ConditionSession, error) {
	if seed != "" && (model == nil || len(seed) > 512 || !utf8.ValidString(seed)) {
		return nil, errors.New("condition seed requires model and bounded UTF-8")
	}
	core, err := prepared.NewSession(ctx, nil, cases, "")
	if core == nil {
		return nil, err
	}
	s := &ConditionSession{core: core, input: ConditionInput{prepared: prepared}}
	if err != nil {
		return s, err
	}
	r := s.receipt()
	if model == nil {
		r.Unnecessary = true
		_, err = s.finish(r, nil)
		return s, err
	}
	r.ModelFingerprint = model.Fingerprint()
	s.featureVersion = model.FeatureVersion()
	if s.featureVersion != decision.ConditionChannelFeatureVersion {
		r.FeatureVersion = s.featureVersion
	}
	core.result.Selection.ModelVariant = "condition_fp32"
	if seed != "" {
		core.result.Selection.SeedSHA256 = hash([]byte(seed))
	}
	var inputs [16][conditiondecision.FeatureDim]float32
	if err = s.features(&inputs, &r); err != nil {
		r.Declined = true
		_, err = s.finish(r, err)
		return s, nil
	}
	if err = ctx.Err(); err != nil {
		core.initialized, core.initialError = false, err
		_, _ = s.finish(r, err)
		return s, err
	}
	err = s.predict(model, inputs[:r.ChoiceCount], &r)
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && seed != "" {
		r.Proposed, err = s.sample(r, seed)
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		core.initialized, core.initialError = false, err
		_, _ = s.finish(r, err)
		return s, err
	}
	core.ranked = true
	core.logWeights = conditionWeights(r.Logits)
	core.queue = core.queue[:0]
	clear(core.scheduled)
	core.enqueue(r.Proposed)
	r.Applied, r.AddedMask = true, true
	for i, choice := range prepared.plan.Decisions {
		label := choice.Options[r.Proposed>>i&1].Label
		core.result.InitialProposals[choice.ID] = label
		core.result.Selection.Receipts[i].Selected = label
		core.result.Selection.Receipts[i].Proposed = label
		core.result.Selection.Receipts[i].Mode = "condition_score_rank_before_finite_validation"
	}
	_, err = s.finish(r, nil)
	return s, err
}

func (s *ConditionSession) receipt() ConditionRanking {
	c := s.core
	return ConditionRanking{Schema: "gooo/condition-path-ranking/v1", PreviousSHA: s.latest.SHA, FromProgressSHA: c.previous,
		PlanSHA: c.prepared.sha, CaseSHA: c.caseSHA, ModelFingerprint: s.latest.ModelFingerprint, FeatureVersion: s.latest.FeatureVersion, Attempted: c.attempted,
		ChoiceCount: len(c.prepared.plan.Decisions), ObservationAttempt: s.observationAttempt, ObservedMask: s.observedMask,
		HasFailure: s.input.present, Failure: s.input.failure}
}

func (s *ConditionSession) features(inputs *[16][conditiondecision.FeatureDim]float32, r *ConditionRanking) error {
	for i, choice := range s.core.prepared.plan.Decisions {
		if err := s.input.FeaturesIntoVersion(choice.ID, s.featureVersion, &inputs[i]); err != nil {
			return err
		}
		var raw [conditiondecision.FeatureDim * 4]byte
		for j, v := range inputs[i] {
			binary.LittleEndian.PutUint32(raw[j*4:], math.Float32bits(v))
		}
		r.FeatureSHA[i] = hash(raw[:])
	}
	return nil
}

func (s *ConditionSession) predict(model *conditiondecision.Model, inputs [][conditiondecision.FeatureDim]float32, r *ConditionRanking) error {
	var workspace conditiondecision.Workspace
	var prediction conditiondecision.ChoicePrediction
	start := time.Now()
	err := model.PredictChoicesInto(inputs, &workspace, &prediction)
	r.Calls, r.PredictNS = 1, time.Since(start).Nanoseconds()
	if err != nil {
		return err
	}
	r.PredictionValid, r.Logits = true, prediction.Logits
	for i := range r.ChoiceCount {
		if r.Logits[i][1] > r.Logits[i][0] {
			r.Proposed |= 1 << i
		}
	}
	return nil
}

func (s *ConditionSession) sample(r ConditionRanking, seed string) (uint16, error) {
	var mask uint16
	for i, choice := range s.core.prepared.plan.Decisions {
		difference := float64(r.Logits[i][1]) - float64(r.Logits[i][0])
		p := 1 / (1 + math.Exp(-difference))
		label, err := sample(s.core.prepared.sha, choice, conditiondecision.Schema, r.ModelFingerprint, seed, [2]float64{1 - p, p})
		if err != nil {
			return 0, err
		}
		if label == choice.Options[1].Label {
			mask |= 1 << i
		}
	}
	return mask, nil
}

func conditionWeights(logits [16][2]float32) (weights [16][2]float64) {
	for i, row := range logits {
		for j, v := range row {
			weights[i][j] = float64(v)
		}
	}
	return weights
}

func (s *ConditionSession) finish(r ConditionRanking, failure error) (ConditionRanking, error) {
	if failure != nil {
		r.Error = failure.Error()
	}
	r.CumulativeCalls = s.core.result.Selection.ModelCalls + r.Calls
	raw, err := json.Marshal(r)
	if err != nil {
		return ConditionRanking{}, err
	}
	r.SHA = hash(raw)
	s.core.result.Selection.ModelCalls = r.CumulativeCalls
	if r.Round > 0 {
		s.core.feedbackRounds = r.Round
		s.core.feedbackCalls += r.Calls
		s.core.feedbackAt = s.core.attempted
		s.core.feedbackSHA = r.SHA
	}
	s.latest = r
	if r.Applied {
		s.applied = r
	}
	return r, failure
}

func (s *ConditionSession) progress(p SessionProgress) (ConditionProgress, error) {
	result := ConditionProgress{Schema: "gooo/condition-path-progress/v1", Search: p, Ranking: s.latest}
	raw, err := json.Marshal(result)
	if err != nil {
		return ConditionProgress{}, err
	}
	result.SHA = hash(raw)
	return result, nil
}

func (s *ConditionSession) Observe() (ConditionProgress, error) {
	if s == nil || s.core == nil {
		return ConditionProgress{}, errors.New("condition session required")
	}
	if !s.lock.TryLock() {
		return ConditionProgress{}, ErrSessionBusy
	}
	defer s.lock.Unlock()
	p, err := s.core.Observe()
	if err != nil {
		return ConditionProgress{}, err
	}
	return s.progress(p)
}

// Advance records the latest committed, typed candidate's condition observation.
// Feature projection reuses that observation without executing the body again.
// An output-only failure clears a previous condition failure; rejected types do
// not supply a new condition observation. All int64 values remain exact.
func (s *ConditionSession) Advance(ctx context.Context, budget int) (ConditionProgress, *bodyplan.Program, error) {
	if s == nil || s.core == nil {
		return ConditionProgress{}, nil, errors.New("condition session required")
	}
	if !s.lock.TryLock() {
		return ConditionProgress{}, nil, ErrSessionBusy
	}
	defer s.lock.Unlock()
	p, body, failure := s.core.Advance(ctx, budget)
	if p.Schema == "" {
		return ConditionProgress{}, body, failure
	}
	for i, attempt := range p.NewAttempts {
		if attempt.Status != "EVALUATED" && attempt.Status != "CONDITION_REJECTED" {
			continue
		}
		s.input.failure, s.input.present = ConditionFailure{}, false
		s.observationAttempt = p.Attempted - len(p.NewAttempts) + i + 1
		s.observedMask = attempt.Mask
		for _, result := range attempt.Conditions {
			if !result.Passed {
				s.input.failure, s.input.present = ConditionFailure{Mask: attempt.Mask, Result: result}, true
				break
			}
		}
	}
	result, err := s.progress(p)
	if err != nil {
		return ConditionProgress{}, body, err
	}
	return result, body, failure
}

// Reconsider uses the same frozen model at most once per newly committed batch,
// for at most 16 rounds. Only frontier scores and an unscheduled proposal may
// change. Unchanged inputs and the last remaining path require no neural call.
func (s *ConditionSession) Reconsider(ctx context.Context, model *conditiondecision.Model) (ConditionRanking, error) {
	if err := searchBounds(ctx, []TestCase{{}}, 1); err != nil {
		return ConditionRanking{}, err
	}
	if s == nil || s.core == nil {
		return ConditionRanking{}, errors.New("condition session required")
	}
	if !s.lock.TryLock() {
		return ConditionRanking{}, ErrSessionBusy
	}
	defer s.lock.Unlock()
	c := s.core
	if model == nil || !c.initialized || !c.ranked || model.Fingerprint() != s.latest.ModelFingerprint {
		return ConditionRanking{}, errors.New("condition feedback requires the successfully initialized original model")
	}
	if c.result.Status == "TRAINING_COMPLETE" || c.attempted == c.result.DeclaredCombinations || c.attempted <= c.feedbackAt || c.feedbackRounds >= 16 {
		return ConditionRanking{}, errors.New("condition feedback requires new partial attempts, remaining paths and fewer than 16 rounds")
	}
	r := s.receipt()
	r.Round = c.feedbackRounds + 1
	if c.result.DeclaredCombinations-c.attempted == 1 {
		r.Unnecessary = true
		return s.finish(r, nil)
	}
	var inputs [16][conditiondecision.FeatureDim]float32
	if err := s.features(&inputs, &r); err != nil {
		r.Declined = true
		return s.finish(r, err)
	}
	if err := ctx.Err(); err != nil {
		return s.finish(r, err)
	}
	if r.FeatureSHA == s.applied.FeatureSHA {
		r.Reused, r.PredictionValid, r.Logits, r.Proposed = true, true, s.applied.Logits, s.applied.Proposed
		return s.finish(r, nil)
	}
	if err := s.predict(model, inputs[:r.ChoiceCount], &r); err != nil {
		return s.finish(r, err)
	}
	weights := conditionWeights(r.Logits)
	score := func(mask uint16) float64 {
		var sum float64
		for i := range r.ChoiceCount {
			sum += weights[i][mask>>i&1]
		}
		return sum
	}
	queue := make(searchHeap, 0, len(c.queue)+1)
	for _, node := range c.queue {
		queue = append(queue, searchNode{node.mask, score(node.mask)})
	}
	word, bit := int(r.Proposed)/64, uint64(1)<<(r.Proposed%64)
	r.AddedMask = c.scheduled[word]&bit == 0
	if r.AddedMask {
		queue = append(queue, searchNode{r.Proposed, score(r.Proposed)})
	}
	heap.Init(&queue)
	if err := ctx.Err(); err != nil {
		r.AddedMask = false
		return s.finish(r, err)
	}
	c.queue, c.logWeights = queue, weights
	if r.AddedMask {
		c.scheduled[word] |= bit
	}
	r.Applied = true
	return s.finish(r, nil)
}
