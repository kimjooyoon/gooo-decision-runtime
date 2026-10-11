package bodyplan

import (
	"math"
	"strings"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestTypedArenaIntegerDivision(t *testing.T) {
	for _, denominator := range []int64{-3, -1, 1, 3} {
		p := Plan{Schema: Schema, ID: "division", Name: "Divide", ResultType: decision.TypeInt,
			Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: denominator},
				{Kind: ExprBinary, Operation: "divide", Left: 0, Right: 1}}, Statements: []Stmt{{Kind: StmtReturn, Expr: 2}}, Root: []int{0}}
		body, err := Compile(p, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(body.GoSource(), " / ") || !strings.Contains(body.GoooSource(), " / ") {
			t.Fatal("division emission missing")
		}
		for _, input := range []int64{math.MinInt64, -8, -1, 0, 1, 8, math.MaxInt64} {
			value, err := body.Evaluate(input)
			if err != nil || value.Type != decision.TypeInt || value.Int != input/denominator {
				t.Fatal(input, denominator, value, err)
			}
		}
	}
}

func TestTypedArenaDynamicZeroDivisionReturnsError(t *testing.T) {
	p := Plan{Schema: Schema, ID: "zero-division", Name: "Divide", ResultType: decision.TypeInt,
		Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: 1},
			{Kind: ExprBinary, Operation: "divide", Left: 1, Right: 0}}, Statements: []Stmt{{Kind: StmtReturn, Expr: 2}}, Root: []int{0}}
	body, err := Compile(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := body.Evaluate(0); err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatal("zero division was not an error", err)
	}
	value, err := body.Evaluate(-1)
	if err != nil || value.Int != -1 {
		t.Fatal(value, err)
	}
}
