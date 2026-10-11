package pathplan

import (
	"context"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

// V2 predicts once from its explicitly versioned source representation. The
// ordinary finite frontier validates outputs and conditions; nil is deterministic.
func (p *PreparedPlan) NewSourceRelativeV2ContractSession(ctx context.Context,
	model *contractdecision.SourceRelativeV2Model, cases []TestCase) (*ContractSession, error) {
	if model == nil {
		return p.newContractSession(ctx, cases, "", "", "", nil)
	}
	return p.newInteractionRequirementSession(ctx, model, cases, relativeV2InteractionSource)
}
