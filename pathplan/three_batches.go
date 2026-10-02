package pathplan

import (
	"context"
	"errors"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
)

func (prepared *PreparedPlan) SearchThreeFeedbackBatches(ctx context.Context, model *jointdecision.ThreeModel, cases []TestCase, total, step int, seed string, rounds int, ci *CIHint) (SearchResult, *bodyplan.Program, []SessionProgress, []FeedbackReceipt, error) {
	if err := searchBounds(ctx, cases, total); err != nil {
		return SearchResult{}, nil, nil, nil, err
	}
	if rounds < 0 || rounds > 16 || step < 1 || step > 64 {
		return SearchResult{}, nil, nil, nil, errors.New("three-choice batch budgets outside bounds")
	}
	if err := ci.Validate(); err != nil {
		return SearchResult{}, nil, nil, nil, err
	}
	session, startErr := prepared.NewThreeSession(ctx, model, cases, seed)
	if session == nil {
		return SearchResult{}, nil, nil, nil, startErr
	}
	initial, err := session.Observe()
	if err != nil {
		return SearchResult{}, nil, nil, nil, err
	}
	records := []SessionProgress{initial}
	result := sessionSearchResult(initial, nil)
	if startErr != nil {
		return result, nil, records, nil, startErr
	}
	var attempts []SearchAttempt
	var feedback []FeedbackReceipt
	var body *bodyplan.Program
	for len(attempts) < total {
		progress, selected, failure := session.Advance(ctx, min(step, total-len(attempts)))
		if progress.Schema != "" {
			attempts = append(attempts, progress.NewAttempts...)
			records = append(records, progress)
			result, body = sessionSearchResult(progress, attempts), selected
		}
		if failure != nil && !errors.Is(failure, ErrNoTypedCandidate) {
			return result, body, records, feedback, failure
		}
		if progress.Status == "TRAINING_COMPLETE" || progress.Exhausted || len(attempts) >= total {
			return result, body, records, feedback, failure
		}
		if len(progress.NewAttempts) == 0 {
			return result, body, records, feedback, errors.New("three-choice batch made no candidate progress")
		}
		if session.joint && len(feedback) < rounds {
			receipt, failure := session.ReconsiderThree(ctx, model, ci)
			if receipt.Schema != "" {
				feedback = append(feedback, receipt)
			}
			if errors.Is(failure, ErrFeedbackContextBound) && receipt.ContextDeclined && receipt.ModelCalls == 0 && receipt.SHA != "" {
				failure = nil
			}
			observed, observeErr := session.observe(failure != nil)
			if observeErr != nil {
				return result, body, records, feedback, observeErr
			}
			records = append(records, observed)
			result = sessionSearchResult(observed, attempts)
			if failure != nil {
				return result, body, records, feedback, failure
			}
		}
	}
	return result, body, records, feedback, nil
}
