package orderprepared_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/orderjudge"
	"github.com/kimjooyoon/gooo-decision-runtime/orderprepared"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func fixture() pathplan.Plan {
	p := pathplan.Plan{Schema: pathplan.Schema, Base: bodyplan.Plan{
		Schema: bodyplan.Schema, ID: "prepared", Name: "Compose", ResultType: decision.TypeInt,
		Expressions: []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "local", Name: "v"}, {Kind: "int", Int: 1},
			{Kind: "binary", Operation: "subtract", Left: 1, Right: 2}, {Kind: "int", Int: 2},
			{Kind: "binary", Operation: "multiply", Left: 1, Right: 4}},
		Statements: []bodyplan.Stmt{{Kind: "let", Name: "v", Expr: 0}, {Kind: "assign", Name: "v", Expr: 3},
			{Kind: "assign", Name: "v", Expr: 5}, {Kind: "return", Expr: 1}}, Root: []int{0, 1, 2, 3}}}
	for i, target := range []int{3, 5} {
		p.Decisions = append(p.Decisions, pathplan.Choice{ID: []string{"a", "b"}[i], Kind: pathplan.OperandOrder,
			Target: target, Intent: "Keep operands.", Options: []pathplan.Option{{Label: "layout_forward"},
				{Label: "layout_reverse", Reverse: true}}, Fallback: "layout_forward"})
	}
	p.Decisions = append(p.Decisions, pathplan.Choice{ID: "root", Kind: pathplan.RootOrder, Intent: "2를 곱한 뒤 1을 뺀다.",
		Options: []pathplan.Option{{Label: "schedule_forward", Order: []int{0, 1, 2, 3}},
			{Label: "schedule_reverse", Order: []int{0, 2, 1, 3}}}, Fallback: "schedule_forward"})
	return p
}

func model(t *testing.T) *orderjudge.Model {
	t.Helper()
	var weights [orderjudge.ParameterCount]float32
	for i := range weights {
		weights[i] = float32(i%13-6) * 0.03
	}
	m, err := orderjudge.New(weights)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestAllFallbacksBudgetsAndDedupMatchOriginal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, m := range []*orderjudge.Model{nil, model(t)} {
		runtime, err := orderprepared.NewRuntime(m)
		if err != nil {
			t.Fatal(err)
		}
		for fallback := range 8 {
			plan := fixture()
			for bit := range plan.Decisions {
				c := &plan.Decisions[bit]
				c.Fallback = c.Options[(fallback>>bit)&1].Label
			}
			p, err := runtime.Prepare(ctx, plan)
			if err != nil {
				t.Fatal(err)
			}
			for _, dedup := range []bool{false, true} {
				for budget := 1; budget <= 8; budget++ {
					for _, cases := range [][]pathplan.TestCase{{{Input: -2, Expected: -5}, {Input: 3, Expected: 5}}, {{Input: 0, Expected: 999}}} {
						want, wp, wr, err := orderjudge.Search(ctx, plan, m, cases, budget, dedup)
						if err != nil {
							t.Fatal(err)
						}
						got, gp, gr, err := p.Search(ctx, p.PlanSHA256(), cases, budget, dedup)
						if err != nil {
							t.Fatal(err)
						}
						wr.PredictNS, gr.PredictNS = 0, 0
						if !reflect.DeepEqual(want, got) || !reflect.DeepEqual(wr, gr) ||
							wp.GoooSource() != gp.GoooSource() || wp.GoSource() != gp.GoSource() {
							t.Fatalf("replay differs: model=%v fallback=%d budget=%d dedup=%v", m != nil, fallback, budget, dedup)
						}
					}
				}
			}
		}
	}
}

