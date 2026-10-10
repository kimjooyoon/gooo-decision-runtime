package pathplan

import (
	"slices"
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

func assignmentFlowPlan(reverse bool) Plan {
	p := branchLocalPlan("result")
	p.Base.Statements = []bodyplan.Stmt{{Kind: "let", Name: "result", Expr: 1},
		{Kind: "assign", Name: "result", Expr: 0}, {Kind: "assign", Name: "result", Expr: 1},
		{Kind: "if", Expr: 2, Then: []int{1}, Else: []int{2}}, {Kind: "return", Expr: 3}}
	p.Base.Root = []int{0, 3, 4}
	p.Decisions[0].Target = 3
	if reverse {
		p.Base.Statements[3].Then, p.Base.Statements[3].Else = []int{2}, []int{1}
	}
	return p
}

func flowPlan(t *testing.T, plan Plan) (*PreparedPlan, decision.BranchValueFlow) {
	t.Helper()
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	flow, err := p.BranchValueFlow("branches")
	if err != nil {
		t.Fatal(err)
	}
	return p, flow
}

func TestBranchValueFlowResolvesAssignmentsAfterBranch(t *testing.T) {
	var old [2][decision.ExecutionFeatureDim]float32
	var next [2][decision.ExecutionFlowFeatureDim]float32
	want := [2]decision.FlowValue{{Present: true, Kind: "input"}, {Present: true, Kind: "int", Int: 10}}
	for i, reverse := range []bool{false, true} {
		plan := assignmentFlowPlan(reverse)
		p, flow := flowPlan(t, plan)
		if reverse {
			want[0], want[1] = want[1], want[0]
		}
		if flow.Returns != want || flow.Predicate != [2]decision.FlowValue{{Present: true, Kind: "input"}, {Present: true, Kind: "int", Int: 10}} {
			t.Fatal("return lost the last assignment", flow)
		}
		input, _ := p.InitialExecutionInput([]TestCase{{0, 10}})
		if err := input.ExecutionFeaturesInto("branches", &old[i]); err != nil {
			t.Fatal(err)
		}
		if err := input.ExecutionFlowFeaturesInto("branches", &next[i]); err != nil || !slices.Equal(old[i][:], next[i][:320]) {
			t.Fatal("new flow changed old channels", err)
		}
		plan.Base.Expressions[1].Int = 99
		again, err := p.BranchValueFlow("branches")
		if err != nil || again != flow {
			t.Fatal("mutable caller source retained", err)
		}
	}
	if old[0] != old[1] || next[0] == next[1] {
		t.Fatal("assignment collision was not distinguished")
	}
}

func TestBranchValueFlowUsesCopiedValueNotLaterWrite(t *testing.T) {
	p := branchLocalPlan("value")
	p.Base.Expressions = append(p.Base.Expressions, bodyplan.Expr{Kind: "local", Name: "saved"})
	p.Base.Statements = []bodyplan.Stmt{{Kind: "let", Name: "value", Expr: 0},
		{Kind: "let", Name: "saved", Expr: 3}, {Kind: "assign", Name: "value", Expr: 1},
		{Kind: "return", Expr: 4}, {Kind: "return", Expr: 3},
		{Kind: "if", Expr: 2, Then: []int{3}, Else: []int{4}}}
	p.Base.Root, p.Decisions[0].Target = []int{0, 1, 2, 5}, 5
	_, flow := flowPlan(t, p)
	if flow.Returns != [2]decision.FlowValue{{Present: true, Kind: "input"}, {Present: true, Kind: "int", Int: 10}} {
		t.Fatal("copied value acquired a later write", flow)
	}
	p.Decisions[0].Fallback = "layout_reverse"
	_, reversed := flowPlan(t, p)
	if reversed.Returns != [2]decision.FlowValue{flow.Returns[1], flow.Returns[0]} {
		t.Fatal("fallback not normalized", reversed)
	}
}

func TestBranchValueFlowJoinsOtherBranchesAndKeepsContinuation(t *testing.T) {
	p := assignmentFlowPlan(false)
	// A later conditional may overwrite result. Both outcomes contribute.
	p.Base.Statements = append(p.Base.Statements, bodyplan.Stmt{Kind: "assign", Name: "result", Expr: 1},
		bodyplan.Stmt{Kind: "if", Expr: 2, Then: []int{5}, Else: nil})
	p.Base.Root = []int{0, 3, 6, 4}
	_, flow := flowPlan(t, p)
	if flow.Returns != [2]decision.FlowValue{{Present: true, Kind: "unknown"}, {Present: true, Kind: "int", Int: 10}} {
		t.Fatal("unknown merge invented a definite value", flow)
	}
	// An unconditional overwrite after the branch makes both returns equal.
	p.Base.Statements = p.Base.Statements[:6]
	p.Base.Root = []int{0, 3, 5, 4}
	_, overwritten := flowPlan(t, p)
	if overwritten.Returns != [2]decision.FlowValue{{Present: true, Kind: "int", Int: 10}, {Present: true, Kind: "int", Int: 10}} {
		t.Fatal("continuation ignored", overwritten)
	}
}

func TestBranchValueFlowNoLabelsRenameExactValuesAndConcurrency(t *testing.T) {
	p := assignmentFlowPlan(false)
	p.Base.Expressions[1].Int = -9007199254740995
	p.ConditionCases = []ConditionCase{{ChoiceID: "branches", Input: 9, Expected: true}}
	prepared, flow := flowPlan(t, p)
	if flow.Returns[1].Int != -9007199254740995 || flow.Predicate[1].Int != -9007199254740995 {
		t.Fatal("lost exact literal")
	}
	p.ConditionCases[0].Expected = false
	p.Decisions[0].Intent = "different wording 다른 문구"
	p.Base.Expressions[3].Name = "renamed"
	for i := range 3 {
		p.Base.Statements[i].Name = "renamed"
	}
	_, renamed := flowPlan(t, p)
	if renamed != flow {
		t.Fatal("cases, wording or local spelling changed static values")
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			got, err := prepared.BranchValueFlow("branches")
			if err != nil || got != flow {
				t.Error("shared mutable flow scratch", err)
			}
		})
	}
	workers.Wait()
}

