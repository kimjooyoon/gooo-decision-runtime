package pathplan

import (
	"context"
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// ConditionInput binds model features to an immutable prepared Gooo plan and,
// optionally, the first failing declared condition of one compiled candidate.
// It is reusable across that plan's choices. It retains no mutable caller data.
// This is model input preparation, not training or candidate acceptance.
type ConditionInput struct {
	prepared *PreparedPlan
	failure  ConditionFailure
	present  bool
}

// InitialConditionInput uses source-authored intents with no candidate outcome.
// It performs no evaluation and makes no model predictions.
func (prepared *PreparedPlan) InitialConditionInput() (*ConditionInput, error) {
	if prepared == nil || prepared.fallback == nil {
		return nil, errors.New("prepared condition input plan required")
	}
	return &ConditionInput{prepared: prepared}, nil
}

// ObserveConditionInput compiles a complete choice map and evaluates the plan's
// declared conditions once. The first mismatch or unreached condition becomes
// typed feedback. A successful observation has no failure channel; it does not
// establish output correctness. Synchronize caller choices during this call.
// FeaturesInto subsequently reuses this observation without evaluating again.
func (prepared *PreparedPlan) ObserveConditionInput(ctx context.Context, choices map[string]string) (*ConditionInput, error) {
	input, err := prepared.InitialConditionInput()
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, errors.New("condition input context required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	mask, err := diagnosisMask(prepared.plan, choices)
	if err != nil {
		return nil, err
	}
	// Compile even with no declared conditions, so invalid combined selections
	// cannot acquire a source-bound input through the empty-case fast path.
	program, err := prepared.Compile(choices)
	if err != nil {
		return nil, err
	}
	results, err := prepared.checkProgramConditions(ctx, program)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		if !result.Passed {
			input.failure = ConditionFailure{Mask: mask, Result: result}
			input.present = true
			break
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return input, nil
}

// PlanSHA256 identifies the source snapshot, including intents and conditions.
func (input *ConditionInput) PlanSHA256() string {
	if input == nil || input.prepared == nil {
		return ""
	}
	return input.prepared.sha
}

// Failure returns an owned value, including exact int64 input and output.
func (input *ConditionInput) Failure() (ConditionFailure, bool) {
	if input == nil {
		return ConditionFailure{}, false
	}
	return input.failure, input.present
}

// FeaturesInto obtains source fields, full authored intent and choice indexes
// from the same immutable plan. Callers supply no rewritten intent or condition.
// Concurrent calls use separate output arrays and share no mutable scratch.
func (input *ConditionInput) FeaturesInto(id string, output *[decision.FeatureDim]float32) error {
	return input.FeaturesIntoVersion(id, decision.ConditionChannelFeatureVersion, output)
}

// FeaturesIntoVersion uses an explicit model ABI and preserves the destination
// for unsupported versions. FeaturesInto remains byte-for-byte v1.
func (input *ConditionInput) FeaturesIntoVersion(id, version string, output *[decision.FeatureDim]float32) error {
	if version != decision.ConditionChannelFeatureVersion && version != decision.ConditionBranchFeatureVersion {
		return errors.New("unsupported source condition feature version")
	}
	if input == nil || input.prepared == nil {
		return errors.New("source-bound condition input required")
	}
	prepared := input.prepared
	source, err := prepared.SourceFeatures(id)
	if err != nil {
		return err
	}
	var feedback decision.ConditionFeedback
	if input.present {
		row := input.failure.Result
		feedback = decision.ConditionFeedback{
			Present: true, ChoiceCount: uint8(len(prepared.plan.Decisions)),
			CandidateMask: input.failure.Mask, Input: row.Case.Input,
			Expected: row.Case.Expected, Reached: row.Observation.Reached,
			Actual: row.Observation.Reached && row.Observation.Value,
		}
		for i, choice := range prepared.plan.Decisions {
			if choice.ID == row.Case.ChoiceID {
				feedback.ObservedChoice = uint8(i)
			}
		}
	}
	for i, choice := range prepared.plan.Decisions {
		if choice.ID == id {
			if feedback.Present {
				feedback.Choice = uint8(i)
			}
			if version == decision.ConditionBranchFeatureVersion {
				roles, err := prepared.BranchReturnRoles(id)
				if err != nil {
					return err
				}
				return decision.ConditionBranchFeaturesInto(source, roles, choice.Intent, feedback, output)
			}
			return decision.ConditionFeaturesInto(source, choice.Intent, feedback, output)
		}
	}
	return errors.New("condition input choice is undeclared")
}
