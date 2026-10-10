package pathplan

import (
	"context"
	"errors"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
)

// SearchConditionBatches retains at most 64 attempts and 16 feedback records.
// The feature ABI includes source, authored intent and observed conditions;
// this API does not accept CI hints or encode output failures as model features.
// All output cases still participate in candidate acceptance.
func (p *PreparedPlan) SearchConditionBatches(ctx context.Context, model *conditiondecision.Model, cases []TestCase, total, step int, seed string, rounds int) (SearchResult, *bodyplan.Program, []ConditionProgress, []ConditionRanking, error) {
	if err := searchBounds(ctx, cases, total); err != nil {
		return SearchResult{}, nil, nil, nil, err
	}
	if step < 1 || step > total || rounds < 0 || rounds > 16 || (model == nil && rounds != 0) {
		return SearchResult{}, nil, nil, nil, errors.New("condition batches require step within total, 0..16 rounds and model for feedback")
	}
	s, startErr := p.NewConditionSession(ctx, model, cases, seed)
	if s == nil {
		return SearchResult{}, nil, nil, nil, startErr
	}
	initial, err := s.Observe()
	if err != nil {
		return SearchResult{}, nil, nil, nil, err
	}
	records := []ConditionProgress{initial}
	result := sessionSearchResult(initial.Search, nil)
	if startErr != nil {
		return result, nil, records, nil, startErr
	}
	var attempts []SearchAttempt
	var rankings []ConditionRanking
	var body *bodyplan.Program
	for len(attempts) < total {
		progress, selected, failure := s.Advance(ctx, min(step, total-len(attempts)))
		if progress.Schema != "" {
			attempts = append(attempts, progress.Search.NewAttempts...)
			records = append(records, progress)
			result, body = sessionSearchResult(progress.Search, attempts), selected
		}
		if failure != nil && !errors.Is(failure, ErrNoTypedCandidate) && !errors.Is(failure, ErrNoConditionCandidate) {
			return result, body, records, rankings, failure
		}
		if progress.Search.Status == "TRAINING_COMPLETE" || progress.Search.Exhausted || len(attempts) >= total {
			return result, body, records, rankings, failure
		}
		if len(progress.Search.NewAttempts) == 0 {
			return result, body, records, rankings, errors.New("condition batch made no candidate progress")
		}
		if !initial.Ranking.Declined && len(rankings) < rounds {
			r, rankingErr := s.Reconsider(ctx, model)
			if r.Schema != "" {
				rankings = append(rankings, r)
			}
			observed, observeErr := s.Observe()
			if observeErr != nil {
				return result, body, records, rankings, observeErr
			}
			records = append(records, observed)
			result = sessionSearchResult(observed.Search, attempts)
			if rankingErr != nil {
				return result, body, records, rankings, rankingErr
			}
		}
	}
	return result, body, records, rankings, nil
}
