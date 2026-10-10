package pathplan

import (
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// ExecutionFlowFeaturesInto is an explicit 384-cell projection for subsequent
// model development. Existing 256/320-cell model decoders and ranking APIs stay
// unchanged. A v3 model cannot consume this array or silently acquire new facts.
func (input *ConditionInput) ExecutionFlowFeaturesInto(id string, output *[decision.ExecutionFlowFeatureDim]float32) error {
	if output == nil {
		return errors.New("flow feature destination required")
	}
	var prefix [decision.ExecutionFeatureDim]float32
	if err := input.ExecutionFeaturesInto(id, &prefix); err != nil {
		return err
	}
	flow, err := input.prepared.BranchValueFlow(id)
	if err != nil {
		return err
	}
	return decision.ExecutionFlowFeaturesInto(prefix, flow, output)
}
