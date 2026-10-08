package bodyplan

import (
	"strings"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestUnreadLocalKeepsInitializationAndAssignment(t *testing.T) {
	plan := Plan{Schema: Schema, ID: "unread", Name: "Unread", ResultType: decision.TypeInt,
		Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: 3}},
		Statements: []Stmt{{Kind: StmtLet, Name: "future", Expr: 0},
			{Kind: StmtAssign, Name: "future", Expr: 1}, {Kind: StmtReturn, Expr: 0}}, Root: []int{0, 1, 2}}
	program, err := Compile(plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"var future = input\n\t_ = future", "future = int64(3)"} {
		if !strings.Contains(program.GoSource(), fragment) {
			t.Fatal("local evaluation was dropped", program.GoSource())
		}
	}
	if strings.Contains(program.GoooBody(), "_ =") || !strings.Contains(program.GoooBody(), "let future = input") {
		t.Fatal("target marker changed source body", program.GoooBody())
	}
	if value, err := program.Evaluate(7); err != nil || value.Int != 7 {
		t.Fatal("unused local changed interpretation", value, err)
	}
	plan.Expressions[1] = Expr{Kind: ExprBool, Bool: true}
	if _, err := Compile(plan, nil); err == nil {
		t.Fatal("unused local bypassed assignment type checking")
	}
}

func TestUnreadLocalIsResolvedByScope(t *testing.T) {
	plan := Plan{Schema: Schema, ID: "scoped-unread", Name: "Scoped", ResultType: decision.TypeInt,
		Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: 0},
			{Kind: ExprBinary, Operation: "less_than", Left: 0, Right: 1}, {Kind: ExprLocal, Name: "value"}},
		Statements: []Stmt{{Kind: StmtIf, Expr: 2, Then: []int{1, 2}, Else: []int{3, 4}},
			{Kind: StmtLet, Name: "value", Expr: 0}, {Kind: StmtReturn, Expr: 3},
			{Kind: StmtLet, Name: "value", Expr: 1}, {Kind: StmtReturn, Expr: 0}}, Root: []int{0}}
	program, err := Compile(plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(program.GoSource(), "_ = value") != 1 ||
		!strings.Contains(program.GoSource(), "var value = int64(0)\n\t\t_ = value") {
		t.Fatal("same-name local reads crossed branch scope", program.GoSource())
	}
	for _, input := range []int64{-1, 1} {
		if value, err := program.Evaluate(input); err != nil || value.Int != input {
			t.Fatal("branch interpretation differs", value, err)
		}
	}
	used, err := Compile(arithmeticPlan(), nil)
	if err != nil || strings.Contains(used.GoSource(), "_ =") {
		t.Fatal("used locals acquired target markers", err)
	}
}
