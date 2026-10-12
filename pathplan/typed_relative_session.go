package pathplan

import (
	"context"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

// NewTypedSourceRelativeContractSession routes source kinds within one initial
// prediction. Finite checks still execute the source-owned candidates. Omitting
// the model retains deterministic behavior, and unsupported input views decline.
func (p *PreparedPlan) NewTypedSourceRelativeContractSession(ctx context.Context, model *contractdecision.TypedSourceRelativeModel, cases []TestCase) (*ContractSession, error) {
	if model == nil {
		return p.newContractSession(ctx, cases, "", "", "", nil)
	}
	return p.newRequirementSession(ctx, model, cases, typedRelativeInteractionSource,
		func(source [][contractdecision.OrderedFeatureDim]float32, outputs contractdecision.CaseSource,
			conditions contractdecision.ConditionSource, prediction *contractdecision.ChoicePrediction) error {
			var workspace contractdecision.TypedSourceRelativeWorkspace
			return model.PredictChoicesInto(source, outputs, conditions, &workspace, prediction)
		})
}
