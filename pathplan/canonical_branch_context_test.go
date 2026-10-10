package pathplan

import (
	"math"
	"reflect"
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

func canonicalAssignmentPlan() Plan {
	p := orderedArithmeticPlan()
	p.Base.Expressions = append(p.Base.Expressions, bodyplan.Expr{Kind: "local", Name: "result"})
	p.Base.Statements[0] = bodyplan.Stmt{Kind: "assign", Name: "result", Expr: 3}
	p.Base.Statements[1] = bodyplan.Stmt{Kind: "assign", Name: "result", Expr: 4}
	p.Base.Statements = append(p.Base.Statements,
		bodyplan.Stmt{Kind: "let", Name: "result", Expr: 0},
		bodyplan.Stmt{Kind: "return", Expr: 5})
	p.Base.Root = []int{3, 2, 4}
	return p
}

func TestCanonicalBranchInputPreservesSemanticsAcrossAssignments(t *testing.T) {
	for _, literal := range []int64{53, -37, 9007199254740993, math.MinInt64, math.MaxInt64} {
		for fallback := range 4 {
			plans := []Plan{orderedArithmeticPlan(), canonicalAssignmentPlan()}
			var sources [2]*PreparedPlan
			var input [2]*ContractInput
			for i := range plans {
				plans[i].Base.Expressions[1].Int = literal
				for j := range plans[i].Decisions {
					plans[i].Decisions[j].Fallback = plans[i].Decisions[j].Options[fallback>>j&1].Label
				}
				var err error
				sources[i], err = Prepare(plans[i])
				if err != nil {
					t.Fatal(err)
				}
				input[i], err = sources[i].InitialContractInput([]TestCase{{-2, 0}, {literal, 0}})
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, c := range plans[0].Decisions {
				var got [2][528]float32
				var original [2][528]float32
				for i := range sources {
					if err := input[i].OrderedSourceFeaturesInto(c.ID, &original[i]); err != nil {
						t.Fatal(err)
					}
					if err := input[i].CanonicalOrderedSourceFeaturesInto(c.ID, &got[i]); err != nil {
						t.Fatal(err)
					}
					view, err := sources[i].CanonicalOrderedBranchContext(c.ID)
					if err != nil || !view.Available || view.Schema != CanonicalOrderedBranchContextSchema {
						t.Fatal(view, err)
					}
					var suffix [144]float32
					if decision.OrderedExpressionFeaturesInto(view.Expressions, &suffix) != nil || !reflect.DeepEqual(suffix[:], got[i][384:]) {
						t.Fatal("displayed expressions differ from model input")
					}
					if !reflect.DeepEqual(got[i][192:384], make([]float32, 192)) {
						t.Fatal("reserved initial channels populated")
					}
					var after [528]float32
					if input[i].OrderedSourceFeaturesInto(c.ID, &after) != nil || original[i] != after {
						t.Fatal("legacy input changed")
					}
				}
				if got[0] != got[1] || original[0] == original[1] {
					t.Fatal("assignment distinction was not removed only from the new input", literal, fallback, c.ID)
				}
			}
			// Fresh regression fixtures: all candidate choices still execute the
			// original direct/assignment programs, including integer overflow.
			for mask := range 4 {
				choices := map[string]string{}
				for j, c := range plans[0].Decisions {
					choices[c.ID] = c.Options[mask>>j&1].Label
				}
				a, err := sources[0].Compile(choices)
				if err != nil {
					t.Fatal(err)
				}
				b, err := sources[1].Compile(choices)
				if err != nil {
					t.Fatal(err)
				}
				for _, x := range []int64{-9007199254740995, 0, literal, 9007199254740993} {
					av, ae := a.Evaluate(x)
					bv, be := b.Evaluate(x)
					if ae != nil || be != nil || av != bv {
						t.Fatal("candidate semantics changed", x, av, bv, ae, be)
					}
				}
			}
		}
	}
}

func TestCanonicalBranchContextErrorsAndConcurrentSnapshots(t *testing.T) {
	p := canonicalAssignmentPlan()
	prepared, err := Prepare(p)
	if err != nil {
		t.Fatal(err)
	}
	input, err := prepared.InitialContractInput([]TestCase{{-5, 15}})
	if err != nil {
		t.Fatal(err)
	}
	id := p.Decisions[0].ID
	var want [528]float32
	if input.CanonicalOrderedSourceFeaturesInto(id, &want) != nil {
		t.Fatal("canonical input")
	}
	p.Base.Expressions[1].Int = 123
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			var got [528]float32
			if input.CanonicalOrderedSourceFeaturesInto(id, &got) != nil || got != want {
				t.Error("snapshot changed")
			}
		})
	}
	wg.Wait()
	var nilInput *ContractInput
	for _, call := range []func(*[528]float32) error{
		func(out *[528]float32) error { return nilInput.CanonicalOrderedSourceFeaturesInto(id, out) },
		func(out *[528]float32) error { return input.CanonicalOrderedSourceFeaturesInto("missing", out) },
	} {
		got := want
		if call(&got) == nil || got != want {
			t.Fatal("error changed destination")
		}
	}
	if input.CanonicalOrderedSourceFeaturesInto(id, nil) == nil {
		t.Fatal("nil output")
	}
	p = orderedArithmeticPlan()
	p.Base.Expressions[4].Left = 3
	unsupported, err := Prepare(p)
	if err != nil {
		t.Fatal(err)
	}
	u, err := unsupported.InitialContractInput([]TestCase{{1, 0}})
	if err != nil {
		t.Fatal(err)
	}
	got := want
	if u.CanonicalOrderedSourceFeaturesInto(id, &got) == nil || got != want {
		t.Fatal("unsupported source fabricated an input")
	}
	view, err := unsupported.CanonicalOrderedBranchContext(id)
	if err != nil || view.Available || view.Reason == "" || view.Source != ([64]byte{}) {
		t.Fatal("unsupported context", view, err)
	}
}

func TestCanonicalInputIgnoresAtomSharingAndLocalNames(t *testing.T) {
	plans := []Plan{orderedArithmeticPlan(), canonicalAssignmentPlan()}
	p := &plans[1]
	p.Base.Expressions[5].Name = "copied_value"
	for i := range p.Base.Statements {
		if p.Base.Statements[i].Name == "result" {
			p.Base.Statements[i].Name = "copied_value"
		}
	}
	literal, local := p.Base.Expressions[1], p.Base.Expressions[5]
	p.Base.Expressions = append(p.Base.Expressions[:4:4], literal,
		bodyplan.Expr{Kind: "binary", Operation: "subtract", Left: 4, Right: 0}, local)
	p.Base.Statements[1].Expr, p.Base.Statements[4].Expr = 5, 6
	var inputs [2][528]float32
	for i, plan := range plans {
		prepared, err := Prepare(plan)
		if err != nil {
			t.Fatal(err)
		}
		input, err := prepared.InitialContractInput([]TestCase{{-5, 15}})
		if err != nil {
			t.Fatal(err)
		}
		if err := input.CanonicalOrderedSourceFeaturesInto(plan.Decisions[0].ID, &inputs[i]); err != nil {
			t.Fatal(err)
		}
	}
	if inputs[0] != inputs[1] {
		t.Fatal("node sharing or local spelling changed the canonical input")
	}
}
