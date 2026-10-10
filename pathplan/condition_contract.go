package pathplan

import (
	"context"
	"fmt"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

// ErrNoConditionCandidate also matches ErrNoTypedCandidate so bounded sessions
// may continue without restarting ranking or discarding committed attempts.
var ErrNoConditionCandidate = fmt.Errorf("source condition cases have no eligible candidate: %w", ErrNoTypedCandidate)

func validateConditionCases(plan Plan) error {
	if len(plan.ConditionCases) > 128 {
		return fmt.Errorf("source condition cases exceed 128")
	}
	prepared := &PreparedPlan{plan: plan}
	type key struct {
		statement int
		input     int64
	}
	seen := make(map[key]bool, len(plan.ConditionCases))
	for _, test := range plan.ConditionCases {
		target, err := prepared.conditionTarget(test.ChoiceID)
		if err != nil {
			return err
		}
		k := key{target, test.Input}
		if seen[k] {
			return fmt.Errorf("duplicate condition input for the same if statement")
		}
		seen[k] = true
	}
	return nil
}

func (prepared *PreparedPlan) HasConditions() bool {
	return prepared != nil && len(prepared.plan.ConditionCases) > 0
}

// ConditionsPassed means all returned finite observations matched. An empty
// result represents absence of declared conditions, not evidence of correctness.
func ConditionsPassed(results []ConditionResult) bool {
	for _, result := range results {
		if !result.Passed {
			return false
		}
	}
	return true
}

// CheckDeclaredConditions checks the immutable plan contract for a complete
// selection. Compile alone remains a type/scope check, allowing rejected bodies
// to be inspected and counted rather than silently removed from the palette.
func (prepared *PreparedPlan) CheckDeclaredConditions(ctx context.Context, choices map[string]string) ([]ConditionResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("condition context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if prepared == nil {
		return nil, fmt.Errorf("prepared plan is required")
	}
	if !prepared.HasConditions() {
		return nil, nil
	}
	return prepared.CheckConditions(ctx, choices, prepared.plan.ConditionCases)
}

func (prepared *PreparedPlan) checkProgramConditions(ctx context.Context, program *bodyplan.Program) ([]ConditionResult, error) {
	return prepared.observeConditions(ctx, program, prepared.plan.ConditionCases)
}

// Both one-shot and incremental search use this evaluator. Condition failures
// retain final-output observations but cannot become the selected program.
func (prepared *PreparedPlan) evaluateCandidate(ctx context.Context, mask uint16, choices map[string]string,
	cases []TestCase) (SearchAttempt, *bodyplan.Program, error) {
	attempt := SearchAttempt{Mask: mask, Choices: choices, Total: len(cases), Status: "TYPE_REJECTED"}
	program, err := assemble(prepared.plan, choices)
	if err != nil {
		return attempt, nil, nil
	}
	attempt.Status, attempt.GoooSHA = "EVALUATED", hash([]byte(program.GoooSource()))
	attempt.Conditions, err = prepared.checkProgramConditions(ctx, program)
	if err != nil {
		return attempt, nil, err
	}
	if !ConditionsPassed(attempt.Conditions) {
		attempt.Status = "CONDITION_REJECTED"
	}
	for _, test := range cases {
		if err := ctx.Err(); err != nil {
			return attempt, nil, err
		}
		value, err := program.Evaluate(test.Input)
		if err != nil {
			return attempt, nil, fmt.Errorf("finite candidate interpreter failed: %w", err)
		}
		passed := value.Int == test.Expected
		if passed {
			attempt.Passed++
		}
		attempt.Results = append(attempt.Results, TestResult{test.Input, test.Expected, value.Int, passed})
	}
	if err := ctx.Err(); err != nil {
		return attempt, nil, err
	}
	return attempt, program, nil
}
