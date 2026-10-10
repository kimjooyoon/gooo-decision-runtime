package pathplan

import (
	"errors"
	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// ExecutionRelationalFlowFeaturesInto exposes the explicit v6 input. Source
// forms outside the existing semantic normalization retain the complete v5
// representation. Committed observation channels and exact atoms are preserved.
func (input *ConditionInput) ExecutionRelationalFlowFeaturesInto(id string, out *[decision.ExecutionFlowFeatureDim]float32) error {
	if out == nil {
		return errors.New("relational flow destination required")
	}
	var original [decision.ExecutionFlowFeatureDim]float32
	if err := input.ExecutionSemanticFlowFeaturesInto(id, &original); err != nil {
		return err
	}
	view, err := input.prepared.SemanticBranchContext(id)
	if err != nil {
		return err
	}
	if !view.Normalized {
		*out = original
		return nil
	}
	return decision.RelationalFlowFeaturesInto(original, view.Flow, out)
}
