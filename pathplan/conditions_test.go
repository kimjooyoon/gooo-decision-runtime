package pathplan

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

func conditionObservationPlan() Plan {
	plan := branchProbePlan()
	plan.Decisions = append(plan.Decisions,
		Choice{ID: "comparison", Kind: OperandOrder, Target: 2, Intent: "Check whether input is negative.",
			Fallback: "layout_forward", Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}},
		Choice{ID: "branches", Kind: BranchLayout, Target: 2, Intent: "Choose branches.",
			Fallback: "layout_forward", Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}})
	return plan
}

func TestCheckConditionsBindsChoiceAndReportsMatchSeparatelyFromOutput(t *testing.T) {
	plan := conditionObservationPlan()
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	// Preparing owns the actual arena and choice-to-expression binding.
	plan.Decisions[2].Target = 3
	plan.Base.Expressions[2].Operation = "equal"
	cases := []ConditionCase{{"comparison", -9007199254740995, true}, {"branches", 0, false},
		{"comparison", 9007199254740995, false}}
	for _, reverse := range []bool{false, true} {
		choices := p.Defaults()
		if reverse {
			choices["comparison"] = "layout_reverse"
		}
		results, err := p.CheckConditions(context.Background(), choices, cases)
		if err != nil || len(results) != len(cases) {
			t.Fatal(results, err)
		}
		for i, result := range results {
			want := "MATCH"
			if reverse && i != 1 {
				want = "MISMATCH"
			}
			if result.Status != want || result.Passed != (want == "MATCH") || result.Observation.Expression != 2 ||
				result.Output.Int != cases[i].Input || !result.Observation.Reached {
				t.Fatal("final outputs match even when intermediate predicate is wrong", result)
			}
		}
		raw, _ := json.Marshal(results)
		if !strings.Contains(string(raw), `9007199254740995`) || !strings.Contains(string(raw), `"expected":false`) {
			t.Fatal("integer precision or explicit false was lost", string(raw))
		}
	}
}

func TestCheckConditionsRejectsNonConditionAndAmbiguousExpression(t *testing.T) {
	plan := conditionObservationPlan()
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := p.CheckConditions(context.Background(), p.Defaults(), []ConditionCase{{"negative", -1, false}}); err == nil || rows != nil {
		t.Fatal("arithmetic expression accepted as an if condition", rows, err)
	}
	// Two distinct if statements read the same arena expression. The stable
	// branch choice can identify one, while the operand choice is ambiguous.
	plan.Base.Statements = append(plan.Base.Statements, bodyplan.Stmt{Kind: "if", Expr: 2})
	plan.Base.Root = []int{3, 2}
	p, err = Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := p.CheckConditions(context.Background(), p.Defaults(), []ConditionCase{{"comparison", -1, true}}); err == nil || rows != nil {
		t.Fatal("ambiguous if identity accepted", rows, err)
	}
	rows, err := p.CheckConditions(context.Background(), p.Defaults(), []ConditionCase{{"branches", -1, true}})
	if err != nil || !rows[0].Passed || rows[0].Observation.Statement != 2 {
		t.Fatal(rows, err)
	}
}

func TestCheckConditionsSkippedConditionRemainsUnknown(t *testing.T) {
	plan := conditionObservationPlan()
	plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "int", Int: -10},
		bodyplan.Expr{Kind: "binary", Operation: "less_than", Left: 0, Right: 5})
	plan.Base.Statements[2].Then = []int{3}
	plan.Base.Statements = append(plan.Base.Statements,
		bodyplan.Stmt{Kind: "if", Expr: 6, Then: []int{0}}, bodyplan.Stmt{Kind: "return", Expr: 0})
	plan.Base.Root = []int{2, 4}
	plan.Decisions = append(plan.Decisions, Choice{ID: "nested", Kind: BranchLayout, Target: 3, Intent: "Observe nested condition.",
		Fallback: "layout_forward", Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}})
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := p.CheckConditions(context.Background(), p.Defaults(), []ConditionCase{{"nested", 1, false}})
	if err != nil || rows[0].Status != "NOT_REACHED" || rows[0].Passed || rows[0].Observation.Reached {
		t.Fatal(rows, err)
	}
}

func TestCheckConditionsRejectsBoundsCancellationAndUnsupportedChoices(t *testing.T) {
	p, _ := Prepare(interactingPlan())
	valid := []ConditionCase{{"reference", 1, false}}
	for _, cases := range [][]ConditionCase{nil, make([]ConditionCase, 129), valid, {{"unknown", 1, false}}} {
		if rows, err := p.CheckConditions(context.Background(), p.Defaults(), cases); err == nil || rows != nil {
			t.Fatal("unsupported observation accepted", rows, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, ctx} {
		if rows, err := p.CheckConditions(ctx, p.Defaults(), valid); err == nil || rows != nil {
			t.Fatal("invalid context accepted", rows, err)
		}
	}
	var absent *PreparedPlan
	if rows, err := absent.CheckConditions(context.Background(), nil, valid); err == nil || rows != nil {
		t.Fatal("missing prepared plan accepted", rows, err)
	}
	if rows, err := p.CheckConditions(context.Background(), nil, valid); err == nil || rows != nil {
		t.Fatal("incomplete selection accepted", rows, err)
	}
}

func TestConditionCaseJSONRequiresExplicitBooleanAndExactInteger(t *testing.T) {
	for _, raw := range []string{
		`{"choice_id":"comparison","input":1}`,
		`{"choice_id":"comparison","input":1,"expected":null}`,
		`{"choice_id":"comparison","input":1,"expected":false,"expected":true}`,
		`{"choice_id":"comparison","input":1,"expected":"false"}`,
		`{"choice_id":"comparison","input":1.5,"expected":false}`,
		`{"choice_id":"comparison","input":9223372036854775808,"expected":false}`,
		`{"choice_id":"comparison","input":1,"expected":false,"extra":0}`,
	} {
		var test ConditionCase
		if json.Unmarshal([]byte(raw), &test) == nil {
			t.Fatal("invalid condition case accepted", raw)
		}
	}
	var test ConditionCase
	if err := json.Unmarshal([]byte(`{"choice_id":"comparison","input":9007199254740995,"expected":false}`), &test); err != nil || test.Input != 9007199254740995 || test.Expected {
		t.Fatal(test, err)
	}
}
