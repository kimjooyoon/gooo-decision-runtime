package pathplan

import (
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// ConditionCount is the number of source-authored Boolean requirements. Zero
// means that no conditions were supplied, not that conditions passed.
func (input *ContractInput) ConditionCount() int {
	if input == nil || input.initial.prepared == nil {
		return 0
	}
	return len(input.initial.prepared.plan.ConditionCases)
}

func (input *ContractInput) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}

// ConditionCase returns an owned scalar value from the immutable source plan.
// No candidate is assembled, evaluated or predicted by this reader.
func (input *ContractInput) ConditionCase(index int) (ConditionCase, error) {
	if index < 0 || index >= input.ConditionCount() {
		return ConditionCase{}, errors.New("declared condition index outside bound contract")
	}
	return input.initial.prepared.plan.ConditionCases[index], nil
}

// ConditionFeaturesInto streams one declared condition through caller-owned
// scratch, retaining all 0..128 rows and source order. The target index comes
// from the same prepared plan as RelationalSourceFeaturesInto. Existing model
// case readers remain unchanged; a model must opt into this separate ABI.
func (input *ContractInput) ConditionFeaturesInto(index int, output *[decision.DeclaredConditionFeatureDim]float32) error {
	condition, err := input.ConditionCase(index)
	if err != nil {
		return err
	}
	choices := input.initial.prepared.plan.Decisions
	for target, choice := range choices {
		if choice.ID == condition.ChoiceID {
			return decision.DeclaredConditionFeaturesInto(decision.DeclaredConditionFeatureInput{
				Input: condition.Input, Expected: condition.Expected, TargetChoice: target,
				ChoiceCount: len(choices), Index: index, Count: input.ConditionCount(),
			}, output)
		}
	}
	return errors.New("declared condition target is absent from its prepared plan")
}

func (input *ContractInput) ConditionFeatures(index int) ([decision.DeclaredConditionFeatureDim]float32, error) {
	var row [decision.DeclaredConditionFeatureDim]float32
	err := input.ConditionFeaturesInto(index, &row)
	return row, err
}
