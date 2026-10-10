package pathplan

import (
	"context"
	"errors"
	"fmt"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/internal/strictjson"
)

// ConditionCase names an existing structural choice, never generated code. The
// owning compiler must bind these finite expectations to its source contract.
// BranchLayout watches that if statement. OperandOrder is supported only when
// its target expression is the full condition of exactly one if statement.
type ConditionCase struct {
	ChoiceID string `json:"choice_id"`
	Input    int64  `json:"input"`
	Expected bool   `json:"expected"`
}

func (test *ConditionCase) UnmarshalJSON(raw []byte) error {
	var fields struct {
		ChoiceID *string `json:"choice_id"`
		Input    *int64  `json:"input"`
		Expected *bool   `json:"expected"`
	}
	if err := strictjson.Decode(raw, &fields); err != nil {
		return err
	}
	if fields.ChoiceID == nil || fields.Input == nil || fields.Expected == nil {
		return errors.New("condition case requires explicit non-null choice_id, input and expected")
	}
	*test = ConditionCase{ChoiceID: *fields.ChoiceID, Input: *fields.Input, Expected: *fields.Expected}
	return nil
}

type ConditionResult struct {
	Case        ConditionCase                 `json:"case"`
	Observation bodyplan.ConditionObservation `json:"observation"`
	Output      bodyplan.Value                `json:"output"`
	Status      string                        `json:"status"`
	Passed      bool                          `json:"passed"`
}

// CheckConditions compiles one complete declared selection and observes its
// actual if conditions. It does not select another candidate or modify a model/
// search receipt. A condition passes only if reached with the declared Boolean
// value. Skipped conditions stay NOT_REACHED rather than passing as false. The
// result has at most 128 rows.
// Search consumes Plan.ConditionCases. This method also supports separate
// caller-supplied diagnostic cases without changing the prepared contract.
func (prepared *PreparedPlan) CheckConditions(ctx context.Context, choices map[string]string,
	cases []ConditionCase) ([]ConditionResult, error) {
	if ctx == nil || len(cases) == 0 || len(cases) > 128 {
		return nil, errors.New("condition observations need a context and 1..128 cases")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	program, err := prepared.Compile(choices)
	if err != nil {
		return nil, err
	}
	return prepared.observeConditions(ctx, program, cases)
}

func (prepared *PreparedPlan) observeConditions(ctx context.Context, program *bodyplan.Program,
	cases []ConditionCase) ([]ConditionResult, error) {
	if len(cases) == 0 {
		return nil, nil
	}
	var targets [128]int
	for i, test := range cases {
		target, err := prepared.conditionTarget(test.ChoiceID)
		if err != nil {
			return nil, err
		}
		targets[i] = target
	}
	results := make([]ConditionResult, len(cases))
	for i, test := range cases {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		value, observation, err := program.ObserveCondition(test.Input, targets[i])
		if err != nil {
			return nil, err
		}
		status := "MISMATCH"
		if !observation.Reached {
			status = "NOT_REACHED"
		} else if observation.Value == test.Expected {
			status = "MATCH"
		}
		results[i] = ConditionResult{Case: test, Observation: observation, Output: value, Status: status, Passed: status == "MATCH"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (prepared *PreparedPlan) conditionTarget(id string) (int, error) {
	for _, choice := range prepared.plan.Decisions {
		if choice.ID != id {
			continue
		}
		if choice.Kind == BranchLayout {
			return choice.Target, nil
		}
		if choice.Kind == OperandOrder {
			target := -1
			for i, statement := range prepared.plan.Base.Statements {
				if statement.Kind != bodyplan.StmtIf || statement.Expr != choice.Target {
					continue
				}
				if target != -1 {
					return 0, fmt.Errorf("choice %q names multiple if conditions; name a branch_layout choice", id)
				}
				target = i
			}
			if target != -1 {
				return target, nil
			}
		}
		return 0, fmt.Errorf("choice %q has no supported if condition observation", id)
	}
	return 0, fmt.Errorf("condition case names unknown choice %q", id)
}
