package pathplan

import (
	"errors"
	"fmt"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

const CanonicalOrderedBranchContextSchema = "gooo/canonical-ordered-branch-context/v1"

// CanonicalOrderedBranchContext describes the resolved predicate and both
// returns as a virtual direct-return branch. Original IDs, source, declarations
// and execution remain authoritative. The view does not execute any candidate.
type CanonicalOrderedBranchContext struct {
	Schema      string                        `json:"schema"`
	Available   bool                          `json:"available"`
	Reason      string                        `json:"reason"`
	Source      [64]byte                      `json:"source_fields"`
	Expressions [3]decision.OrderedExpression `json:"expressions"`
}

func (p *PreparedPlan) CanonicalOrderedBranchContext(id string) (CanonicalOrderedBranchContext, error) {
	return p.canonicalOrderedBranchContext(id, false)
}

func (p *PreparedPlan) CanonicalOrderedBranchContextV2(id string) (CanonicalOrderedBranchContext, error) {
	return p.canonicalOrderedBranchContext(id, true)
}

func (p *PreparedPlan) canonicalOrderedBranchContext(id string, v2 bool) (CanonicalOrderedBranchContext, error) {
	result := CanonicalOrderedBranchContext{Schema: CanonicalOrderedBranchContextSchema}
	view, err := p.orderedBranchContext(id, v2)
	if v2 {
		result.Schema = "gooo/canonical-ordered-branch-context/v2"
	}
	if err != nil {
		return result, err
	}
	result.Reason = view.Reason
	if !view.Available {
		return result, nil
	}
	var arena sourceFeatureArena
	arena.resultType = p.plan.Base.ResultType
	var expressions [3]int
	for i, expression := range view.Expressions {
		expressions[i] = canonicalExpression(&arena, expression)
	}
	arena.statements[0] = bodyplan.Stmt{Kind: bodyplan.StmtReturn, Expr: expressions[1]}
	arena.statements[1] = bodyplan.Stmt{Kind: bodyplan.StmtReturn, Expr: expressions[2]}
	arena.statements[2] = bodyplan.Stmt{Kind: bodyplan.StmtIf, Expr: expressions[0], Then: []int{0}, Else: []int{1}}
	arena.statementCount, arena.rootCount, arena.roots[0] = 3, 1, 2
	arena.markStatement(2)
	for _, original := range p.plan.Decisions {
		if original.ID != id {
			continue
		}
		choice := original
		choice.Target = expressions[0]
		if choice.Kind == BranchLayout {
			choice.Target = 2
		}
		// OrderedBranchContext already applied fallback orientation. Express each
		// option relative to that view without mutating the source-owned slice.
		var options [2]Option
		for i, option := range original.Options {
			options[i] = option
			options[i].Reverse = option.Reverse != fallbackReverse(original)
		}
		choice.Options = options[:]
		if err := arena.targetFields(choice, &result.Source); err != nil {
			return result, err
		}
		arena.graphFields(&result.Source)
		for i, option := range choice.Options {
			fields := arena.optionFields(choice, option)
			copy(result.Source[44+i*10:54+i*10], fields[:])
		}
		result.Available, result.Reason, result.Expressions = true, "CANONICAL_PREDICATE_AND_RETURNS", view.Expressions
		return result, nil
	}
	return result, errors.New("canonical source choice is undeclared")
}

// At most nine nodes are needed for three already-validated flat expressions.
// Atom occurrences are kept separately, so prior node sharing cannot change
// canonical counts. Integer operations are not folded or reassociated.
func canonicalExpression(arena *sourceFeatureArena, expression decision.OrderedExpression) int {
	left := canonicalAtom(arena, expression.Operands[0])
	if expression.Operation == "value" {
		return left
	}
	right := canonicalAtom(arena, expression.Operands[1])
	index := arena.expressionCount
	arena.expressions[index] = bodyplan.Expr{Kind: bodyplan.ExprBinary, Operation: expression.Operation, Left: left, Right: right}
	arena.expressionCount++
	return index
}

func canonicalAtom(arena *sourceFeatureArena, value decision.FlowValue) int {
	e := bodyplan.Expr{Kind: bodyplan.ExprInput}
	switch value.Kind {
	case "int":
		e.Kind, e.Int = bodyplan.ExprInt, value.Int
	case "bool":
		e.Kind, e.Bool = bodyplan.ExprBool, value.Int != 0
	}
	index := arena.expressionCount
	arena.expressions[index] = e
	arena.expressionCount++
	return index
}

// CanonicalOrderedSourceFeaturesInto is a new ABI: canonical source facts
// [0:64], unchanged intent [64:192], reserved zeros [192:384], and exact ordered
// expressions [384:528]. Declared output and condition rows stay separate.
// Existing ordered/relational inputs are not changed. No model or case is read
// to resolve the source; failures preserve the caller-owned destination.
func (input *ContractInput) CanonicalOrderedSourceFeaturesInto(id string, out *[528]float32) error {
	if input == nil || input.initial.prepared == nil || out == nil {
		return errors.New("canonical source input and destination required")
	}
	view, err := input.initial.prepared.CanonicalOrderedBranchContext(id)
	if err != nil {
		return err
	}
	if !view.Available {
		return fmt.Errorf("CANONICAL_SOURCE_CONTEXT_UNAVAILABLE: %s", view.Reason)
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
		if err := decision.OrderedExpressionFeaturesInto(view.Expressions, &suffix); err != nil {
			return err
		}
		var next [528]float32
		copy(next[:192], prefix[:192])
		copy(next[384:], suffix[:])
		*out = next
		return nil
	}
	return errors.New("canonical source choice is undeclared")
}
