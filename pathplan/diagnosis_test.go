package pathplan

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

func probePlan(operation string) Plan {
	return Plan{Schema: Schema, Base: bodyplan.Plan{Schema: bodyplan.Schema, ID: "diagnosis", Name: "DiagnoseBody", ResultType: decision.TypeInt,
		Expressions: []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 2}, {Kind: "binary", Operation: operation, Left: 0, Right: 1}},
		Statements:  []bodyplan.Stmt{{Kind: "return", Expr: 2}}, Root: []int{0}},
		Decisions: []Choice{{ID: "operands", Kind: OperandOrder, Target: 2, Intent: "Select operand order.", Fallback: "layout_forward",
			Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}}}}
}

func TestDiagnosisExposesAmbiguityWithoutInventingExpectedOutput(t *testing.T) {
	prepared, err := Prepare(probePlan("subtract"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cases := []TestCase{{Input: 2, Expected: 0}, {Input: 2, Expected: 1}}
	result, err := prepared.Diagnose(ctx, prepared.Defaults(), cases, []int64{2, 3, math.MaxInt64}, 2)
	if err != nil || result.Status != "COMPLETE" || result.Observed != 2 || result.Unobserved != 0 ||
		result.CaseIndistinguishable != 2 || result.ProbeDistinguished != 1 || result.ProbeUnresolved != 1 || result.ModelPredictions != 0 {
		t.Fatalf("diagnosis: %+v, %v", result, err)
	}
	if result.Candidates[0].Passed != 1 || result.Candidates[1].Passed != 1 || result.Candidates[0].Witness != nil ||
		!reflect.DeepEqual(result.Candidates[1].Witness, &DistinguishingInput{Input: 3, Reference: 1, Alternative: -1}) {
		t.Fatal("contradictory cases or first distinguishing input changed")
	}
	partial, err := prepared.Diagnose(ctx, map[string]string{"operands": "layout_reverse"}, cases, []int64{3}, 1)
	if err != nil || partial.ReferenceMask != 1 || partial.Candidates[0].Mask != 1 || partial.Status != "PARTIAL" || partial.Unobserved != 1 || partial.ProbeDistinguished != 0 {
		t.Fatal("reference was not prioritized or unobserved space was presented as resolved")
	}
}

func TestDiagnosisProbeAgreementRemainsUnresolvedAndChecksCombinedScope(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	prepared, _ := Prepare(probePlan("add"))
	result, err := prepared.Diagnose(ctx, prepared.Defaults(), []TestCase{{Input: 2, Expected: 4}}, []int64{0, 3, math.MinInt64}, 2)
	if err != nil || result.ProbeUnresolved != 2 || result.ProbeDistinguished != 0 || result.CaseIndistinguishable != 2 {
		t.Fatal("finite probe agreement was not retained as unresolved")
	}
	prepared, _ = Prepare(interactingPlan())
	result, err = prepared.Diagnose(ctx, prepared.Defaults(), []TestCase{{Input: 3, Expected: 16}}, []int64{2, 3}, 4)
	if err != nil || result.Typed != 3 || result.TypeRejected != 1 || result.Candidates[3].Status != "TYPE_REJECTED" || result.Candidates[3].GoooSHA256 != "" {
		t.Fatal("interacting type/scope rejection became an evaluated candidate")
	}
}

func TestDiagnosisRejectsInvalidBoundsBeforeWork(t *testing.T) {
	prepared, _ := Prepare(probePlan("subtract"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cases, probes := []TestCase{{Input: 2}}, []int64{3}
	for _, invalid := range []string{"nil", "deadline", "canceled", "cases", "probes", "budget", "selection", "prepared"} {
		c, p, tests, inputs, budget, choices := ctx, prepared, cases, probes, 2, prepared.Defaults()
		switch invalid {
		case "nil":
			c = nil
		case "deadline":
			c = context.Background()
		case "canceled":
			stopped, stop := context.WithTimeout(context.Background(), time.Second)
			stop()
			c = stopped
		case "cases":
			tests = make([]TestCase, 129)
		case "probes":
			inputs = make([]int64, 33)
		case "budget":
			budget = 65
		case "selection":
			choices["unknown"] = "layout_forward"
		case "prepared":
			p = nil
		}
		result, err := p.Diagnose(c, choices, tests, inputs, budget)
		if err == nil || result.Observed != 0 || len(result.Candidates) != 0 {
			t.Fatalf("accepted %s", invalid)
		}
		if invalid == "canceled" && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
}

func TestDiagnosisOwnsResultsAcrossConcurrentCalls(t *testing.T) {
	prepared, _ := Prepare(probePlan("subtract"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cases, probes := []TestCase{{Input: 2}}, []int64{3}
	want, err := prepared.Diagnose(ctx, prepared.Defaults(), cases, probes, 2)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			got, err := prepared.Diagnose(ctx, prepared.Defaults(), cases, probes, 2)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("concurrent diagnosis: %v", err)
				return
			}
			got.Candidates[1].Witness.Reference = 999
			got.Candidates[0].CaseOutputsSHA256 = "mutated"
		})
	}
	group.Wait()
	got, err := prepared.Diagnose(ctx, prepared.Defaults(), cases, probes, 2)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("returned result changed shared state")
	}
}

func TestDiagnosisHighestMaskWithPartialBudget(t *testing.T) {
	plan := interactingPlan()
	plan.Decisions = nil
	plan.Base.Expressions = plan.Base.Expressions[:4]
	plan.Base.Statements = plan.Base.Statements[:2]
	plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "local", Name: "first"}, bodyplan.Expr{Kind: "local", Name: "second"},
		bodyplan.Expr{Kind: "binary", Operation: "add", Left: 4, Right: 5})
	root := 6
	var leaves []int
	for i := range 16 {
		index := len(plan.Base.Expressions)
		plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "local", Name: "first"})
		plan.Decisions = append(plan.Decisions, Choice{ID: fmt.Sprintf("pick-%d", i), Kind: LocalReference, Target: index, Intent: "Choose a local.", Fallback: "reference_first",
			Options: []Option{{Label: "reference_first", Name: "first"}, {Label: "reference_second", Name: "second"}}})
		leaves = append(leaves, index)
	}
	for len(leaves) > 1 {
		var next []int
		for i := 0; i < len(leaves); i += 2 {
			plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "binary", Operation: "add", Left: leaves[i], Right: leaves[i+1]})
			next = append(next, len(plan.Base.Expressions)-1)
		}
		leaves = next
	}
	plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "binary", Operation: "add", Left: root, Right: leaves[0]})
	root = len(plan.Base.Expressions) - 1
	plan.Base.Statements = append(plan.Base.Statements, bodyplan.Stmt{Kind: "return", Expr: root})
	plan.Base.Root = []int{0, 1, 2}
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	choices := prepared.Defaults()
	for _, choice := range plan.Decisions {
		choices[choice.ID] = "reference_second"
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := prepared.Diagnose(ctx, choices, []TestCase{{Input: 0}}, []int64{1}, 2)
	if err != nil || result.ReferenceMask != math.MaxUint16 || result.Declared != 65536 || result.Unobserved != 65534 || result.Candidates[1].Mask != 0 {
		t.Fatalf("highest mask: %+v, %v", result, err)
	}
}

type diagnosisCanceledAfterReference struct {
	context.Context
	stop  context.CancelFunc
	calls int
}

func (ctx *diagnosisCanceledAfterReference) Err() error {
	ctx.calls++
	if ctx.calls == 10 {
		ctx.stop()
	}
	return ctx.Context.Err()
}

func TestDiagnosisCancellationRetainsOnlyCompletedCandidates(t *testing.T) {
	prepared, _ := Prepare(probePlan("subtract"))
	base, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ctx := &diagnosisCanceledAfterReference{Context: base, stop: cancel}
	result, err := prepared.Diagnose(ctx, prepared.Defaults(), []TestCase{{Input: 2}}, []int64{3}, 2)
	if !errors.Is(err, context.Canceled) || result.Observed != 1 || len(result.Candidates) != 1 || result.Unobserved != 1 || result.Status != "PARTIAL" {
		t.Fatalf("interrupted receipts: %+v, %v", result, err)
	}
}
