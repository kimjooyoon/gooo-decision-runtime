package pathplan

import (
	"errors"
	"fmt"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

// OrderedSourceFeaturesInto appends ordered predicate/return expressions to
// the unchanged384-cell source input. Unsupported source shapes return a
// diagnostic and preserve the destination; they never receive fabricated zeros.
func (input *ContractInput) OrderedSourceFeaturesInto(id string, output *[contractdecision.OrderedFeatureDim]float32) error {
	if input == nil || input.initial.prepared == nil || output == nil {
		return errors.New("source-bound ordered input and destination required")
	}
	var prefix [contractdecision.FeatureDim]float32
	if err := input.RelationalSourceFeaturesInto(id, &prefix); err != nil {
		return err
	}
	view, err := input.initial.prepared.OrderedBranchContext(id)
	if err != nil {
		return err
	}
	if !view.Available {
		return fmt.Errorf("ORDERED_SOURCE_CONTEXT_UNAVAILABLE: %s", view.Reason)
	}
	var ordered [decision.OrderedExpressionFeatureDim]float32
	if err := decision.OrderedExpressionFeaturesInto(view.Expressions, &ordered); err != nil {
		return err
	}
	var next [contractdecision.OrderedFeatureDim]float32
	copy(next[:], prefix[:])
	copy(next[contractdecision.FeatureDim:], ordered[:])
	*output = next
	return nil
}
