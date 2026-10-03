package orderfacts

import (
	"math"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func fixture() pathplan.Plan {
	return pathplan.Plan{Schema: pathplan.Schema, Base: bodyplan.Plan{
		Schema: bodyplan.Schema, ID: "order", Name: "Compose", ResultType: decision.TypeInt,
		Expressions: []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "local", Name: "v"}, {Kind: "int", Int: 1},
			{Kind: "binary", Operation: "add", Left: 1, Right: 2}, {Kind: "int", Int: 2},
			{Kind: "binary", Operation: "multiply", Left: 1, Right: 4}},
		Statements: []bodyplan.Stmt{{Kind: "let", Name: "v", Expr: 0}, {Kind: "assign", Name: "v", Expr: 3},
			{Kind: "assign", Name: "v", Expr: 5}, {Kind: "return", Expr: 1}}, Root: []int{0, 1, 2, 3}},
		Decisions: []pathplan.Choice{{ID: "order", Kind: pathplan.RootOrder, Intent: "multiply then add",
			Options: []pathplan.Option{{Label: "schedule_forward", Order: []int{0, 1, 2, 3}},
				{Label: "schedule_reverse", Order: []int{0, 2, 1, 3}}}, Fallback: "schedule_forward"}}}
}

func TestSourceOrderAndRename(t *testing.T) {
	p := fixture()
	facts, err := Alternatives(p, "order")
	if err != nil || facts[0] == facts[1] || facts[0][8] != 1 || facts[0][28] != 3 {
		t.Fatal("ordered operations were lost", err, facts)
	}
	p.Base.Statements[1], p.Base.Statements[2] = p.Base.Statements[2], p.Base.Statements[1]
	reversed, err := Alternatives(p, "order")
	if err != nil || reversed[0] != facts[1] || reversed[1] != facts[0] {
		t.Fatal("source reversal did not reverse alternative semantics", err)
	}
	for i := range p.Base.Statements {
		if p.Base.Statements[i].Name == "v" {
			p.Base.Statements[i].Name = "renamed"
		}
	}
	p.Base.Expressions[1].Name = "renamed"
	p.Base.ID, p.Base.Name, p.Decisions[0].Intent = "other", "Renamed", "곱한 뒤 더한다"
	renamed, err := Alternatives(p, "order")
	if err != nil || renamed != reversed {
		t.Fatal("display or instruction text changed source facts", err)
	}
}

func TestOperandsConstantsAndRepeatedOperations(t *testing.T) {
	p := fixture()
	before, _ := Alternatives(p, "order")
	p.Base.Expressions[3].Left, p.Base.Expressions[3].Right = 2, 1
	after, err := Alternatives(p, "order")
	if err != nil || before != after {
		t.Fatal("commutative operand swap changed facts", err)
	}
	p.Base.Expressions[3].Operation = "subtract"
	a, _ := Alternatives(p, "order")
	p.Base.Expressions[3].Left, p.Base.Expressions[3].Right = 1, 2
	b, _ := Alternatives(p, "order")
	if a == b {
		t.Fatal("noncommutative operand swap was lost")
	}
	p.Base.Expressions[5].Operation = "subtract"
	c, _ := Alternatives(p, "order")
	if c[0] == c[1] {
		t.Fatal("different constants with the same opcode were collapsed")
	}
	p.Base.Expressions[2].Int, p.Base.Expressions[4].Int = math.MinInt64, math.MaxInt64
	boundary, err := Alternatives(p, "order")
	if err != nil || boundary[0] == boundary[1] {
		t.Fatal("int64 endpoints lost", err)
	}
	p.Base.Expressions[4].Int = math.MinInt64
	same, err := Alternatives(p, "order")
	if err != nil || same[0] != same[1] {
		t.Fatal("identical operations should have identical descriptors", err)
	}
}

func TestUnsupportedAndBoundsAreAtomic(t *testing.T) {
	for _, mutate := range []func(*bodyplan.Plan, *[]int){
		func(p *bodyplan.Plan, o *[]int) { *o = []int{0, 1, 1, 3} },
		func(p *bodyplan.Plan, o *[]int) { (*o)[1] = -1 },
		func(p *bodyplan.Plan, o *[]int) { (*o)[1] = 999 },
		func(p *bodyplan.Plan, o *[]int) { p.Root[0] = 999 },
		func(p *bodyplan.Plan, o *[]int) { p.Statements[1].Expr = 999 },
		func(p *bodyplan.Plan, o *[]int) { p.Expressions[5].Left = 5 },
		func(p *bodyplan.Plan, o *[]int) { p.Expressions[5].Right = -1 },
		func(p *bodyplan.Plan, o *[]int) { p.Expressions[5].Left = 3 },
		func(p *bodyplan.Plan, o *[]int) { p.Expressions[5].Operation = "equal" },
		func(p *bodyplan.Plan, o *[]int) { p.Expressions[5].Kind = "hole" },
		func(p *bodyplan.Plan, o *[]int) { p.Statements[1].Kind = "if" },
		func(p *bodyplan.Plan, o *[]int) { p.Statements[1].Name = "other" },
		func(p *bodyplan.Plan, o *[]int) { p.Statements[1].Then = []int{2} },
		func(p *bodyplan.Plan, o *[]int) { p.Expressions = make([]bodyplan.Expr, 129) },
	} {
		p := fixture().Base
		order := append([]int(nil), p.Root...)
		mutate(&p, &order)
		out := Signature{255}
		if Encode(p, order, &out) || out != (Signature{255}) {
			t.Fatal("unsupported input wrote partial facts")
		}
	}
	p := fixture()
	if Encode(p.Base, p.Base.Root, nil) {
		t.Fatal("nil output accepted")
	}
	p.Decisions[0].Options[1].Order = []int{0, 1, 1, 3}
	if out, err := Alternatives(p, "order"); err == nil || out != ([2]Signature{}) {
		t.Fatal("invalid typed option accepted or partial alternatives returned")
	}
}

func BenchmarkEncode(b *testing.B) {
	p := fixture().Base
	var out Signature
	b.ReportAllocs()
	for b.Loop() {
		if !Encode(p, p.Root, &out) {
			b.Fatal("unsupported fixture")
		}
	}
}
