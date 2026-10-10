package pathplan

import (
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// ExecutionSemanticFlowFeaturesInto is the explicit v5 input. In an eligible
// single-branch body it replaces syntax counts/local spelling/return roles by
// canonical predicate/option facts and exact value-flow atoms. Source-owned
// intent and committed condition/output observations retain their v4 bytes.
// Other shapes preserve all384 v4 cells. Errors preserve the destination.
func (input *ConditionInput) ExecutionSemanticFlowFeaturesInto(id string, output *[decision.ExecutionFlowFeatureDim]float32) error {
	if output == nil {
		return errors.New("semantic flow destination required")
	}
	var original [decision.ExecutionFlowFeatureDim]float32
	if err := input.ExecutionFlowFeaturesInto(id, &original); err != nil {
		return err
	}
	semantic, err := input.prepared.SemanticBranchContext(id)
	if err != nil {
		return err
	}
	if !semantic.Normalized {
		*output = original
		return nil
	}
	for _, choice := range input.prepared.plan.Decisions {
		if choice.ID != id {
			continue
		}
		var source [decision.FeatureDim]float32
		if err := decision.ConditionFeaturesInto(semantic.Source, choice.Intent, decision.ConditionFeedback{}, &source); err != nil {
			return err
		}
		var prefix [decision.ExecutionFeatureDim]float32
		copy(prefix[:], original[:])
		copy(prefix[:64], source[:64])
		clear(prefix[236:256])
		prefix[255] = 2
		return decision.ExecutionFlowFeaturesInto(prefix, semantic.Flow, output)
	}
	return errors.New("semantic flow choice is undeclared")
}
