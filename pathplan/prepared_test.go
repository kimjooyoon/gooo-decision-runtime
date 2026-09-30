package pathplan

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestPreparedSnapshotKeepsCombinedChecksAndCallerOwnership(t *testing.T) {
	plan := interactingPlan()
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(plan)
	if prepared.PlanSHA256() != hash(raw) || prepared.ActivityName() != "Combined" {
		t.Fatal("prepared identity differs from the declared plan")
	}
	defaults := prepared.Defaults()
	defaults["reference"] = "undeclared"
	plan.Base.Expressions[1].Int = 999
	plan.Base.Root[0] = 3
	plan.Decisions[0].Options[1].Name = "unbound"
	plan.Decisions[1].Options[0].Order[0] = 3
	plan.Decisions[0].Intent = "changed"
	plan.Base.Name = "Changed"
	if value, err := prepared.Fallback().Evaluate(3); err != nil || value.Int != 16 {
		t.Fatal("caller mutation changed cached fallback")
	}
	if prepared.ActivityName() != "Combined" || prepared.Defaults()["reference"] != "reference_first" {
		t.Fatal("prepared snapshot leaked caller-owned state")
	}
	if _, err := prepared.Compile(map[string]string{"reference": "reference_second", "order": "schedule_reverse"}); err == nil {
		t.Fatal("combined scope violation escaped the compiler")
	}
	if _, err := prepared.Compile(defaults); err == nil {
		t.Fatal("undeclared choice escaped the closed alternatives")
	}
}

func TestPreparedSearchRetainsExactLegacySemantics(t *testing.T) {
	plan := interactingPlan()
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, expected := range []int64{16, 17, 999} {
		cases := []TestCase{{Input: 3, Expected: expected}}
		before, oldBody, err := Search(ctx, plan, nil, cases, 4, "")
		if err != nil {
			t.Fatal(err)
		}
		after, newBody, err := prepared.Search(ctx, nil, cases, 4, "")
		if err != nil || !reflect.DeepEqual(before, after) || oldBody.GoSource() != newBody.GoSource() {
			t.Fatalf("prepared search changed finite outcomes: %v", err)
		}
	}
	ctxCanceled, stop := context.WithTimeout(context.Background(), time.Second)
	stop()
	if result, _, err := prepared.Search(ctxCanceled, nil, []TestCase{{Input: 3, Expected: 17}}, 4, ""); !errors.Is(err, context.Canceled) || result.Selection.ModelCalls != 0 {
		t.Fatal("canceled search did work")
	}
	var missing *PreparedPlan
	if _, _, err := missing.Search(ctx, nil, []TestCase{{Input: 3, Expected: 17}}, 4, ""); err == nil {
		t.Fatal("nil prepared plan accepted")
	}
	if _, err := missing.Compile(prepared.Defaults()); err == nil {
		t.Fatal("nil prepared compilation accepted")
	}
}

func TestPreparedConcurrentSearchesOwnWorkState(t *testing.T) {
	prepared, err := Prepare(interactingPlan())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			result, program, err := prepared.Search(ctx, nil, []TestCase{{Input: 3, Expected: 17}}, 4, "")
			if err != nil || result.SelectedTrainingPassed != 1 || program == nil {
				t.Errorf("concurrent search: %v", err)
				return
			}
			result.Selection.Choices["reference"] = "caller mutation"
		}()
	}
	workers.Wait()
	if prepared.Defaults()["reference"] != "reference_first" {
		t.Fatal("search mutated shared defaults")
	}
}

func TestPreparationBoundsBeforeNestedCopies(t *testing.T) {
	for _, field := range []string{"allowed", "branch", "options", "order", "root", "intent"} {
		plan := interactingPlan()
		switch field {
		case "allowed":
			plan.Base.Expressions[0].Allowed = make([]string, 9)
		case "branch":
			plan.Base.Statements[0].Then = make([]int, 129)
		case "options":
			plan.Decisions[0].Options = make([]Option, 3)
		case "order":
			plan.Decisions[1].Options[0].Order = make([]int, 129)
		case "root":
			plan.Base.Root = make([]int, 129)
		case "intent":
			plan.Decisions[0].Intent = string(make([]byte, 513))
		}
		if _, err := Prepare(plan); err == nil {
			t.Fatalf("unbounded %s accepted", field)
		}
	}
}

// This isolates the native adapter's previous four preparation calls. It is
// not an end-to-end compiler or model latency measurement.
func BenchmarkSourceBindingPreparation(b *testing.B) {
	plan := interactingPlan()
	cases := []TestCase{{Input: 3, Expected: 999}}
	for _, arm := range []string{"repeated", "snapshot"} {
		b.Run(arm, func(b *testing.B) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if arm == "snapshot" {
					prepared, err := Prepare(plan)
					if err != nil {
						b.Fatal(err)
					}
					if _, _, err := prepared.Search(ctx, nil, cases, 4, ""); err != nil {
						b.Fatal(err)
					}
				} else {
					if _, err := Validate(plan); err != nil {
						b.Fatal(err)
					}
					defaults, err := Validate(plan)
					if err != nil {
						b.Fatal(err)
					}
					if _, err := Compile(plan, defaults); err != nil {
						b.Fatal(err)
					}
					if _, _, err := Search(ctx, plan, nil, cases, 4, ""); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
