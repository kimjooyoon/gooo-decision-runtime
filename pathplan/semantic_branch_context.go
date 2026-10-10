package pathplan

import (
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

// SemanticBranchContext describes a bounded, source-only normalization. An
// unsupported shape keeps its full v4 representation. It does not inspect
// intent, test cases, observed outcomes or a model. Known atoms describe both
// assumed arms, not whether an input reaches either arm.
type SemanticBranchContext struct {
	Normalized bool                     `json:"normalized"`
	Reason     string                   `json:"reason"`
	Source     [64]byte                 `json:"source_fields"`
	Flow       decision.BranchValueFlow `json:"value_flow"`
}

func (p *PreparedPlan) SemanticBranchContext(id string) (SemanticBranchContext, error) {
	var out SemanticBranchContext
	fields, err := p.SourceFeatures(id)
	if err != nil {
		return out, err
	}
	out.Source, out.Reason = fields, "SOURCE_SHAPE_RETAINED"
	var arena sourceFeatureArena
	arena.normalize(p.plan)
	for _, root := range arena.roots[:arena.rootCount] {
		arena.markStatement(root)
	}
	branch, ok := semanticBranch(&arena)
	if !ok || !semanticBranchChoices(p.plan.Decisions, &arena, branch) {
		return out, nil
	}
	flow, err := semanticBranchFlow(&arena, branch)
	if err != nil {
		return out, err
	}
	for _, atom := range [4]decision.FlowValue{flow.Returns[0], flow.Returns[1], flow.Predicate[0], flow.Predicate[1]} {
		if !atom.Present || atom.Kind == "unknown" {
			out.Reason = "UNKNOWN_VALUE_RETAINS_SOURCE_SHAPE"
			return out, nil
		}
	}
	out.Normalized, out.Reason, out.Flow = true, "SINGLE_BRANCH_KNOWN_ATOMS", flow
	clear(out.Source[7:13])
	out.Source[11] = 128     // the resolved predicate is a binary expression
	clear(out.Source[25:33]) // local spelling is replaced by exact predicate atoms
	clear(out.Source[35:44]) // counts of syntax nodes are not value flow
	out.Source[35] = sourceCount(len(p.plan.Decisions))
	for i := range 2 {
		clear(out.Source[45+i*10 : 53+i*10]) // retain only each option's orientation
	}
	return out, nil
}

// Exactly one root branch, no earlier return, no nested branch, no arithmetic
// other than its predicate. Repeated root nodes and other mutable choices are
// outside this normalization. The compiler can still assemble those programs.
func semanticBranch(a *sourceFeatureArena) (int, bool) {
	branch := -1
	var seen [128]bool
	for _, index := range a.roots[:a.rootCount] {
		if seen[index] {
			return 0, false
		}
		seen[index] = true
		s := a.statements[index]
		if s.Kind == bodyplan.StmtReturn && branch < 0 {
			return 0, false
		}
		if s.Kind == bodyplan.StmtIf {
			if branch >= 0 {
				return 0, false
			}
			branch = index
		}
	}
	if branch < 0 {
		return 0, false
	}
	predicate := a.statements[branch].Expr
	for i, s := range a.statements[:a.statementCount] {
		if a.statementReachable[i] && s.Kind == bodyplan.StmtIf && i != branch {
			return 0, false
		}
	}
	for i, e := range a.expressions[:a.expressionCount] {
		if !a.expressionReachable[i] {
			continue
		}
		switch e.Kind {
		case bodyplan.ExprInput, bodyplan.ExprInt, bodyplan.ExprBool, bodyplan.ExprLocal:
		case bodyplan.ExprBinary, bodyplan.ExprHole:
			if i != predicate {
				return 0, false
			}
		default:
			return 0, false
		}
	}
	e := a.expressions[predicate]
	return branch, e.Kind == bodyplan.ExprBinary || e.Kind == bodyplan.ExprHole
}

func semanticBranchChoices(choices []Choice, a *sourceFeatureArena, branch int) bool {
	var seen [2]bool
	for _, choice := range choices {
		var index int
		switch {
		case choice.Kind == BranchLayout && choice.Target == branch:
			index = 0
		case choice.Kind == OperandOrder && choice.Target == a.statements[branch].Expr:
			index = 1
		default:
			return false
		}
		if seen[index] {
			return false
		}
		seen[index] = true
	}
	return len(choices) > 0
}

func semanticBranchFlow(a *sourceFeatureArena, branch int) (decision.BranchValueFlow, error) {
	var out decision.BranchValueFlow
	for arm := range 2 {
		analysis := branchFlow{arena: a, target: branch, arm: arm, remaining: branchFlowWork}
		var start [2]flowState
		start[0].live = true
		frame := analysis.sequence(a.roots[:a.rootCount], start)
		if analysis.err != nil {
			return out, analysis.err
		}
		if frame.returns[0].present {
			return out, errors.New("semantic branch normalization encountered an earlier return")
		}
		out.Returns[arm] = frame.returns[1].export()
		for i, value := range analysis.operands {
			out.Predicate[i] = value.export()
		}
	}
	return out, nil
}
