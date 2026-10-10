package bodyplan

import (
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func predicateObservationPlan() Plan {
	return Plan{Schema: Schema, ID: "predicate-observation", Name: "Choose", ResultType: decision.TypeInt,
		Expressions: []Expr{
			{Kind: ExprInput, Name: "input"}, {Kind: ExprInt},
			{Kind: ExprBinary, Operation: "less_than", Left: 0, Right: 1},
			{Kind: ExprBinary, Operation: "subtract", Left: 1, Right: 0},
		},
		Statements: []Stmt{{Kind: StmtIf, Expr: 2, Then: []int{1}, Else: []int{2}},
			{Kind: StmtReturn, Expr: 3}, {Kind: StmtReturn, Expr: 0}}, Root: []int{0},
	}
}

func TestObserveConditionDistinguishesSameOutputOppositePredicate(t *testing.T) {
	original := predicateObservationPlan()
	reversed := predicateObservationPlan()
	reversed.Expressions[2].Left, reversed.Expressions[2].Right = 1, 0
	reversed.Statements[0].Then, reversed.Statements[0].Else = []int{2}, []int{1}
	for _, input := range []int64{-9007199254740995, -1, 0, 1, 9007199254740995} {
		var observations [2]ConditionObservation
		var results [2]Value
		for i, plan := range []Plan{original, reversed} {
			program, err := Compile(plan, nil)
			if err != nil {
				t.Fatal(err)
			}
			results[i], observations[i], err = program.ObserveCondition(input, 0)
			if err != nil || !observations[i].Reached {
				t.Fatalf("input %d: %+v, %v", input, observations[i], err)
			}
			plain, err := program.Evaluate(input)
			if err != nil || plain != results[i] {
				t.Fatal("observation changed the result", err)
			}
		}
		want := input
		if want < 0 {
			want = -want
		}
		if results[0] != results[1] || results[0].Int != want ||
			observations[0].Value != (input < 0) || observations[1].Value != (0 < input) {
			t.Fatalf("input %d: results %+v, observations %+v", input, results, observations)
		}
	}
}

func TestObserveConditionUsesActualReachability(t *testing.T) {
	plan := predicateObservationPlan()
	plan.Expressions = append(plan.Expressions, Expr{Kind: ExprInt, Int: -10},
		Expr{Kind: ExprBinary, Operation: "less_than", Left: 0, Right: 4})
	plan.Statements[0].Then = []int{3}
	plan.Statements = append(plan.Statements, Stmt{Kind: StmtIf, Expr: 5, Then: []int{1}}, Stmt{Kind: StmtReturn, Expr: 0})
	plan.Root = []int{0, 4}
	program, err := Compile(plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []int64{-11, -5, 5} {
		result, got, err := program.ObserveCondition(input, 3)
		plain, plainErr := program.Evaluate(input)
		if err != nil || plainErr != nil || result != plain || got.Reached != (input < 0) || got.Value != (input < -10) {
			t.Fatal("nested condition reachability changed", input, result, got, err, plainErr)
		}
	}
}

func TestObserveConditionUsesLiveLocalAndConcurrentState(t *testing.T) {
	plan := Plan{Schema: Schema, ID: "live-local", Name: "Advance", ResultType: decision.TypeInt,
		Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprLocal, Name: "x"}, {Kind: ExprInt, Int: 1},
			{Kind: ExprBinary, Operation: "add", Left: 1, Right: 2}, {Kind: ExprInt},
			{Kind: ExprBinary, Operation: "less_than", Left: 1, Right: 4}},
		Statements: []Stmt{{Kind: StmtLet, Name: "x", Expr: 0}, {Kind: StmtAssign, Name: "x", Expr: 3},
			{Kind: StmtIf, Expr: 5, Then: []int{3}, Else: []int{4}},
			{Kind: StmtReturn, Expr: 1}, {Kind: StmtReturn, Expr: 4}}, Root: []int{0, 1, 2}}
	program, err := Compile(plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for input := int64(-8); input < 8; input++ {
		group.Go(func() {
			result, got, err := program.ObserveCondition(input, 2)
			plain, plainErr := program.Evaluate(input)
			if err != nil || plainErr != nil || result != plain || !got.Reached || got.Value != (input+1 < 0) {
				t.Errorf("live local or concurrent state changed: %d %+v %+v %v", input, result, got, err)
			}
		})
	}
	group.Wait()
}

func TestObserveConditionRetainsShortCircuitExpressionSemantics(t *testing.T) {
	for _, operation := range []string{"and", "or"} {
		for _, first := range []bool{false, true} {
			plan := predicateObservationPlan()
			plan.Expressions = append(plan.Expressions, Expr{Kind: ExprBool, Bool: first},
				Expr{Kind: ExprBinary, Operation: operation, Left: 4, Right: 2})
			plan.Statements[0].Expr = 5
			program, err := Compile(plan, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, input := range []int64{-1, 1} {
				result, got, err := program.ObserveCondition(input, 0)
				plain, plainErr := program.Evaluate(input)
				expected := first && input < 0
				if operation == "or" {
					expected = first || input < 0
				}
				if err != nil || plainErr != nil || result != plain || !got.Reached || got.Value != expected {
					t.Fatal(operation, first, input, result, got, err, plainErr)
				}
			}
		}
	}
}

func TestObserveConditionRejectsInvalidIndexAndUncompiledProgram(t *testing.T) {
	program, err := Compile(predicateObservationPlan(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{-1, 1, 4, 999} {
		result, observation, err := program.ObserveCondition(0, index)
		if err == nil || result != (Value{}) || observation != (ConditionObservation{}) {
			t.Fatal("invalid index produced an observation", index, result, observation, err)
		}
	}
	if _, _, err := (Program{}).ObserveCondition(0, 0); err == nil {
		t.Fatal("uncompiled program produced an observation")
	}
}

func BenchmarkObserveCondition(b *testing.B) {
	program, err := Compile(predicateObservationPlan(), nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value, _, err := program.ObserveCondition(int64(i%128-64), 0)
		if err != nil {
			b.Fatal(err)
		}
		evaluateBenchmarkSink = value
	}
}
