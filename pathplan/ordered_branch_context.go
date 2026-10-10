package pathplan

import (
	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

const OrderedBranchContextSchema = "gooo/ordered-branch-context/v1"

// OrderedBranchContext exposes a source-only adjunct for one selected branch.
// Expressions are predicate, then return and else return, after fallback
// normalization and reaching-write substitution. Available is false when this
// bounded representation cannot describe the body; compilation remains valid.
type OrderedBranchContext struct {
	Schema      string                        `json:"schema"`
	Available   bool                          `json:"available"`
	Reason      string                        `json:"reason"`
	Expressions [3]decision.OrderedExpression `json:"expressions"`
}

func (p *PreparedPlan) OrderedBranchContext(id string) (OrderedBranchContext, error) {
	out := OrderedBranchContext{Schema: OrderedBranchContextSchema, Reason: "SINGLE_ROOT_BRANCH_REQUIRED"}
	if _, err := p.SourceFeatures(id); err != nil {
		return out, err
	}
	var arena sourceFeatureArena
	arena.normalize(p.plan)
	for _, root := range arena.roots[:arena.rootCount] {
		arena.markStatement(root)
	}
	branch := orderedRootBranch(&arena)
	if branch < 0 {
		return out, nil
	}
	if !semanticBranchChoices(p.plan.Decisions, &arena, branch) {
		out.Reason = "ONLY_PREDICATE_ORDER_AND_BRANCH_LAYOUT_SUPPORTED"
		return out, nil
	}
	var expressions [3]decision.OrderedExpression
	for arm := range 2 {
		trace := orderedTrace{arena: &arena, arm: arm}
		returned, value := trace.sequence(arena.roots[:arena.rootCount])
		if trace.reason != "" || !trace.reached || !returned {
			out.Reason = trace.reason
			if out.Reason == "" {
				out.Reason = "RETURN_AFTER_SELECTED_BRANCH_REQUIRED"
			}
			return out, nil
		}
		expressions[0], expressions[1+arm] = trace.predicate, value
	}
	var features [decision.OrderedExpressionFeatureDim]float32
	if err := decision.OrderedExpressionFeaturesInto(expressions, &features); err != nil {
		out.Reason = "KNOWN_ATOMS_AND_ONE_OPERATION_PER_EXPRESSION_REQUIRED"
		return out, nil
	}
	out.Available, out.Reason, out.Expressions = true, "ORDERED_PREDICATE_AND_RETURNS", expressions
	return out, nil
}

func orderedRootBranch(a *sourceFeatureArena) int {
	branch := -1
	for _, index := range a.roots[:a.rootCount] {
		if a.statements[index].Kind == bodyplan.StmtIf {
			if branch >= 0 {
				return -1
			}
			branch = index
		}
	}
	for i, statement := range a.statements[:a.statementCount] {
		if a.statementReachable[i] && statement.Kind == bodyplan.StmtIf && i != branch {
			return -1
		}
	}
	return branch
}

type orderedTrace struct {
	arena     *sourceFeatureArena
	arm       int
	variables [128]decision.OrderedExpression
	predicate decision.OrderedExpression
	reached   bool
	reason    string
}

func (t *orderedTrace) sequence(sequence []int) (bool, decision.OrderedExpression) {
	for _, index := range sequence {
		s := t.arena.statements[index]
		switch s.Kind {
		case bodyplan.StmtLet, bodyplan.StmtAssign:
			declaration, _ := t.arena.declaration(s.Name)
			if declaration < 0 {
				t.reason = "LOCAL_DECLARATION_REQUIRED"
				return false, decision.OrderedExpression{}
			}
			t.variables[declaration] = t.expression(s.Expr)
		case bodyplan.StmtReturn:
			return true, t.expression(s.Expr)
		case bodyplan.StmtIf:
			t.predicate, t.reached = t.expression(s.Expr), true
			if returned, value := t.sequence([2][]int{s.Then, s.Else}[t.arm]); returned {
				return true, value
			}
		}
	}
	return false, decision.OrderedExpression{}
}

func (t *orderedTrace) expression(index int) decision.OrderedExpression {
	e := t.arena.expressions[index]
	atom := decision.FlowValue{Present: true}
	switch e.Kind {
	case bodyplan.ExprInput:
		atom.Kind = "input"
	case bodyplan.ExprInt:
		atom.Kind, atom.Int = "int", e.Int
	case bodyplan.ExprBool:
		atom.Kind = "bool"
		if e.Bool {
			atom.Int = 1
		}
	case bodyplan.ExprLocal:
		declaration, _ := t.arena.declaration(e.Name)
		if declaration < 0 {
			return decision.OrderedExpression{}
		}
		return t.variables[declaration]
	case bodyplan.ExprBinary:
		left, right := t.expression(e.Left), t.expression(e.Right)
		if left.Operation == "value" && right.Operation == "value" {
			return decision.OrderedExpression{Operation: e.Operation, Operands: [2]decision.FlowValue{left.Operands[0], right.Operands[0]}}
		}
		return decision.OrderedExpression{}
	default:
		t.reason = "OPERATION_HOLE_REQUIRES_SEPARATE_CONTEXT"
		return decision.OrderedExpression{}
	}
	return decision.OrderedExpression{Operation: "value", Operands: [2]decision.FlowValue{atom}}
}