func TestOwnershipDifferentCasesAndFourConcurrentCallers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m := model(t)
	runtime, err := orderprepared.NewRuntime(m)
	if err != nil {
		t.Fatal(err)
	}
	plan := fixture()
	p, err := runtime.Prepare(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	cases := []pathplan.TestCase{{Input: 3, Expected: 5}}
	want, _, ranking, err := p.Search(ctx, p.PlanSHA256(), cases, 8, true)
	if err != nil {
		t.Fatal(err)
	}
	identity := runtime.Identity()
	if identity.MetadataSHA256 != want.Selection.MetadataSHA256 || identity.WeightsSHA256 != want.Selection.WeightsSHA256 ||
		identity.TensorBytes != 16384 || (orderprepared.Runtime{}).Identity() != (orderprepared.Identity{}) {
		t.Fatal("captured model identity differs", identity)
	}
	*m = orderjudge.Model{}
	runtime = orderprepared.Runtime{}
	plan.Base.Expressions[2].Int = 999
	plan.Decisions[0].ID = "changed"
	plan.Decisions[2].Options[0].Order[0] = 3
	plan.Decisions[2].Intent = "changed"
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			for range 4 {
				got, program, receipt, err := p.Search(ctx, p.PlanSHA256(), cases, 8, true)
				if err != nil || !reflect.DeepEqual(want, got) || receipt.Prediction != ranking.Prediction {
					t.Error("caller state escaped", err)
					return
				}
				got.Selection.Choices["a"] = "changed"
				got.Attempts[0].Results[0].Actual = 999
				receipt.Aliases = append(receipt.Aliases, orderjudge.Alias{Mask: 7})
				*program = bodyplan.Program{}
			}
		})
	}
	group.Wait()
	other, _, receipt, err := p.Search(ctx, p.PlanSHA256(), []pathplan.TestCase{{Input: 3, Expected: 999}}, 8, true)
	if err != nil || other.Status != "PARTIAL" || other.SelectedTrainingPassed != 0 || receipt.Prediction != ranking.Prediction {
		t.Fatal("test facts leaked between calls", err)
	}
}

func TestRejectStaleBindingBoundsAndCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	runtime, _ := orderprepared.NewRuntime(model(t))
	p, err := runtime.Prepare(ctx, fixture())
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct {
		ctx    context.Context
		sha    string
		cases  []pathplan.TestCase
		budget int
	}{{nil, p.PlanSHA256(), []pathplan.TestCase{{}}, 1}, {context.Background(), p.PlanSHA256(), []pathplan.TestCase{{}}, 1},
		{ctx, "", []pathplan.TestCase{{}}, 1}, {ctx, strings.Repeat("a", 64), []pathplan.TestCase{{}}, 1},
		{ctx, p.PlanSHA256(), nil, 1}, {ctx, p.PlanSHA256(), make([]pathplan.TestCase, 129), 1},
		{ctx, p.PlanSHA256(), []pathplan.TestCase{{}}, 0}, {ctx, p.PlanSHA256(), []pathplan.TestCase{{}}, 9}} {
		r, program, receipt, err := p.Search(bad.ctx, bad.sha, bad.cases, bad.budget, true)
		if err == nil || r.Selection.ModelCalls != 0 || program != nil || receipt != nil {
			t.Fatal("invalid call performed work", err)
		}
	}
	cancel()
	if _, err := runtime.Prepare(ctx, fixture()); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled preparation", err)
	}
	if _, _, _, err := p.Search(ctx, p.PlanSHA256(), []pathplan.TestCase{{}}, 1, true); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled search", err)
	}
}

func TestUnsupportedInputFailsPreparationAtomically(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	runtime, _ := orderprepared.NewRuntime(model(t))
	for _, mutate := range []func(*pathplan.Plan){
		func(p *pathplan.Plan) { p.Decisions = p.Decisions[:2] },
		func(p *pathplan.Plan) { p.Decisions[0].Target = 999 },
		func(p *pathplan.Plan) { p.Decisions[0].Kind = pathplan.LocalReference },
		func(p *pathplan.Plan) { p.Decisions[0].Options = nil },
		func(p *pathplan.Plan) { p.Base.Expressions[2].Int = 17 },
		func(p *pathplan.Plan) { p.Decisions[0].Intent = strings.Repeat("x", 513) },
	} {
		plan := fixture()
		mutate(&plan)
		p, err := runtime.Prepare(ctx, plan)
		if err == nil || p != nil {
			t.Fatal("partial prepared object escaped", err)
		}
	}
}

type cancelAfter struct {
	context.Context
	remaining int
}

func (c *cancelAfter) Err() error {
	c.remaining--
	if c.remaining < 0 {
		return context.Canceled
	}
	return nil
}

func TestCancellationDuringPreparationAndEvaluation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	runtime, _ := orderprepared.NewRuntime(model(t))
	p, err := runtime.Prepare(&cancelAfter{ctx, 3}, fixture())
	if p != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("partial preparation escaped", err)
	}
	p, err = runtime.Prepare(ctx, fixture())
	if err != nil {
		t.Fatal(err)
	}
	r, program, receipt, err := p.Search(&cancelAfter{ctx, 2}, p.PlanSHA256(), []pathplan.TestCase{{}}, 8, true)
	if !errors.Is(err, context.Canceled) || program != nil || r.Evaluated != 0 || receipt == nil || r.Selection.ModelCalls != 1 {
		t.Fatal("evaluation cancellation differs", err, r)
	}
}
