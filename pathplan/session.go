package pathplan

import (
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/bits"
	"sync"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

var ErrSessionBusy = errors.New("path session already has an active advance")
var ErrNoTypedCandidate = errors.New("no typed candidate has been evaluated in this session")

// Session keeps one immutable plan/test snapshot and a bounded finite frontier.
// It retains the best body and one bit per scheduled mask, not old attempt logs.
// The caller owns returned progress and may append it to its own evidence store.
// Default sessions rank only at initialization. Explicit Reconsider calls require
// the original frozen model again; model pointers are never retained here.
type Session struct {
	lock            sync.Mutex
	prepared        *PreparedPlan
	cases           []TestCase
	result          SearchResult
	logWeights      [16][2]float64
	fallbackMask    uint16
	ranked          bool
	queue           searchHeap
	scheduled       []uint64
	attempted       int
	committedBits   [16][2]uint16
	best            *bodyplan.Program
	bestPassed      int
	bestCases       []TestResult
	caseSHA         string
	previous        string
	sequence        int
	initialized     bool
	initialError    error
	feedbackRounds  int
	feedbackCalls   int
	feedbackAt      int
	feedbackSHA     string
	joint           bool
	jointLogWeights [4]float64
	jointInput      string
}

type SessionProgress struct {
	Schema                 string            `json:"schema"`
	Sequence               int               `json:"sequence"`
	PreviousSHA            string            `json:"previous_progress_sha256,omitempty"`
	SHA                    string            `json:"progress_sha256,omitempty"`
	CaseSHA                string            `json:"finite_cases_sha256"`
	Status                 string            `json:"status"`
	Selection              Selection         `json:"selection"`
	Declared               int               `json:"declared_combinations"`
	Attempted              int               `json:"cumulative_attempts"`
	Unattempted            int               `json:"unattempted_combinations"`
	Evaluated              int               `json:"cumulative_evaluated_candidates"`
	TypeRejected           int               `json:"cumulative_type_rejected_candidates"`
	SelectedPassed         int               `json:"selected_training_passed"`
	Cases                  int               `json:"training_cases"`
	BestCases              []TestResult      `json:"selected_case_results,omitempty"`
	NewAttempts            []SearchAttempt   `json:"new_attempts,omitempty"`
	InitialProposals       map[string]string `json:"initial_proposals"`
	EligibleProbabilities  [][2]float64      `json:"eligible_probabilities,omitempty"`
	ModelAbstentions       int               `json:"model_abstentions_observed"`
	PredictionsThisAdvance int               `json:"new_local_model_predictions"`
	Exhausted              bool              `json:"space_exhausted"`
	Interrupted            bool              `json:"interrupted"`
	Scope                  string            `json:"scope"`
	Initialized            bool              `json:"ranking_complete"`
	InitializationError    string            `json:"initialization_error,omitempty"`
	ScheduledBytes         int               `json:"scheduled_bitset_bytes"`
	FrontierNodes          int               `json:"frontier_nodes"`
	FrontierStorage        int               `json:"frontier_capacity_bytes"`
	FeedbackRounds         int               `json:"feedback_rounds,omitempty"`
	FeedbackPredictions    int               `json:"feedback_local_model_predictions,omitempty"`
	LatestFeedbackSHA      string            `json:"latest_feedback_sha256,omitempty"`
}

// NewSession ranks once, before any candidate tests. Each Advance has its own
// deadline and 1..64 new-attempt budget. Across advances the finite declared
// space can be exhausted; no model, test case or plan can be changed in place.
func (prepared *PreparedPlan) NewSession(ctx context.Context, model *decision.Model, cases []TestCase, seed string) (*Session, error) {
	if err := searchBounds(ctx, cases, 1); err != nil {
		return nil, err
	}
	if prepared == nil || prepared.fallback == nil || prepared.plan.Base.ResultType != decision.TypeInt {
		return nil, errors.New("session requires a prepared integer path plan")
	}
	if model != nil && model.Schema() != decision.PathMetadataSchema {
		return nil, errors.New("session requires the structural model ABI")
	}
	if seed != "" && (model == nil || len(seed) > 512 || !utf8.ValidString(seed)) {
		return nil, errors.New("seeded session requires a model and bounded seed")
	}
	ownedCases := append([]TestCase(nil), cases...)
	raw, err := json.Marshal(ownedCases)
	if err != nil {
		return nil, err
	}
	defaults := prepared.Defaults()
	result := SearchResult{Schema: "gooo/typed-path-tdd-search/v1", Status: "PARTIAL", DeclaredCombinations: 1 << len(prepared.plan.Decisions), TrainingTotal: len(ownedCases), Selection: Selection{Schema: "gooo/typed-body-path-selection/v1", PlanSHA256: prepared.sha, Choices: defaults, ExternalCallsKnown: true}, InitialProposals: cloneChoices(defaults)}
	result.Unattempted = result.DeclaredCombinations
	if model != nil {
		result.Selection.ModelVariant, result.Selection.MetadataSHA256, result.Selection.WeightsSHA256 = model.Variant(), model.MetadataSHA256(), model.WeightsSHA256()
	}
	if seed != "" {
		result.Selection.SeedSHA256 = hash([]byte(seed))
	}
	session := &Session{prepared: prepared, cases: ownedCases, result: result, bestPassed: -1, caseSHA: hash(raw), ranked: model != nil, scheduled: make([]uint64, (result.DeclaredCombinations+63)/64)}
	var initial uint16
	var workspace decision.Workspace
	for i, choice := range prepared.plan.Decisions {
		if err := ctx.Err(); err != nil {
			session.initialError = err
			return session, err
		}
		if choice.Fallback == choice.Options[1].Label {
			session.fallbackMask |= 1 << i
		}
		receipt := Receipt{ID: choice.ID, Kind: choice.Kind, IntentSHA256: hash([]byte(choice.Intent)), Selected: choice.Fallback, Mode: "declared_fallback"}
		if model != nil {
			var prediction decision.Prediction
			started := time.Now()
			err := model.PredictInto(choice.Intent, &workspace, &prediction)
			receipt.PredictNS = time.Since(started).Nanoseconds()
			session.result.Selection.ModelCalls++
			if err != nil {
				session.initialError = errors.New("session structural ranking prediction failed")
				return session, session.initialError
			}
			receipt.Proposed, receipt.Confidence, receipt.Probabilities = model.PredictLabel(&prediction), prediction.Confidence, prediction.Probabilities
			if prediction.Abstained {
				session.result.ModelAbstentionsObserved++
			}
			weights := eligibleProbabilities(choice, prediction, model.Temperature())
			session.result.EligibleProbabilities = append(session.result.EligibleProbabilities, weights)
			for j, weight := range weights {
				session.logWeights[i][j] = math.Log(math.Max(weight, 1e-12))
			}
			selected := 0
			if weights[1] > weights[0] {
				selected = 1
			}
			if seed != "" {
				label, err := sample(prepared.sha, choice, model.MetadataSHA256(), model.WeightsSHA256(), seed, weights)
				if err != nil {
					session.initialError = err
					return session, err
				}
				if label == choice.Options[1].Label {
					selected = 1
				} else {
					selected = 0
				}
			}
			if selected == 1 {
				initial |= 1 << i
			}
			receipt.Selected, receipt.Mode = choice.Options[selected].Label, "typed_probability_rank_before_finite_validation"
			session.result.InitialProposals[choice.ID] = receipt.Selected
		}
		session.result.Selection.Receipts = append(session.result.Selection.Receipts, receipt)
	}
	if !session.ranked {
		initial = session.fallbackMask
	}
	session.queue = make(searchHeap, 0, min(64, result.DeclaredCombinations))
	session.enqueue(initial)
	if err := ctx.Err(); err != nil {
		session.initialError = err
		return session, err
	}
	session.initialized = true
	return session, nil
}

// Observe returns owned initialization/current facts without model or candidate
// execution. It is also available after a failed initialization so calls made
// before cancellation or a numerical prediction error remain accountable.
func (session *Session) Observe() (SessionProgress, error) {
	return session.observe(false)
}

func (session *Session) observe(interrupted bool) (SessionProgress, error) {
	if session == nil {
		return SessionProgress{}, errors.New("path session is required")
	}
	if !session.lock.TryLock() {
		return SessionProgress{}, ErrSessionBusy
	}
	defer session.lock.Unlock()
	return session.progress(nil, interrupted || session.initialError != nil)
}
func (session *Session) score(mask uint16) float64 {
	if session.joint {
		return session.jointLogWeights[mask]
	}
	if !session.ranked {
		return -float64(bits.OnesCount16(mask ^ session.fallbackMask))
	}
	sum := 0.0
	for i := range session.prepared.plan.Decisions {
		sum += session.logWeights[i][int(mask>>i&1)]
	}
	return sum
}
func (session *Session) enqueue(mask uint16) {
	word, bit := int(mask)/64, uint64(1)<<(mask%64)
	if session.scheduled[word]&bit != 0 {
		return
	}
	session.scheduled[word] |= bit
	heap.Push(&session.queue, searchNode{mask, session.score(mask)})
}

// Advance never blocks on another caller's mutex. Cancellation returns committed
// progress; an interrupted, unfinished candidate is put back on the frontier.
// A trace digest links observations, not authority to change semantic source.
func (session *Session) Advance(ctx context.Context, maxNewAttempts int) (SessionProgress, *bodyplan.Program, error) {
	if err := searchBounds(ctx, []TestCase{{}}, maxNewAttempts); err != nil {
		return SessionProgress{}, nil, err
	}
	if session == nil {
		return SessionProgress{}, nil, errors.New("path session is required")
	}
	if !session.lock.TryLock() {
		return SessionProgress{}, nil, ErrSessionBusy
	}
	defer session.lock.Unlock()
	if !session.initialized {
		progress, err := session.progress(nil, true)
		if err != nil {
			return SessionProgress{}, nil, err
		}
		return progress, nil, session.initialError
	}
	var attempts []SearchAttempt
	var failure error
	for session.queue.Len() > 0 && len(attempts) < maxNewAttempts && session.result.Status != "TRAINING_COMPLETE" {
		if err := ctx.Err(); err != nil {
			failure = err
			break
		}
		node := heap.Pop(&session.queue).(searchNode)
		attempt, program, err := session.evaluate(ctx, node.mask)
		if err != nil {
			heap.Push(&session.queue, node)
			failure = err
			break
		}
		session.attempted++
		// Each coordinate occurs at most 32,768 times in the 16-bit space.
		// Only completed attempts (including type rejections) consume a mask.
		for i := range session.prepared.plan.Decisions {
			session.committedBits[i][int(node.mask>>i&1)]++
		}
		if program == nil {
			session.result.TypeRejected++
		} else {
			session.result.Evaluated++
			if attempt.Passed > session.bestPassed {
				session.bestPassed, session.best = attempt.Passed, program
				session.bestCases = append([]TestResult(nil), attempt.Results...)
				session.result.SelectedTrainingPassed = attempt.Passed
				session.result.Selection.Choices = cloneChoices(attempt.Choices)
				for i := range session.result.Selection.Receipts {
					receipt := &session.result.Selection.Receipts[i]
					receipt.Selected, receipt.Mode = attempt.Choices[receipt.ID], "finite_tdd_selection"
				}
			}
		}
		attempts = append(attempts, attempt)
		if program != nil && attempt.Passed == len(session.cases) {
			session.result.Status = "TRAINING_COMPLETE"
			break
		}
		for i := range session.prepared.plan.Decisions {
			session.enqueue(node.mask ^ (1 << i))
		}
		if session.joint {
			for mask := uint16(0); mask < 4; mask++ {
				session.enqueue(mask)
			}
		}
	}
	if failure == nil && session.best == nil {
		failure = ErrNoTypedCandidate
	}
	progress, err := session.progress(attempts, failure != nil && !errors.Is(failure, ErrNoTypedCandidate))
	if err != nil {
		return SessionProgress{}, session.best, err
	}
	return progress, session.best, failure
}
func (session *Session) evaluate(ctx context.Context, mask uint16) (SearchAttempt, *bodyplan.Program, error) {
	choices := make(map[string]string, len(session.prepared.plan.Decisions))
	for i, choice := range session.prepared.plan.Decisions {
		choices[choice.ID] = choice.Options[int(mask>>i&1)].Label
	}
	attempt := SearchAttempt{Mask: mask, Choices: choices, Total: len(session.cases), Status: "TYPE_REJECTED"}
	program, err := assemble(session.prepared.plan, choices)
	if err != nil {
		return attempt, nil, nil
	}
	attempt.Status, attempt.GoooSHA = "EVALUATED", hash([]byte(program.GoooSource()))
	for _, test := range session.cases {
		if err := ctx.Err(); err != nil {
			return attempt, nil, err
		}
		value, err := program.Evaluate(test.Input)
		if err != nil {
			return attempt, nil, errors.New("finite session interpreter failed")
		}
		passed := value.Int == test.Expected
		if passed {
			attempt.Passed++
		}
		attempt.Results = append(attempt.Results, TestResult{test.Input, test.Expected, value.Int, passed})
	}
	if err := ctx.Err(); err != nil {
		return attempt, nil, err
	}
	return attempt, program, nil
}
func ownedSelection(selection Selection) Selection {
	selection.Choices = cloneChoices(selection.Choices)
	selection.Receipts = append([]Receipt(nil), selection.Receipts...)
	if selection.Joint != nil {
		copy := *selection.Joint
		selection.Joint = &copy
	}
	return selection
}
func (session *Session) progress(attempts []SearchAttempt, interrupted bool) (SessionProgress, error) {
	progress := SessionProgress{Schema: "gooo/typed-path-session-progress/v1", Sequence: session.sequence + 1, PreviousSHA: session.previous, CaseSHA: session.caseSHA, Status: session.result.Status, Selection: ownedSelection(session.result.Selection), Declared: session.result.DeclaredCombinations, Attempted: session.attempted, Unattempted: session.result.DeclaredCombinations - session.attempted, Evaluated: session.result.Evaluated, TypeRejected: session.result.TypeRejected, SelectedPassed: session.result.SelectedTrainingPassed, Cases: len(session.cases), BestCases: append([]TestResult(nil), session.bestCases...), NewAttempts: attempts, InitialProposals: cloneChoices(session.result.InitialProposals), EligibleProbabilities: append([][2]float64(nil), session.result.EligibleProbabilities...), ModelAbstentions: session.result.ModelAbstentionsObserved, Exhausted: session.attempted == session.result.DeclaredCombinations, Interrupted: interrupted, ScheduledBytes: len(session.scheduled) * 8, FrontierNodes: session.queue.Len(), FrontierStorage: cap(session.queue) * int(unsafe.Sizeof(searchNode{})), Scope: "Fixed finite cases and declared typed paths; not general language accuracy or all-input correctness. Digests link observations, not authorization to mutate source."}
	progress.Initialized = session.initialized
	progress.FeedbackRounds, progress.FeedbackPredictions, progress.LatestFeedbackSHA = session.feedbackRounds, session.feedbackCalls, session.feedbackSHA
	if session.initialError != nil {
		progress.InitializationError = session.initialError.Error()
	}
	raw, err := json.Marshal(progress)
	if err != nil {
		return SessionProgress{}, err
	}
	progress.SHA = hash(raw)
	session.sequence++
	session.previous = progress.SHA
	return progress, nil
}

// SearchBatches adapts incremental observations to the existing finite-search
// result. Total attempts remain 1..64 for native document compatibility. Users
// needing a larger declared-space traversal use Session.Advance and stream its
// per-call records instead of retaining them all in this helper.
func (prepared *PreparedPlan) SearchBatches(ctx context.Context, model *decision.Model, cases []TestCase, total, step int, seed string) (SearchResult, *bodyplan.Program, []SessionProgress, error) {
	if err := searchBounds(ctx, cases, total); err != nil {
		return SearchResult{}, nil, nil, err
	}
	if step < 1 || step > 64 {
		return SearchResult{}, nil, nil, errors.New("path session step budget must be 1..64")
	}
	session, startErr := prepared.NewSession(ctx, model, cases, seed)
	if session == nil {
		return SearchResult{}, nil, nil, startErr
	}
	initial, err := session.Observe()
	if err != nil {
		return SearchResult{}, nil, nil, err
	}
	records := []SessionProgress{initial}
	result := sessionSearchResult(initial, nil)
	if startErr != nil {
		return result, nil, records, startErr
	}
	var attempts []SearchAttempt
	var body *bodyplan.Program
	for len(attempts) < total {
		progress, selected, failure := session.Advance(ctx, min(step, total-len(attempts)))
		if progress.Schema != "" {
			attempts = append(attempts, progress.NewAttempts...)
			records = append(records, progress)
			result = sessionSearchResult(progress, attempts)
			body = selected
		}
		if failure != nil && !errors.Is(failure, ErrNoTypedCandidate) {
			return result, body, records, failure
		}
		if progress.Status == "TRAINING_COMPLETE" || progress.Exhausted || len(attempts) >= total {
			return result, body, records, failure
		}
		if len(progress.NewAttempts) == 0 {
			return result, body, records, errors.New("path session made no candidate progress")
		}
	}
	return result, body, records, nil
}
func sessionSearchResult(progress SessionProgress, attempts []SearchAttempt) SearchResult {
	return SearchResult{Schema: "gooo/typed-path-tdd-search/v1", Status: progress.Status, Selection: progress.Selection, DeclaredCombinations: progress.Declared, Unattempted: progress.Unattempted, Evaluated: progress.Evaluated, TypeRejected: progress.TypeRejected, SelectedTrainingPassed: progress.SelectedPassed, TrainingTotal: progress.Cases, Attempts: attempts, ModelAbstentionsObserved: progress.ModelAbstentions, EligibleProbabilities: progress.EligibleProbabilities, InitialProposals: progress.InitialProposals}
}
