package pathplan

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

func TestConditionInputBindsSourceIntentAndActualCandidate(t *testing.T) {
	plan := conditionContractPlan()
	plan.Decisions[0].Intent = strings.Repeat("한글 조건 ", 30)
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := prepared.InitialConditionInput()
	if err != nil || initial.PlanSHA256() != prepared.PlanSHA256() {
		t.Fatal(initial, err)
	}
	if _, ok := initial.Failure(); ok {
		t.Fatal("initial input has invented observation")
	}
	choices := prepared.Defaults()
	choices["branches"] = "layout_reverse"
	observed, err := prepared.ObserveConditionInput(context.Background(), choices)
	if err != nil {
		t.Fatal(err)
	}
	failure, ok := observed.Failure()
	if !ok || failure.Mask != 2 || failure.Result.Status != "MISMATCH" ||
		failure.Result.Case.Input != -9007199254740995 || failure.Result.Case.Expected ||
		!failure.Result.Observation.Reached || !failure.Result.Observation.Value {
		t.Fatal("candidate mask or declared observation changed", failure)
	}
	// All caller data and returned observations are detached from the input.
	plan.Decisions[0].Intent = "changed"
	plan.ConditionCases[0].Expected = true
	choices["comparison"] = "layout_reverse"
	failure.Result.Case.Input = 1
	for i, id := range []string{"comparison", "branches"} {
		var before, after, want [decision.FeatureDim]float32
		if err := initial.FeaturesInto(id, &before); err != nil {
			t.Fatal(err)
		}
		if err := observed.FeaturesInto(id, &after); err != nil {
			t.Fatal(err)
		}
		source, _ := prepared.SourceFeatures(id)
		err := decision.ConditionFeaturesInto(source, prepared.plan.Decisions[i].Intent, decision.ConditionFeedback{
			Present: true, ChoiceCount: 2, Choice: uint8(i), ObservedChoice: 0, CandidateMask: 2,
			Input: -9007199254740995, Expected: false, Reached: true, Actual: true,
		}, &want)
		if err != nil || after != want || !reflect.DeepEqual(before[:192], after[:192]) {
			t.Fatal("authored inputs or feedback binding differ", id, err)
		}
		if n := testing.AllocsPerRun(20, func() {
			if err := observed.FeaturesInto(id, &after); err != nil {
				panic(err)
			}
		}); n != 0 {
			t.Fatal("reusing input allocates", n)
		}
	}
	matched, err := prepared.ObserveConditionInput(context.Background(), choices)
	if err != nil {
		t.Fatal(err)
	}
	if _, failed := matched.Failure(); failed {
		t.Fatal("passing condition became a counterexample")
	}
}

func TestConditionInputKeepsFirstFailureAndExactInputs(t *testing.T) {
	for _, value := range []int64{math.MinInt64, math.MaxInt64, 9007199254740993,
		-9007199254740995, 9007199254740995, 18014398509481990} {
		plan := conditionContractPlan()
		plan.ConditionCases = []ConditionCase{{"comparison", value, value >= 0}, {"comparison", 0, true}}
		prepared, err := Prepare(plan)
		if err != nil {
			t.Fatal(err)
		}
		input, err := prepared.ObserveConditionInput(context.Background(), prepared.Defaults())
		if err != nil {
			t.Fatal(err)
		}
		failure, ok := input.Failure()
		if !ok || failure.Result.Case.Input != value || failure.Result.Output.Int != value {
			t.Fatal("first exact failure lost", value, failure)
		}
		var features [decision.FeatureDim]float32
		if err := input.FeaturesInto("comparison", &features); err != nil {
			t.Fatal(err)
		}
		var bits uint64
		for _, cell := range features[209:217] {
			bits = bits<<8 | uint64(cell*2048)
		}
		if int64(bits) != value {
			t.Fatal("encoded integer changed", int64(bits), value)
		}
	}
}

func TestConditionInputUnreachedIsUnknown(t *testing.T) {
	plan := conditionObservationPlan()
	plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "int", Int: -10},
		bodyplan.Expr{Kind: "binary", Operation: "less_than", Left: 0, Right: 5})
	plan.Base.Statements[2].Then = []int{3}
	plan.Base.Statements = append(plan.Base.Statements,
		bodyplan.Stmt{Kind: "if", Expr: 6, Then: []int{0}}, bodyplan.Stmt{Kind: "return", Expr: 0})
	plan.Base.Root = []int{2, 4}
	plan.Decisions = append(plan.Decisions, Choice{ID: "nested", Kind: BranchLayout, Target: 3, Intent: "안쪽 조건 관측",
		Fallback: "layout_forward", Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}})
	plan.ConditionCases = []ConditionCase{{"nested", 1, false}}
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	input, err := prepared.ObserveConditionInput(context.Background(), prepared.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	var features [decision.FeatureDim]float32
	if err := input.FeaturesInto("nested", &features); err != nil {
		t.Fatal(err)
	}
	failure, _ := input.Failure()
	if failure.Result.Status != "NOT_REACHED" || features[199] != 0.125 || features[200] != 0 || features[201] != 0 {
		t.Fatal("unreached branch became false", failure, features[199:202])
	}
}

func TestConditionInputRejectsInvalidSelectionAndPreservesDestination(t *testing.T) {
	var missing *PreparedPlan
	if _, err := missing.InitialConditionInput(); err == nil {
		t.Fatal("missing plan accepted")
	}
	prepared, _ := Prepare(interactingPlan())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := prepared.ObserveConditionInput(ctx, prepared.Defaults()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := prepared.ObserveConditionInput(nil, prepared.Defaults()); err == nil {
		t.Fatal("nil context accepted")
	}
	if _, err := prepared.ObserveConditionInput(context.Background(), nil); err == nil {
		t.Fatal("incomplete selection accepted")
	}
	// Every individual option is valid, but their interaction is not.
	bad := map[string]string{"reference": "reference_second", "order": "schedule_reverse"}
	if _, err := prepared.ObserveConditionInput(context.Background(), bad); err == nil || !strings.Contains(err.Error(), "not in scope") {
		t.Fatal("combined invalid selection accepted without declared conditions")
	}
	if accepted, err := prepared.ObserveConditionInput(context.Background(), prepared.Defaults()); err != nil || accepted.present {
		t.Fatal("valid no-condition program did not retain empty feedback", err)
	}
	input, _ := prepared.InitialConditionInput()
	var output [decision.FeatureDim]float32
	output[0] = 17
	want := output
	if err := input.FeaturesInto("absent", &output); err == nil || output != want {
		t.Fatal("invalid destination was changed", err)
	}
	if err := input.FeaturesInto("reference", nil); err == nil {
		t.Fatal("nil destination accepted")
	}
	for _, input := range []*ConditionInput{nil, {}} {
		if input.PlanSHA256() != "" {
			t.Fatal("empty identity fabricated")
		}
		if _, ok := input.Failure(); ok {
			t.Fatal("empty failure fabricated")
		}
		if err := input.FeaturesInto("reference", &output); err == nil || output != want {
			t.Fatal(err)
		}
	}
}

func TestConditionInputConcurrentReuse(t *testing.T) {
	prepared, _ := Prepare(conditionContractPlan())
	input, err := prepared.ObserveConditionInput(context.Background(), prepared.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	var want [decision.FeatureDim]float32
	if err := input.FeaturesInto("comparison", &want); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			for range 16 {
				var got [decision.FeatureDim]float32
				if err := input.FeaturesInto("comparison", &got); err != nil || got != want {
					t.Error("concurrent feature mismatch", err)
				}
			}
		})
	}
	workers.Wait()
}
