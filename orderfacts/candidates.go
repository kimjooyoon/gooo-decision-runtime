package orderfacts

import (
	"fmt"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

// Candidates describes every composed mask in the three-choice profile. Each
// mask bit selects the corresponding declared option, including operand swaps.
// Type checking precedes observation; no input cases or expected values are read.
// The returned array is owned by the caller and failure returns no partial facts.
func Candidates(plan pathplan.Plan) ([8]Signature, error) {
	var result [8]Signature
	if len(plan.Decisions) != 3 {
		return result, fmt.Errorf("three structural choices required")
	}
	root, operands := 0, 0
	for _, choice := range plan.Decisions {
		switch choice.Kind {
		case pathplan.RootOrder:
			root++
		case pathplan.OperandOrder:
			operands++
		default:
			return result, fmt.Errorf("only one root and two operand choices supported")
		}
	}
	if root != 1 || operands != 2 {
		return result, fmt.Errorf("one root and two operand choices required")
	}
	prepared, err := pathplan.Prepare(plan)
	if err != nil {
		return result, err
	}
	selected := prepared.Defaults()
	for mask := range 8 {
		var expressions [128]bodyplan.Expr
		base := plan.Base
		base.Expressions = expressions[:copy(expressions[:], plan.Base.Expressions)]
		order := base.Root
		for i, choice := range plan.Decisions {
			option := choice.Options[(mask>>i)&1]
			selected[choice.ID] = option.Label
			if choice.Kind == pathplan.RootOrder {
				order = option.Order
			} else if option.Reverse {
				e := &base.Expressions[choice.Target]
				e.Left, e.Right = e.Right, e.Left
			}
		}
		if _, err := prepared.Compile(selected); err != nil {
			return [8]Signature{}, fmt.Errorf("composed candidate %d: %w", mask, err)
		}
		if !Encode(base, order, &result[mask]) {
			return [8]Signature{}, fmt.Errorf("candidate %d is outside the two-update profile", mask)
		}
	}
	return result, nil
}
