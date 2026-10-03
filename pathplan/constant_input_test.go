package pathplan

import (
	"context"
	"math"
	"testing"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

func TestConstantPathsSearchAndProbeWithUnreadInput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	plan := Plan{Schema: Schema, Base: bodyplan.Plan{Schema: bodyplan.Schema, ID: "constant-paths", Name: "Constant", ResultType: decision.TypeInt,
		Expressions: []bodyplan.Expr{{Kind: bodyplan.ExprInput, Name: "input"}, {Kind: bodyplan.ExprInt, Int: 2},
			{Kind: bodyplan.ExprInt, Int: 3}, {Kind: bodyplan.ExprBinary, Operation: "subtract", Left: 1, Right: 2}},
		Statements: []bodyplan.Stmt{{Kind: bodyplan.StmtReturn, Expr: 3}}, Root: []int{0}},
		Decisions: []Choice{{ID: "order", Kind: OperandOrder, Target: 3, Intent: "3에서 2를 뺀다. Subtract two from three.",
			Fallback: "layout_forward", Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}}}}
	result, program, err := Search(ctx, plan, nil, []TestCase{{Input: 0, Expected: 1}}, 2, "")
	if err != nil || program == nil || result.Selection.ModelCalls != 0 {
		t.Fatal("constant search failed", err)
	}
	for _, input := range []int64{math.MinInt64, -7, 0, 17, math.MaxInt64} {
		value, err := program.Evaluate(input)
		if err != nil || value.Int != 1 {
			t.Fatal("constant candidate changed with input", input, value, err)
		}
	}
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	ranking, err := prepared.RankProbes(ctx, []TestCase{{Input: 0, Expected: 1}}, []int64{-2, 2}, 2)
	if err != nil || len(ranking.SurvivingMasks) != 1 || ranking.SurvivingMasks[0] != 1 {
		t.Fatal("constant probe did not retain one candidate", ranking, err)
	}
}
