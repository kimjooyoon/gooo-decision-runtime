package orderjudge

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func searchFixture() pathplan.Plan {
	p := pathplan.Plan{Schema: pathplan.Schema, Base: bodyplan.Plan{
		Schema: bodyplan.Schema, ID: "search", Name: "Compose", ResultType: decision.TypeInt,
		Expressions: []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "local", Name: "v"}, {Kind: "int", Int: 1},
			{Kind: "binary", Operation: "add", Left: 1, Right: 2}, {Kind: "int", Int: 2},
			{Kind: "binary", Operation: "multiply", Left: 1, Right: 4}},
		Statements: []bodyplan.Stmt{{Kind: "let", Name: "v", Expr: 0}, {Kind: "assign", Name: "v", Expr: 3},
			{Kind: "assign", Name: "v", Expr: 5}, {Kind: "return", Expr: 1}}, Root: []int{0, 1, 2, 3}}}
	for i, target := range []int{3, 5} {
		p.Decisions = append(p.Decisions, pathplan.Choice{ID: []string{"a", "b"}[i], Kind: pathplan.OperandOrder,
			Target: target, Intent: "Keep operands.", Options: []pathplan.Option{{Label: "layout_forward"},
				{Label: "layout_reverse", Reverse: true}}, Fallback: "layout_forward"})
	}
	p.Decisions = append(p.Decisions, pathplan.Choice{ID: "root", Kind: pathplan.RootOrder, Intent: "Multiply then add.",
		Options: []pathplan.Option{{Label: "schedule_forward", Order: []int{0, 1, 2, 3}},
			{Label: "schedule_reverse", Order: []int{0, 2, 1, 3}}}, Fallback: "schedule_forward"})
	return p
}

func TestSearchSkipsOnlyEqualBodiesAndKeepsPredictionsIndependentOfCases(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	m, _ := New([ParameterCount]float32{})
	p := searchFixture()
	cases := []pathplan.TestCase{{Input: 3, Expected: 7}, {Input: -1, Expected: -1}}
	plain, program, a, err := Search(ctx, p, m, cases, 8, false)
	if err != nil || program == nil || plain.Evaluated != 5 || plain.SelectedTrainingPassed != 2 || plain.Selection.ModelCalls != 1 {
		t.Fatal("unfiltered search", err, plain)
	}
	unique, selected, b, err := Search(ctx, p, m, cases, 8, true)
	if err != nil || unique.Evaluated != 2 || len(b.Aliases) != 3 || unique.Unattempted != 6 || selected.GoooSource() != program.GoooSource() {
		t.Fatal("descriptor search", err, unique, b)
	}
	_, _, c, err := Search(ctx, p, m, []pathplan.TestCase{{Input: 42, Expected: 999}}, 1, true)
	if err != nil || a.Prediction != b.Prediction || a.Prediction != c.Prediction {
		t.Fatal("finite expected outputs influenced ranking", err)
	}
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			r, _, receipt, err := Search(ctx, p, m, cases, 8, true)
			if err != nil || r.Evaluated != 2 || receipt.Prediction != b.Prediction {
				t.Error("shared model search differs", err)
			}
		})
	}
	group.Wait()
}

func TestDeterministicSearchMatchesSDKAcrossFallbacksAndBudgets(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for fallback := range 8 {
		p := searchFixture()
		for bit := range p.Decisions {
			c := &p.Decisions[bit]
			c.Fallback = c.Options[(fallback>>bit)&1].Label
		}
		for budget := 1; budget <= 8; budget++ {
			cases := []pathplan.TestCase{{Input: 0, Expected: 999}}
			r, _, receipt, err := Search(ctx, p, nil, cases, budget, false)
			if err != nil {
				t.Fatal(err)
			}
			sdk, _, err := pathplan.Search(ctx, p, nil, cases, budget, "")
			if err != nil || !reflect.DeepEqual(r.Attempts, sdk.Attempts) || r.Selection.ModelCalls != 0 || receipt.PredictNS != 0 {
				t.Fatal("deterministic order diverges", fallback, budget, err)
			}
		}
	}
}

func TestSearchDeclinesBeforePrediction(t *testing.T) {
	m, _ := New([ParameterCount]float32{})
	cases := []pathplan.TestCase{{Input: 0, Expected: 1}}
	for _, ctx := range []context.Context{nil, context.Background()} {
		if _, _, _, err := Search(ctx, searchFixture(), m, cases, 8, false); err == nil {
			t.Fatal("missing context/deadline accepted")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	p := searchFixture()
	p.Base.Expressions[2].Int = 17
	r, program, _, err := Search(ctx, p, m, cases, 8, false)
	if err == nil || program != nil || r.Selection.ModelCalls != 0 || r.Evaluated != 0 {
		t.Fatal("unsupported model constant evaluated", err)
	}
	cancel()
	if _, _, _, err := Search(ctx, searchFixture(), m, cases, 8, false); err == nil {
		t.Fatal("cancelled search accepted")
	}
}
