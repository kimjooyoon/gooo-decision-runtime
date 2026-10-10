package pathplan

import (
	"context"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
)

// NewFlowSession ranks source-defined paths using static value flow alongside
// the v3 source/intent/observed-failure channels. A nil model uses deterministic
// continuation. No model pointer is retained by the returned session.
func (p *PreparedPlan) NewFlowSession(ctx context.Context, model *flowdecision.Model, cases []TestCase, seed string) (*ConditionSession, error) {
	return p.newConditionSession(ctx, executionRanker{flow: model}, cases, seed)
}

func (s *ConditionSession) ReconsiderFlow(ctx context.Context, model *flowdecision.Model) (ConditionRanking, error) {
	return s.reconsider(ctx, executionRanker{flow: model})
}

func (p *PreparedPlan) SearchFlowBatches(ctx context.Context, model *flowdecision.Model, cases []TestCase, total, step int, seed string, rounds int) (SearchResult, *bodyplan.Program, []ConditionProgress, []ConditionRanking, error) {
	return p.searchConditionBatches(ctx, executionRanker{flow: model}, cases, total, step, seed, rounds)
}

func (s *ConditionSession) hasOutputChannel() bool {
	return s.featureVersion == decision.ExecutionFeatureVersion || s.featureVersion == decision.ExecutionFlowFeatureVersion || s.featureVersion == decision.SemanticFlowFeatureVersion
}
