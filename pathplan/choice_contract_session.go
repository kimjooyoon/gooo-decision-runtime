package pathplan

import (
	"context"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

// NewChoiceContractSession keeps every original v6 source cell and declared
// case. The local model computes a separate case summary for each choice, then
// immediately seeds the ordinary finite frontier. Nil is deterministic.
func (p *PreparedPlan) NewChoiceContractSession(ctx context.Context, model *contractdecision.ChoiceModel, cases []TestCase) (*ContractSession, error) {
	if model == nil {
		return p.newContractSession(ctx, cases, "", "", "", nil)
	}
	predict := func(source [][contractdecision.FeatureDim]float32, input contractdecision.CaseSource) (contractdecision.ChoicePrediction, error) {
		var workspace contractdecision.ChoiceWorkspace
		var result contractdecision.ChoicePrediction
		err := model.PredictChoicesInto(source, input, &workspace, &result)
		return result, err
	}
	return p.newContractSession(ctx, cases, model.Fingerprint(), model.CaseFeatureVersion(), "choice_conditioned_contract_fp32", predict)
}
