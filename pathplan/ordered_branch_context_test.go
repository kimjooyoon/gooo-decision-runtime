package pathplan

import (
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

func orderedArithmeticPlan() Plan {
	p := executionMaxPlan()
	p.Base.Expressions = append(p.Base.Expressions,
		bodyplan.Expr{Kind: "binary", Operation: "subtract", Left: 0, Right: 1},
		bodyplan.Expr{Kind: "binary", Operation: "subtract", Left: 1, Right: 0})
	p.Base.Statements[0].Expr, p.Base.Statements[1].Expr = 3, 4
	return p
}

func TestOrderedContextPreservesOperandBranchFallbackAndSnapshot(t *testing.T) {
	p := orderedArithmeticPlan()
	prepared, err := Prepare(p)
	if err != nil {
		t.Fatal(err)
	}
	view, err := prepared.OrderedBranchContext(p.Decisions[0].ID)
	if err != nil || !view.Available || view.Expressions[1].Operation != "subtract" ||
		view.Expressions[1].Operands[0].Kind != "input" || view.Expressions[2].Operands[0].Int != 10 {
		t.Fatal(view, err)
	}
	p.Base.Expressions[1].Int = 9007199254740993
	p.Decisions[0].Fallback = p.Decisions[0].Options[1].Label
	p.Decisions[1].Fallback = p.Decisions[1].Options[1].Label
	reversed, err := Prepare(p)
	if err != nil {
		t.Fatal(err)
	}
	other, err := reversed.OrderedBranchContext(p.Decisions[0].ID)
	if err != nil || !other.Available || other.Expressions[0].Operands[0].Int != 9007199254740993 ||
		other.Expressions[1].Operands[0].Int != 9007199254740993 {
		t.Fatal("fallback orientation or exact literal lost", other, err)
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			got, err := prepared.OrderedBranchContext(p.Decisions[0].ID)
			if err != nil || got != view {
				t.Error("snapshot changed", err)
			}
		})
	}
	wg.Wait()
	if _, err := prepared.OrderedBranchContext("absent"); err == nil {
		t.Fatal("missing choice accepted")
	}
	var missing *PreparedPlan
	if _, err := missing.OrderedBranchContext("absent"); err == nil {
		t.Fatal("nil source accepted")
	}
}

func TestOrderedContextReachingWritesAndConservativeDeclines(t *testing.T) {
	p := orderedArithmeticPlan()
	p.Base.Expressions = append(p.Base.Expressions, bodyplan.Expr{Kind: "local", Name: "value"})
	p.Base.Statements = append(p.Base.Statements,
		bodyplan.Stmt{Kind: "let", Name: "value", Expr: 3},
		bodyplan.Stmt{Kind: "assign", Name: "value", Expr: 4})
	p.Base.Root = []int{3, 2}
	p.Base.Statements[0].Expr, p.Base.Statements[1].Expr = 5, 5
	p.Base.Statements[2].Else = []int{4, 1}
	prepared, err := Prepare(p)
	if err != nil {
		t.Fatal(err)
	}
	view, err := prepared.OrderedBranchContext(p.Decisions[0].ID)
	if err != nil || !view.Available || view.Expressions[1].Operands[0].Kind != "input" ||
		view.Expressions[2].Operands[0].Int != 10 {
		t.Fatal("assignment snapshot lost", view, err)
	}
	for _, kind := range []string{"nested_expression", "nested_branch", "early_return", "operation_hole"} {
		p := orderedArithmeticPlan()
		p.ConditionCases = nil
		switch kind {
		case "nested_expression":
			p.Base.Expressions = append(p.Base.Expressions, bodyplan.Expr{Kind: "binary", Operation: "add", Left: 3, Right: 1})
			p.Base.Statements[0].Expr = 5
		case "nested_branch":
			p.Base.Statements = append(p.Base.Statements, bodyplan.Stmt{Kind: "if", Expr: 2, Then: []int{0}, Else: []int{4}}, p.Base.Statements[1])
			p.Base.Statements[2].Then = []int{3}
		case "early_return":
			p.Base.Statements = append(p.Base.Statements, p.Base.Statements[0])
			p.Base.Root = []int{3, 2}
		case "operation_hole":
			p.Base.Expressions[3] = bodyplan.Expr{Kind: "hole", HoleID: "math", Text: "compute", Left: 0, Right: 1, Allowed: []string{"add", "subtract"}, Fallback: "subtract"}
		}
		prepared, err := Prepare(p)
		if kind == "early_return" {
			if err == nil {
				t.Fatal("unreachable branch bypassed compiler validation")
			}
			continue
		}
		if err != nil {
			t.Fatal(kind, err)
		}
		got, err := prepared.OrderedBranchContext(p.Decisions[0].ID)
		if err != nil || got.Available || got.Reason == "" || got.Expressions != ([3]decision.OrderedExpression{}) {
			t.Fatal("unsupported context advertised complete", kind, got, err)
		}
	}
}
