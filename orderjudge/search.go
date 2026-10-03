package orderjudge

import (
	"context"
	"encoding/hex"
	"errors"
	"math/bits"
	"strings"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/orderfacts"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type Alias struct {
	Mask        uint8 `json:"mask"`
	EvaluatedAs uint8 `json:"evaluated_as"`
}

// SearchReceipt records one whole-candidate prediction. Its normalized scores
// are rankings, not calibrated probabilities of functional completeness.
type SearchReceipt struct {
	Schema       string     `json:"schema"`
	Mode         string     `json:"mode"`
	IntentSHA256 string     `json:"intent_sha256"`
	Descriptors  [8]string  `json:"descriptors"`
	Prediction   Prediction `json:"prediction"`
	PredictNS    int64      `json:"predict_ns"`
	Deduplicate  bool       `json:"deduplicate_equal_descriptors"`
	Aliases      []Alias    `json:"skipped_equal_descriptors,omitempty"`
}

// Search prepares and ranks all admitted candidates before evaluating cases.
// Source binding belongs to the compiler caller. Evaluation is bounded by 1..8
// actual bodies and 1..128 finite cases. Equal descriptors can skip an already
// evaluated body; skipped masks remain unattempted in the ordinary search record.
// A nil model retains deterministic fallback-distance order and zero predictions.
// Callers may share an immutable model; every search owns its workspace/results.
func Search(ctx context.Context, plan pathplan.Plan, model *Model, cases []pathplan.TestCase,
	maxAttempts int, deduplicate bool) (pathplan.SearchResult, *bodyplan.Program, *SearchReceipt, error) {
	var result pathplan.SearchResult
	if ctx == nil {
		return result, nil, nil, errors.New("search context required")
	}
	if _, ok := ctx.Deadline(); !ok || maxAttempts < 1 || maxAttempts > 8 || len(cases) == 0 || len(cases) > 128 {
		return result, nil, nil, errors.New("deadline, 1..8 attempts and 1..128 cases required")
	}
	if err := ctx.Err(); err != nil {
		return result, nil, nil, err
	}
	prepared, err := pathplan.Prepare(plan)
	if err != nil {
		return result, nil, nil, err
	}
	descriptors, err := orderfacts.Candidates(plan)
	if err != nil {
		return result, nil, nil, err
	}
	receipt := &SearchReceipt{Schema: "gooo/two-update-candidate-search/v1", Mode: "deterministic_fallback_distance", Deduplicate: deduplicate}
	var intents []string
	var fallback uint8
	for i, c := range plan.Decisions {
		intents = append(intents, c.Intent)
		if c.Fallback == c.Options[1].Label {
			fallback |= 1 << i
		}
	}
	intent := strings.Join(intents, "\n")
	receipt.IntentSHA256 = digest([]byte(intent))
	for mask := range 8 {
		receipt.Descriptors[mask] = hex.EncodeToString(descriptors[mask][:])
		receipt.Prediction.Ranking[mask] = uint8(mask)
	}
	for i := 1; i < 8; i++ {
		for j := i; j > 0 && bits.OnesCount8(receipt.Prediction.Ranking[j]^fallback) < bits.OnesCount8(receipt.Prediction.Ranking[j-1]^fallback); j-- {
			receipt.Prediction.Ranking[j], receipt.Prediction.Ranking[j-1] = receipt.Prediction.Ranking[j-1], receipt.Prediction.Ranking[j]
		}
	}
	result = pathplan.SearchResult{Schema: "gooo/typed-path-tdd-search/v1", Status: "PARTIAL", DeclaredCombinations: 8,
		Unattempted: 8, TrainingTotal: len(cases), Selection: pathplan.Selection{Schema: "gooo/typed-body-path-selection/v1",
			PlanSHA256: prepared.PlanSHA256(), Choices: prepared.Defaults(), ExternalCallsKnown: true}}
	if model != nil {
		var features [IntentDim]float32
		var candidates [8][SourceDim]float32
		if err := IntentFeatures(intent, &features); err != nil {
			return result, nil, receipt, err
		}
		for i, d := range descriptors {
			if err := SourceFeatures(d, &candidates[i]); err != nil {
				return result, nil, receipt, err
			}
		}
		metadata, weights, err := model.Marshal()
		if err != nil {
			return result, nil, receipt, err
		}
		result.Selection.ModelVariant = Schema
		result.Selection.MetadataSHA256, result.Selection.WeightsSHA256 = digest(metadata), digest(weights)
		if err := ctx.Err(); err != nil {
			return result, nil, receipt, err
		}
		var work Workspace
		started := time.Now()
		result.Selection.ModelCalls++
		err = model.Predict(&features, &candidates, &work, &receipt.Prediction)
		receipt.PredictNS = time.Since(started).Nanoseconds()
		if err != nil {
			return result, nil, receipt, err
		}
		receipt.Mode = "whole_candidate_rank_before_finite_validation"
	}
	choicesFor := func(mask uint8) map[string]string {
		choices := make(map[string]string, 3)
		for bit, c := range plan.Decisions {
			choices[c.ID] = c.Options[(mask>>bit)&1].Label
		}
		return choices
	}
	result.InitialProposals = choicesFor(receipt.Prediction.Ranking[0])
	for _, c := range plan.Decisions {
		result.Selection.Receipts = append(result.Selection.Receipts, pathplan.Receipt{ID: c.ID, Kind: c.Kind,
			IntentSHA256: digest([]byte(c.Intent)), Mode: receipt.Mode, Proposed: result.InitialProposals[c.ID], Selected: c.Fallback})
	}
	var evaluated [8]bool
	var best *bodyplan.Program
	bestPassed := -1
	for _, mask := range receipt.Prediction.Ranking {
		if result.Evaluated >= maxAttempts {
			break
		}
		if err := ctx.Err(); err != nil {
			return result, best, receipt, err
		}
		alias := -1
		if deduplicate {
			for previous, wasEvaluated := range evaluated {
				if wasEvaluated && descriptors[previous] == descriptors[mask] {
					alias = previous
					break
				}
			}
		}
		if alias >= 0 {
			receipt.Aliases = append(receipt.Aliases, Alias{mask, uint8(alias)})
			continue
		}
		choices := choicesFor(mask)
		program, err := prepared.Compile(choices)
		if err != nil {
			return result, best, receipt, err
		}
		attempt := pathplan.SearchAttempt{Mask: uint16(mask), Choices: choices, Status: "EVALUATED",
			Total: len(cases), GoooSHA: digest([]byte(program.GoooSource()))}
		for _, c := range cases {
			if err := ctx.Err(); err != nil {
				return result, best, receipt, err
			}
			v, err := program.Evaluate(c.Input)
			if err != nil {
				return result, best, receipt, err
			}
			passed := v.Int == c.Expected
			if passed {
				attempt.Passed++
			}
			attempt.Results = append(attempt.Results, pathplan.TestResult{Input: c.Input, Expected: c.Expected, Actual: v.Int, Passed: passed})
		}
		evaluated[mask] = true
		result.Evaluated++
		result.Attempts = append(result.Attempts, attempt)
		result.Unattempted = 8 - result.Evaluated
		if attempt.Passed > bestPassed {
			bestPassed, best = attempt.Passed, program
			result.SelectedTrainingPassed, result.Selection.Choices = bestPassed, choices
			for i := range result.Selection.Receipts {
				r := &result.Selection.Receipts[i]
				r.Selected, r.Mode = choices[r.ID], "finite_tdd_selection"
			}
		}
		if attempt.Passed == len(cases) {
			result.Status = "TRAINING_COMPLETE"
			break
		}
	}
	return result, best, receipt, nil
}
