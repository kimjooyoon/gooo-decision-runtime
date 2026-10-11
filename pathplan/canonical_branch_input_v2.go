package pathplan

import (
	"errors"
	"fmt"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// V2 preserves the V1 prefix and uses the explicitly versioned ordered adjunct.
// Preparation reads no model and evaluates no candidate or probe input.
func (input *ContractInput) SourceRelativeV2FeaturesInto(id string, out *[528]float32) error {
	if input == nil || input.initial.prepared == nil || out == nil {
		return errors.New("source-relative V2 input and destination required")
	}
	view, err := input.initial.prepared.CanonicalOrderedBranchContextV2(id)
	if err != nil {
		return err
	}
	if !view.Available {
		return fmt.Errorf("CANONICAL_SOURCE_CONTEXT_UNAVAILABLE: %s", view.Reason)
	}
	relations, err := input.SourceRelativeV2Conditions(id)
	if err != nil {
		return err
	}
	if !relations.Available {
		return fmt.Errorf("SOURCE_RELATIVE_CONTEXT_UNAVAILABLE: %s", relations.Reason)
	}
	for _, choice := range input.initial.prepared.plan.Decisions {
		if choice.ID != id {
			continue
		}
		var prefix [256]float32
		if err := decision.ConditionFeaturesInto(view.Source, choice.Intent, decision.ConditionFeedback{}, &prefix); err != nil {
			return err
		}
		var suffix [144]float32
		if err := decision.OrderedExpressionV2FeaturesInto(view.Expressions, &suffix); err != nil {
			return err
		}
		var next [528]float32
		copy(next[:192], prefix[:192])
		copy(next[192:208], relations.Features[:])
		copy(next[384:], suffix[:])
		*out = next
		return nil
	}
	return errors.New("source-relative V2 choice is undeclared")
}
