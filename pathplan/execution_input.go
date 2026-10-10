package pathplan

import (
	"context"
	"encoding/json"
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// OutputFailure is the first wrong output of one committed candidate. It makes
// no causal claim about any individual choice in that candidate.
type OutputFailure struct {
	Mask   uint16     `json:"choice_mask"`
	Result TestResult `json:"result"`
}

// InitialExecutionInput binds a separate finite case identity without exposing
// unseen expected outputs to model features. The owning compiler supplies these
// cases from source. No evaluation or inference occurs here.
func (p *PreparedPlan) InitialExecutionInput(cases []TestCase) (*ConditionInput, error) {
	input, err := p.InitialConditionInput()
	if err != nil {
		return nil, err
	}
	if len(cases) == 0 || len(cases) > 128 || p.plan.Base.ResultType != decision.TypeInt {
		return nil, errors.New("execution input requires integer output and 1..128 cases")
	}
	raw, err := json.Marshal(cases)
	if err != nil {
		return nil, err
	}
	input.caseSHA = hash(raw)
	return input, nil
}

// ObserveExecutionInput compiles one selection and evaluates declared conditions
// and caller-supplied output cases once, with a deadline. Failed type checking
// or interrupted evaluation returns no input. It never searches or invokes a model.
func (p *PreparedPlan) ObserveExecutionInput(ctx context.Context, choices map[string]string, cases []TestCase) (*ConditionInput, error) {
	if err := searchBounds(ctx, cases, 1); err != nil {
		return nil, err
	}
	ownedCases := append([]TestCase(nil), cases...)
	input, err := p.InitialExecutionInput(ownedCases)
	if err != nil {
		return nil, err
	}
	mask, err := diagnosisMask(p.plan, choices)
	if err != nil {
		return nil, err
	}
	attempt, body, err := p.evaluateCandidate(ctx, mask, choices, ownedCases)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, errors.New("execution input requires a compiled complete candidate")
	}
	input.observeExecution(attempt)
	return input, nil
}

// observeExecution consumes only committed evaluator records, without replay.
func (input *ConditionInput) observeExecution(attempt SearchAttempt) {
	input.failure, input.present = ConditionFailure{}, false
	input.outputFailure, input.hasOutputFailure = OutputFailure{}, false
	for _, row := range attempt.Conditions {
		if !row.Passed {
			input.failure, input.present = ConditionFailure{Mask: attempt.Mask, Result: row}, true
			break
		}
	}
	for _, row := range attempt.Results {
		if !row.Passed {
			input.outputFailure, input.hasOutputFailure = OutputFailure{Mask: attempt.Mask, Result: row}, true
			break
		}
	}
}

func (input *ConditionInput) CaseSHA256() string {
	if input == nil {
		return ""
	}
	return input.caseSHA
}

func (input *ConditionInput) OutputFailure() (OutputFailure, bool) {
	if input == nil {
		return OutputFailure{}, false
	}
	return input.outputFailure, input.hasOutputFailure
}

// ExecutionFeaturesInto preserves all v2 channels, including simultaneous
// condition failure. Only an actually observed wrong output enters the new tail.
// A legacy condition-only input cannot silently masquerade as execution input.
func (input *ConditionInput) ExecutionFeaturesInto(id string, output *[decision.ExecutionFeatureDim]float32) error {
	if input == nil || input.caseSHA == "" || output == nil {
		return errors.New("source/case-bound execution input and destination required")
	}
	var prefix [decision.FeatureDim]float32
	if err := input.FeaturesIntoVersion(id, decision.ConditionBranchFeatureVersion, &prefix); err != nil {
		return err
	}
	var feedback decision.OutputFeedback
	if input.hasOutputFailure {
		row := input.outputFailure.Result
		feedback = decision.OutputFeedback{Present: true, ChoiceCount: uint8(len(input.prepared.plan.Decisions)), CandidateMask: input.outputFailure.Mask, Input: row.Input, Expected: row.Expected, Actual: row.Actual}
		for i, choice := range input.prepared.plan.Decisions {
			if choice.ID == id {
				feedback.Choice = uint8(i)
			}
		}
	}
	// Use the public validated encoder, then retain the already encoded prefix.
	source, err := input.prepared.SourceFeatures(id)
	if err != nil {
		return err
	}
	roles, err := input.prepared.BranchReturnRoles(id)
	if err != nil {
		return err
	}
	var candidate [decision.ExecutionFeatureDim]float32
	for _, choice := range input.prepared.plan.Decisions {
		if choice.ID == id {
			if err := decision.ExecutionFeaturesInto(source, roles, choice.Intent, decision.ConditionFeedback{}, feedback, &candidate); err != nil {
				return err
			}
			copy(candidate[:], prefix[:])
			*output = candidate
			return nil
		}
	}
	return errors.New("execution input choice is undeclared")
}
