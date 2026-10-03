package orderprepared

import (
	"context"
	"errors"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/orderjudge"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

// Search requires the caller's freshly source-bound plan digest. It predicts
// once per model call, then evaluates this call's finite cases. No prediction or
// test outcome is reused. The returned Program wrapper is caller-owned.
func (p *Prepared) Search(ctx context.Context, boundPlanSHA string, cases []pathplan.TestCase,
	maxAttempts int, deduplicate bool) (pathplan.SearchResult, *bodyplan.Program, *orderjudge.SearchReceipt, error) {
	var result pathplan.SearchResult
	if err := checkContext(ctx); err != nil {
		return result, nil, nil, err
	}
	if p == nil || p.planSHA == "" || boundPlanSHA != p.planSHA {
		return result, nil, nil, errors.New("prepared candidates require matching source-bound plan digest")
	}
	if maxAttempts < 1 || maxAttempts > 8 || len(cases) == 0 || len(cases) > 128 {
		return result, nil, nil, errors.New("1..8 attempts and 1..128 cases required")
	}
	receipt := &orderjudge.SearchReceipt{Schema: "gooo/two-update-candidate-search/v1",
		Mode: "deterministic_fallback_distance", IntentSHA256: p.intentSHA, Descriptors: p.encoded,
		Prediction: orderjudge.Prediction{Ranking: p.ranking}, Deduplicate: deduplicate}
	result = pathplan.SearchResult{Schema: "gooo/typed-path-tdd-search/v1", Status: "PARTIAL", DeclaredCombinations: 8,
		Unattempted: 8, TrainingTotal: len(cases), Selection: pathplan.Selection{Schema: "gooo/typed-body-path-selection/v1",
			PlanSHA256: p.planSHA, Choices: p.selected(p.fallback), ExternalCallsKnown: true}}
	if p.runtime.model != nil {
		result.Selection.ModelVariant = orderjudge.Schema
		result.Selection.MetadataSHA256, result.Selection.WeightsSHA256 = p.runtime.metadataSHA, p.runtime.weightsSHA
		var workspace orderjudge.Workspace
		started := time.Now()
		result.Selection.ModelCalls++
		err := p.runtime.model.Predict(&p.intent, &p.candidates, &workspace, &receipt.Prediction)
		receipt.PredictNS = time.Since(started).Nanoseconds()
		if err != nil {
			return result, nil, receipt, err
		}
		receipt.Mode = "whole_candidate_rank_before_finite_validation"
	}
	result.InitialProposals = p.selected(receipt.Prediction.Ranking[0])
	for bit, c := range p.choices {
		result.Selection.Receipts = append(result.Selection.Receipts, pathplan.Receipt{ID: c.id, Kind: c.kind,
			IntentSHA256: c.intentSHA, Mode: receipt.Mode, Proposed: result.InitialProposals[c.id], Selected: c.labels[(p.fallback>>bit)&1]})
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
				if wasEvaluated && p.descriptors[previous] == p.descriptors[mask] {
					alias = previous
					break
				}
			}
		}
		if alias >= 0 {
			receipt.Aliases = append(receipt.Aliases, orderjudge.Alias{Mask: mask, EvaluatedAs: uint8(alias)})
			continue
		}
		choices := p.selected(mask)
		attempt := pathplan.SearchAttempt{Mask: uint16(mask), Choices: choices, Status: "EVALUATED",
			Total: len(cases), GoooSHA: p.programSHA[mask], Results: make([]pathplan.TestResult, 0, len(cases))}
		for _, c := range cases {
			if err := ctx.Err(); err != nil {
				return result, best, receipt, err
			}
			value, err := p.programs[mask].Evaluate(c.Input)
			if err != nil {
				return result, best, receipt, err
			}
			passed := value.Int == c.Expected
			if passed {
				attempt.Passed++
			}
			attempt.Results = append(attempt.Results, pathplan.TestResult{Input: c.Input, Expected: c.Expected, Actual: value.Int, Passed: passed})
		}
		evaluated[mask] = true
		result.Evaluated++
		result.Attempts = append(result.Attempts, attempt)
		result.Unattempted = 8 - result.Evaluated
		if attempt.Passed > bestPassed {
			owned := *p.programs[mask]
			bestPassed, best = attempt.Passed, &owned
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
