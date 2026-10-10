package pathplan

import (
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

const branchFlowWork = 8192

type flowAtom struct {
	kind uint8 // unknown, input, integer, Boolean
	int  int64
}
type flowValue struct {
	present bool
	atom    flowAtom
}
type flowState struct {
	live bool
	vars [128]flowAtom
}
type flowFrame struct {
	states  [2]flowState // paths before and after the selected branch
	returns [2]flowValue
}
type branchFlow struct {
	arena     *sourceFeatureArena
	target    int
	arm       int
	remaining int
	operands  [2]flowValue
	err       error
}

// BranchValueFlow traces local reads as snapshots of their reaching writes.
// It joins both arms of every other if: equal atoms remain known, differing
// atoms become unknown. Paths returning before the selected branch are excluded.
// Arithmetic stays unknown; this is neither evaluation nor path feasibility.
// No intent, cases, observations, chosen masks or model are read. The prior
// source representation's shadowing and reachability declines still apply.
func (p *PreparedPlan) BranchValueFlow(id string) (decision.BranchValueFlow, error) {
	var out decision.BranchValueFlow
	if _, err := p.SourceFeatures(id); err != nil {
		return out, err
	}
	for _, choice := range p.plan.Decisions {
		if choice.ID != id || choice.Kind != BranchLayout {
			continue
		}
		var arena sourceFeatureArena
		arena.normalize(p.plan)
		for _, index := range arena.roots[:arena.rootCount] {
			arena.markStatement(index)
		}
		for arm := range 2 {
			analysis := branchFlow{arena: &arena, target: choice.Target, arm: arm, remaining: branchFlowWork}
			var start [2]flowState
			start[0].live = true
			frame := analysis.sequence(arena.roots[:arena.rootCount], start)
			if analysis.err != nil {
				return decision.BranchValueFlow{}, analysis.err
			}
			out.Returns[arm] = frame.returns[1].export()
			for i, value := range analysis.operands {
				out.Predicate[i] = value.export()
			}
		}
	}
	return out, nil
}

func (f flowValue) export() decision.FlowValue {
	if !f.present {
		return decision.FlowValue{}
	}
	return decision.FlowValue{Present: true, Kind: [4]string{"unknown", "input", "int", "bool"}[f.atom.kind], Int: f.atom.int}
}

func joinFlowValue(a, b flowValue) flowValue {
	if !a.present {
		return b
	}
	if !b.present || a.atom == b.atom {
		return a
	}
	return flowValue{present: true}
}

func joinFlowState(a, b flowState) flowState {
	if !a.live {
		return b
	}
	if !b.live {
		return a
	}
	for i, value := range a.vars {
		if value != b.vars[i] {
			a.vars[i] = flowAtom{}
		}
	}
	return a
}

func (a *branchFlow) sequence(sequence []int, states [2]flowState) flowFrame {
	frame := flowFrame{states: states}
	for _, index := range sequence {
		var next [2]flowState
		for phase, state := range frame.states {
			if !state.live || a.err != nil {
				continue
			}
			a.remaining--
			if a.remaining < 0 {
				a.err = errors.New("STATIC_VALUE_FLOW_BUDGET_EXCEEDED")
				return flowFrame{}
			}
			step := a.statement(index, phase, state)
			for i := range 2 {
				next[i] = joinFlowState(next[i], step.states[i])
				frame.returns[i] = joinFlowValue(frame.returns[i], step.returns[i])
			}
		}
		frame.states = next
	}
	return frame
}

func (a *branchFlow) statement(index, phase int, state flowState) flowFrame {
	s := a.arena.statements[index]
	var out flowFrame
	switch s.Kind {
	case bodyplan.StmtLet, bodyplan.StmtAssign:
		declaration, _ := a.arena.declaration(s.Name)
		if declaration >= 0 {
			state.vars[declaration] = a.expression(s.Expr, &state)
		}
		out.states[phase] = state
	case bodyplan.StmtReturn:
		out.returns[phase] = flowValue{present: true, atom: a.expression(s.Expr, &state)}
	case bodyplan.StmtIf:
		out = a.branch(index, phase, state)
	}
	return out
}

func (a *branchFlow) branch(index, phase int, state flowState) flowFrame {
	s := a.arena.statements[index]
	if index == a.target {
		a.observeOperands(s.Expr, &state)
		var start [2]flowState
		start[1] = state
		return a.sequence([2][]int{s.Then, s.Else}[a.arm], start)
	}
	var start [2]flowState
	start[phase] = state
	left, right := a.sequence(s.Then, start), a.sequence(s.Else, start)
	for i := range 2 {
		left.states[i] = joinFlowState(left.states[i], right.states[i])
		left.returns[i] = joinFlowValue(left.returns[i], right.returns[i])
	}
	return left
}

func (a *branchFlow) expression(index int, state *flowState) flowAtom {
	e := a.arena.expressions[index]
	switch e.Kind {
	case bodyplan.ExprInput:
		return flowAtom{kind: 1}
	case bodyplan.ExprInt:
		return flowAtom{kind: 2, int: e.Int}
	case bodyplan.ExprBool:
		value := int64(0)
		if e.Bool {
			value = 1
		}
		return flowAtom{kind: 3, int: value}
	case bodyplan.ExprLocal:
		declaration, _ := a.arena.declaration(e.Name)
		if declaration >= 0 {
			return state.vars[declaration]
		}
	}
	return flowAtom{}
}

func (a *branchFlow) observeOperands(index int, state *flowState) {
	e := a.arena.expressions[index]
	if e.Kind == bodyplan.ExprBinary || e.Kind == bodyplan.ExprHole {
		a.operands[0] = joinFlowValue(a.operands[0], flowValue{true, a.expression(e.Left, state)})
		a.operands[1] = joinFlowValue(a.operands[1], flowValue{true, a.expression(e.Right, state)})
	} else {
		a.operands[0] = joinFlowValue(a.operands[0], flowValue{true, a.expression(index, state)})
	}
}
