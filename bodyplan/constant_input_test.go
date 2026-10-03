package bodyplan

import (
	"math"
	"strings"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestConstantBodyRetainsDeclaredUnreadInput(t *testing.T) {
	for _, resultType := range []decision.ValueType{decision.TypeInt, decision.TypeBool} {
		t.Run(string(resultType), func(t *testing.T) {
			literal := Expr{Kind: ExprInt, Int: 42}
			if resultType == decision.TypeBool {
				literal = Expr{Kind: ExprBool, Bool: true}
			}
			plan := Plan{Schema: Schema, ID: "constant", Name: "Constant", ResultType: resultType,
				Expressions: []Expr{{Kind: ExprInput, Name: "input"}, literal},
				Statements:  []Stmt{{Kind: StmtReturn, Expr: 1}}, Root: []int{0}}
			program, err := Compile(plan, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(program.GoSource(), "Constant(input int64)") || strings.Contains(program.GoooBody(), "input") {
				t.Fatal("unread signature changed the body", program.GoSource())
			}
			for _, input := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
				value, err := program.Evaluate(input)
				if err != nil || value.Type != resultType || resultType == decision.TypeInt && value.Int != 42 ||
					resultType == decision.TypeBool && !value.Bool {
					t.Fatal("constant depends on input", input, value, err)
				}
			}
		})
	}
}

func TestUnreadInputDoesNotPermitOtherUnusedNodes(t *testing.T) {
	base := Plan{Schema: Schema, ID: "constant", Name: "Constant", ResultType: decision.TypeInt,
		Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: 42}},
		Statements:  []Stmt{{Kind: StmtReturn, Expr: 1}}, Root: []int{0}}
	for _, extra := range []Expr{{Kind: ExprInt, Int: 99}, {Kind: ExprBool, Bool: false},
		{Kind: ExprBinary, Operation: "add", Left: 0, Right: 1}, {Kind: ExprInput, Name: "input"}} {
		plan := base
		plan.Expressions = append(append([]Expr(nil), base.Expressions...), extra)
		if _, err := Compile(plan, nil); err == nil {
			t.Fatal("unused or duplicate non-parameter accepted", extra)
		}
	}
	base.Expressions = []Expr{{Kind: ExprInt, Int: 42}}
	base.Statements[0].Expr = 0
	if _, err := Compile(base, nil); err == nil || !strings.Contains(err.Error(), "exactly one Integer input") {
		t.Fatal("missing signature input accepted", err)
	}
}
