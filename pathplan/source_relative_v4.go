package pathplan

import (
	"context"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

// NewSourceRelativeV4ContractSession predicts once from the source-owned branch
// relations. Nil and unsupported representations retain deterministic search.
func (p *PreparedPlan) NewSourceRelativeV4ContractSession(ctx context.Context, model *contractdecision.SourceRelativeV4Model, cases []TestCase) (*ContractSession, error) {
	if model == nil {
		return p.newContractSession(ctx, cases, "", "", "", nil)
	}
	return p.newInteractionRequirementSession(ctx, model, cases, relativeV4InteractionSource)
}
