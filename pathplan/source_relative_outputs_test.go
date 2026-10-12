package pathplan

import (
	"context"
	"math"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func outputRelationFixture(t *testing.T, cases []TestCase) (*PreparedPlan, *ContractInput) {
	t.Helper()
	plan := orderedArithmeticPlan()
	plan.Base.Expressions = []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 10},
		{Kind: "int", Int: 3}, {Kind: "binary", Operation: "less_than", Left: 0, Right: 1},
		{Kind: "binary", Operation: "divide", Left: 0, Right: 2},
		{Kind: "binary", Operation: "subtract", Left: 1, Right: 0}}
	plan.Base.Statements[0].Expr, plan.Base.Statements[1].Expr, plan.Base.Statements[2].Expr = 4, 5, 3
	plan.Decisions[0].Target = 3
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	input, err := p.InitialContractInput(cases)
	if err != nil {
		t.Fatal(err)
	}
	return p, input
}

func TestSourceRelativeOutputsExposeGoalsPreserveV2AndAllCases(t *testing.T) {
	cases := []TestCase{{-13, -4}, {10, 0}, {27, -17}, {9007199254740993, -9007199254740983}}
	p, input := outputRelationFixture(t, cases)
	id := p.plan.Decisions[0].ID
	view, err := input.SourceRelativeOutputs(id)
	if err != nil || !view.Available || !slices.Equal(view.Literals, []int64{3, 10}) || view.CaseCount != 4 ||
		view.PlanSHA256 != p.sha || view.CaseSHA256 != input.CaseSHA256() {
		t.Fatal("source output context", view, err)
	}
	changed := append([]TestCase(nil), cases...)
	changed[0].Expected = 31
	_, other := outputRelationFixture(t, changed)
	var old, oldChanged, next, nextChanged [528]float32
	if input.SourceRelativeV2FeaturesInto(id, &old) != nil || other.SourceRelativeV2FeaturesInto(id, &oldChanged) != nil ||
		input.SourceRelativeV3FeaturesInto(id, &next) != nil || other.SourceRelativeV3FeaturesInto(id, &nextChanged) != nil {
		t.Fatal("feature projection")
	}
	if old != oldChanged || next == nextChanged || !slices.Equal(old[:208], next[:208]) ||
		!slices.Equal(old[256:], next[256:]) {
		t.Fatal("V2 changed or new goal relationship invisible")
	}
	// Output cases already have their own model stream in V2; they were not missing.
	a, _ := input.CaseFeatures(0)
	b, _ := other.CaseFeatures(0)
	if a == b {
		t.Fatal("V2's independent declared output stream lost the changed expectation")
	}
	all := make([]TestCase, 128)
	for index := range all {
		all[index] = TestCase{Input: int64(index) - 64, Expected: int64(index) - 64}
	}
	_, owned := outputRelationFixture(t, all)
	full, _ := owned.SourceRelativeOutputs(id)
	clear(all)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			current, err := owned.SourceRelativeOutputs(id)
			if err != nil || !reflect.DeepEqual(current, full) {
				t.Error("caller mutation or concurrent reader changed the source requirements")
			}
		})
	}
	wg.Wait()
	if full.Counts[0][0]+full.Counts[1][0]+full.Counts[2][0] != 128 {
		t.Fatal("declared output rows were sampled or dropped")
	}
}

func TestSourceRelativeOutputArithmeticWrapDivisionAndZero(t *testing.T) {
	for _, c := range []struct {
		input, expected, literal int64
		slot                     int
	}{
		{-13, -4, 3, 5}, {-13, 0, 3, 6}, {math.MinInt64, math.MinInt64, -1, 5},
		{math.MaxInt64, math.MinInt64, 1, 1}, {math.MinInt64, math.MaxInt64, 1, 2},
		{math.MaxInt64, -2, 2, 4}, {0, 0, 0, 10}, {0, 0, 0, 11},
	} {
		flags := outputLiteralRelations(c.input, c.expected, c.literal)
		if !flags[c.slot] {
			t.Fatal("integer relationship differs", c, flags)
		}
	}
	zero := outputLiteralRelations(7, 0, 0)
	if zero[5] {
		t.Fatal("undefined division became a matching quotient")
	}
	p, input := outputRelationFixture(t, []TestCase{{-13, -4}})
	var out [528]float32
	out[0] = 99
	before := out
	var missing *ContractInput
	if missing.SourceRelativeV3FeaturesInto(p.plan.Decisions[0].ID, &out) == nil || out != before ||
		input.SourceRelativeV3FeaturesInto("absent", &out) == nil || out != before ||
		input.SourceRelativeV3FeaturesInto(p.plan.Decisions[0].ID, nil) == nil {
		t.Fatal("invalid request changed its destination")
	}
}

func TestSourceRelativeV3SessionPredictsOnceAndUsesFiniteOriginalBodies(t *testing.T) {
	cases := []TestCase{{-13, -4}, {10, 0}, {27, -17}}
	p, input := outputRelationFixture(t, cases)
	model, err := contractdecision.NewSourceRelativeV3([contractdecision.InteractionRequirementParameterCount]float32{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := model.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	model, err = contractdecision.DecodeSourceRelativeV3(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := contractdecision.DecodeSourceRelativeV2(raw); err == nil {
		t.Fatal("V3 artifact entered the V2 decoder")
	}
	ctx := sessionContext(t)
	session, err := p.NewSourceRelativeV3ContractSession(ctx, model, cases)
	if err != nil || session.Ranking().Calls != 1 || session.Ranking().Schema != "gooo/source-relative-requirement-path-ranking/v3" {
		t.Fatal("initial source-bound prediction", err)
	}
	initial, err := session.Observe()
	if err != nil || initial.Attempted != 0 {
		t.Fatal("prediction executed a candidate")
	}
	progress, body, err := session.Advance(ctx, 4)
	if err != nil || body == nil || progress.Status != "TRAINING_COMPLETE" || progress.PredictionsThisAdvance != 0 {
		t.Fatal("finite original-body construction", progress, err)
	}
	for _, c := range cases {
		value, err := body.Evaluate(c.Input)
		if err != nil || value.Int != c.Expected {
			t.Fatal("selected original body differs", c, value, err)
		}
	}
	var features [528]float32
	if input.SourceRelativeV3FeaturesInto(p.plan.Decisions[0].ID, &features) != nil {
		t.Fatal("inspection")
	}
	var work contractdecision.InteractionRequirementWorkspace
	var prediction contractdecision.ChoicePrediction
	prediction.Count = 99
	previous := prediction
	for _, alter := range []func(*[528]float32){
		func(row *[528]float32) { row[208] = float32(math.NaN()) },
		func(row *[528]float32) { clear(row[208:256]) },
		func(row *[528]float32) { row[211] = 1 },
		func(row *[528]float32) { row[215] = 1 },
		func(row *[528]float32) { row[256] = 1 },
	} {
		invalid := features
		alter(&invalid)
		if err := model.PredictChoicesInto([][528]float32{invalid}, input, input, &work, &prediction); err == nil || prediction != previous {
			t.Fatal("invalid V3 output profile changed prediction destination")
		}
	}
	without, err := p.NewSourceRelativeV3ContractSession(ctx, nil, cases)
	if err != nil || without.Ranking().Calls != 0 {
		t.Fatal("deterministic omission", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.NewSourceRelativeV3ContractSession(cancelled, model, cases); err == nil {
		t.Fatal("cancelled search performed a prediction")
	}
}
