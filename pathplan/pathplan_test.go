package pathplan

import (
	"strings"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

func interactingPlan() Plan {
	base := bodyplan.Plan{Schema: bodyplan.Schema, ID: "combined-scope", Name: "Combined", ResultType: decision.TypeInt,
		Expressions: []bodyplan.Expr{
			{Kind: "input", Name: "input"}, {Kind: "int", Int: 2}, {Kind: "binary", Operation: "add", Left: 0, Right: 1}, {Kind: "binary", Operation: "multiply", Left: 0, Right: 1},
			{Kind: "local", Name: "first"}, {Kind: "local", Name: "chosen"}, {Kind: "local", Name: "first"}, {Kind: "local", Name: "second"},
			{Kind: "binary", Operation: "add", Left: 6, Right: 7}, {Kind: "binary", Operation: "add", Left: 5, Right: 8},
		},
		Statements: []bodyplan.Stmt{{Kind: "let", Name: "first", Expr: 2}, {Kind: "let", Name: "second", Expr: 3}, {Kind: "let", Name: "chosen", Expr: 4}, {Kind: "return", Expr: 9}}, Root: []int{0, 1, 2, 3},
	}
	return Plan{Schema: Schema, Base: base, Decisions: []Choice{
		{ID: "reference", Kind: LocalReference, Target: 4, Intent: "Choose a local.", Fallback: "reference_first", Options: []Option{{Label: "reference_first", Name: "first"}, {Label: "reference_second", Name: "second"}}},
		{ID: "order", Kind: RootOrder, Target: 0, Intent: "Choose declaration order.", Fallback: "schedule_forward", Options: []Option{{Label: "schedule_forward", Order: []int{0, 1, 2, 3}}, {Label: "schedule_reverse", Order: []int{0, 2, 1, 3}}}},
	}}
}

func TestCombinedSelectionsRecheckScopeAndOwnTheirArrays(t *testing.T) {
	plan := interactingPlan()
	if _, err := Validate(plan); err != nil {
		t.Fatal(err)
	}
	if _, err := Compile(plan, map[string]string{"reference": "reference_second", "order": "schedule_reverse"}); err == nil || !strings.Contains(err.Error(), "not in scope") {
		t.Fatalf("interacting scope failure not retained: %v", err)
	}
	for _, choices := range []map[string]string{
		{"reference": "reference_first", "order": "schedule_forward"},
		{"reference": "reference_second", "order": "schedule_forward"},
		{"reference": "reference_first", "order": "schedule_reverse"},
	} {
		program, err := Compile(plan, choices)
		if err != nil {
			t.Fatal(err)
		}
		value, err := program.Evaluate(3)
		if err != nil {
			t.Fatal(err)
		}
		want := int64(16)
		if choices["reference"] == "reference_second" {
			want = 17
		}
		if value.Int != want {
			t.Fatalf("combined program: %d want %d", value.Int, want)
		}
	}
	program, err := Compile(plan, map[string]string{"reference": "reference_second", "order": "schedule_forward"})
	if err != nil {
		t.Fatal(err)
	}
	plan.Base.Expressions[1].Int = 999
	plan.Base.Root[0] = 3
	plan.Decisions[0].Options[1].Name = "unbound"
	if value, err := program.Evaluate(3); err != nil || value.Int != 17 {
		t.Fatal("caller mutation changed compiled program")
	}
}

func TestDuplicateTargetsAndSelectionsReject(t *testing.T) {
	plan := interactingPlan()
	plan.Decisions = append(plan.Decisions, plan.Decisions[0])
	plan.Decisions[2].ID = "duplicate"
	if _, err := Validate(plan); err == nil {
		t.Fatal("duplicate structural write accepted")
	}
	plan = interactingPlan()
	if _, err := Compile(plan, map[string]string{"reference": "reference_first", "unknown": "schedule_forward"}); err == nil {
		t.Fatal("unknown/missing selection accepted")
	}
}

func TestSeededDistributionBindsPlanModelAndSeed(t *testing.T) {
	choice := Choice{ID: "x", Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse"}}}
	for _, weights := range [][2]float64{{1, 0}, {0, 1}, {0.3, 0.7}} {
		first, err := sample("plan", choice, "metadata", "weights", "seed", weights)
		if err != nil {
			t.Fatal(err)
		}
		second, err := sample("plan", choice, "metadata", "weights", "seed", weights)
		if err != nil || first != second {
			t.Fatal("seeded choice is not reproducible")
		}
		if weights[0] == 1 && first != "layout_forward" || weights[1] == 1 && first != "layout_reverse" {
			t.Fatal("zero-mass choice sampled")
		}
	}
	if _, err := sample("plan", choice, "metadata", "weights", "seed", [2]float64{}); err == nil {
		t.Fatal("empty probability mass accepted")
	}
}
