package pathplan

import (
	"context"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func (p *PreparedPlan) NewSourceRelativeV3ContractSession(ctx context.Context,
	model *contractdecision.SourceRelativeV3Model, cases []TestCase) (*ContractSession, error) {
	if model == nil {
		return p.newContractSession(ctx, cases, "", "", "", nil)
	}
	return p.newInteractionRequirementSession(ctx, model, cases, relativeV3InteractionSource)
}
