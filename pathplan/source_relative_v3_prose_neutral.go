package pathplan

import (
	"context"
	"errors"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

// SourceRelativeV3ProseNeutralFeaturesInto preserves the owned source and all
// requirements, then clears only the explicit intent channel. Failure leaves
// the caller's destination unchanged; preparation executes no candidate/model.
func (input *ContractInput) SourceRelativeV3ProseNeutralFeaturesInto(id string, out *[528]float32) error {
	if out == nil {
		return errors.New("prose-neutral feature destination required")
	}
	var next [528]float32
	if err := input.SourceRelativeV3FeaturesInto(id, &next); err != nil {
		return err
	}
	clear(next[contractdecision.SourceRelativeV3ProseNeutralIntentStart:contractdecision.SourceRelativeV3ProseNeutralIntentEnd])
	*out = next
	return nil
}
func (p *PreparedPlan) NewSourceRelativeV3ProseNeutralContractSession(ctx context.Context, model *contractdecision.SourceRelativeV3ProseNeutralModel, cases []TestCase) (*ContractSession, error) {
	if model == nil {
		return p.newContractSession(ctx, cases, "", "", "", nil)
	}
	return p.newInteractionRequirementSession(ctx, model, cases, relativeV3ProseNeutralInteractionSource)
}