func TestBranchValueFlowExcludesReturnsBeforeTargetAndUsesBudget(t *testing.T) {
	p := assignmentFlowPlan(false)
	p.Base.Statements = append(p.Base.Statements, bodyplan.Stmt{Kind: "return", Expr: 1},
		bodyplan.Stmt{Kind: "if", Expr: 2, Then: []int{5}, Else: []int{3}})
	p.Base.Root = []int{0, 6, 4}
	prepared, flow := flowPlan(t, p)
	if flow.Returns != [2]decision.FlowValue{{Present: true, Kind: "input"}, {Present: true, Kind: "int", Int: 10}} {
		t.Fatal("path not reaching target contaminated return", flow)
	}
	var arena sourceFeatureArena
	arena.normalize(prepared.plan)
	a := branchFlow{arena: &arena, target: 3, remaining: 0}
	var start [2]flowState
	start[0].live = true
	a.sequence(arena.roots[:arena.rootCount], start)
	if a.err == nil {
		t.Fatal("missing static work bound")
	}
}

func TestBranchValueFlowArithmeticUnknownAndNonBranchAbsent(t *testing.T) {
	p := assignmentFlowPlan(false)
	p.Base.Expressions = append(p.Base.Expressions, bodyplan.Expr{Kind: "binary", Operation: "add", Left: 0, Right: 1})
	p.Base.Statements[1].Expr = 4
	prepared, flow := flowPlan(t, p)
	if flow.Returns[0] != (decision.FlowValue{Present: true, Kind: "unknown"}) || flow.Returns[1].Kind != "int" {
		t.Fatal("arithmetic guessed a static value", flow)
	}
	p.Decisions = append(p.Decisions, Choice{ID: "comparison", Kind: OperandOrder, Target: 2, Intent: "compare", Fallback: "layout_forward", Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}})
	prepared, err := Prepare(p)
	if err != nil {
		t.Fatal(err)
	}
	absent, err := prepared.BranchValueFlow("comparison")
	if err != nil || absent != (decision.BranchValueFlow{}) {
		t.Fatal("other choice invented branch values", err)
	}
	if _, err := prepared.BranchValueFlow("missing"); err == nil {
		t.Fatal("undeclared choice accepted")
	}
	if _, err := (*PreparedPlan)(nil).BranchValueFlow("branches"); err == nil {
		t.Fatal("missing snapshot accepted")
	}
}
