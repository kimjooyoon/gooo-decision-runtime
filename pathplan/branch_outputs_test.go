package pathplan

import (
	"encoding/binary"
	"math"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func branchOutputPlan() Plan {
	p := orderedArithmeticPlan()
	p.Base.Expressions = []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 31},
		{Kind: "int", Int: 5}, {Kind: "binary", Operation: "less_than", Left: 0, Right: 1},
		{Kind: "binary", Operation: "divide", Left: 0, Right: 2},
		{Kind: "binary", Operation: "subtract", Left: 1, Right: 0}}
	p.Base.Statements[0].Expr, p.Base.Statements[1].Expr, p.Base.Statements[2].Expr = 4, 5, 3
	p.Decisions[0].Target = 3
	return p
}
func branchOutputInput(t *testing.T, plan Plan, cases []TestCase) (*PreparedPlan, *ContractInput) {
	t.Helper()
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

func TestBranchOutputsKeepReturnOwnershipAndSourceChannels(t *testing.T) {
	cases := []TestCase{{-17, -3}, {31, 0}, {49, -18}, {9007199254740993, -9007199254740962}}
	p, input := branchOutputInput(t, branchOutputPlan(), cases)
	id := p.plan.Decisions[0].ID
	view, err := input.BranchOutputs(id)
	if err != nil || !view.Available || !slices.Equal(view.Literals[0], []int64{5}) ||
		!slices.Equal(view.Literals[1], []int64{31}) || view.CaseCount != 4 ||
		view.PlanSHA256 != input.PlanSHA256() || view.CaseSHA256 != input.CaseSHA256() {
		t.Fatal("branch ownership", view, err)
	}
	if view.Counts[0][0][9] != 1 || view.Counts[1][0][9] != 0 {
		t.Fatal("input/literal relation mixed between then and else")
	}
	var before, after [528]float32
	if input.SourceRelativeV3ProseNeutralFeaturesInto(id, &before) != nil || input.SourceRelativeV4FeaturesInto(id, &after) != nil {
		t.Fatal("feature preparation")
	}
	if !slices.Equal(before[:256], after[:256]) || !slices.Equal(before[352:], after[352:]) ||
		!slices.Equal(after[256:352], view.Features[:]) {
		t.Fatal("V4 overwrote older source channels")
	}
	swapped := branchOutputPlan()
	swapped.Base.Statements[0].Expr, swapped.Base.Statements[1].Expr = 5, 4
	_, other := branchOutputInput(t, swapped, cases)
	var swappedFeatures [528]float32
	if other.SourceRelativeV4FeaturesInto(id, &swappedFeatures) != nil ||
		!slices.Equal(after[256:304], swappedFeatures[304:352]) ||
		!slices.Equal(after[304:352], swappedFeatures[256:304]) ||
		!slices.Equal(after[208:256], swappedFeatures[208:256]) {
		t.Fatal("return swap did not exchange only the owned relationship blocks")
	}
	changed := append([]TestCase(nil), cases...)
	changed[0].Expected = 19
	_, different := branchOutputInput(t, branchOutputPlan(), changed)
	differentView, _ := different.BranchOutputs(id)
	if differentView.Features == view.Features {
		t.Fatal("changed declared output disappeared")
	}
	before = after
	var missing *ContractInput
	if missing.SourceRelativeV4FeaturesInto(id, &after) == nil || after != before ||
		input.SourceRelativeV4FeaturesInto("absent", &after) == nil || after != before ||
		input.SourceRelativeV4FeaturesInto(id, nil) == nil {
		t.Fatal("invalid input changed destination")
	}
}

func TestBranchOutputsRetainAllRowsAndConcurrentSnapshots(t *testing.T) {
	cases := make([]TestCase, 128)
	for i := range cases {
		cases[i] = TestCase{Input: int64(i) - 64, Expected: int64(i) - 64}
	}
	p, input := branchOutputInput(t, branchOutputPlan(), cases)
	id := p.plan.Decisions[0].ID
	want, err := input.BranchOutputs(id)
	if err != nil {
		t.Fatal(err)
	}
	clear(cases)
	for branch := range 2 {
		if want.Counts[branch][0][0]+want.Counts[branch][1][0]+want.Counts[branch][2][0] != 128 {
			t.Fatal("case rows sampled")
		}
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			got, err := input.BranchOutputs(id)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Error("snapshot changed", err)
			}
		})
	}
	wg.Wait()
	plan := branchOutputPlan()
	plan.Base.Expressions = []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 31},
		{Kind: "binary", Operation: "less_than", Left: 0, Right: 1},
		{Kind: "binary", Operation: "subtract", Left: 1, Right: 0}}
	plan.Base.Statements[0].Expr = 0 // Returning input has no literal owner.
	plan.Base.Statements[1].Expr, plan.Base.Statements[2].Expr = 3, 2
	plan.Decisions[0].Target = 2
	_, plain := branchOutputInput(t, plan, []TestCase{{-1, -1}})
	view, err := plain.BranchOutputs(id)
	if err != nil || !view.Available || len(view.Literals[0]) != 0 {
		t.Fatal("literal-free return", view, err)
	}
	for bucket := range 3 {
		for _, count := range view.Counts[0][bucket][4:] {
			if count != 0 {
				t.Fatal("literal-free return invented a relationship")
			}
		}
	}
}

func TestSourceRelativeV4SessionPredictsOnceBeforeFiniteConstruction(t *testing.T) {
	cases := []TestCase{{-17, -3}, {31, 0}, {49, -18}}
	p, input := branchOutputInput(t, branchOutputPlan(), cases)
	model, err := contractdecision.NewSourceRelativeV4([contractdecision.InteractionRequirementParameterCount]float32{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := sessionContext(t)
	s, err := p.NewSourceRelativeV4ContractSession(ctx, model, cases)
	if err != nil {
		t.Fatal(err)
	}
	ranking := s.Ranking()
	initial, err := s.Observe()
	if err != nil || initial.Attempted != 0 || ranking.Calls != 1 || !ranking.Applied ||
		ranking.SourceFeatures != contractdecision.SourceRelativeV4FeatureVersion || ranking.ModelFingerprint != model.Fingerprint() {
		t.Fatal("V4 initial prediction contract", err)
	}
	for i, choice := range p.plan.Decisions {
		var features [528]float32
		if input.SourceRelativeV4FeaturesInto(choice.ID, &features) != nil {
			t.Fatal("V4 inspection")
		}
		var raw [528 * 4]byte
		for j, value := range features {
			binary.LittleEndian.PutUint32(raw[j*4:], math.Float32bits(value))
		}
		if ranking.FeatureSHA[i] != hash(raw[:]) {
			t.Fatal("ranking receipt differs from actual V4 input")
		}
	}
	progress, body, err := s.Advance(ctx, 4)
	if err != nil || body == nil || progress.Status != "TRAINING_COMPLETE" || progress.PredictionsThisAdvance != 0 || s.Ranking() != ranking {
		t.Fatal("bounded construction", progress, err)
	}
	for _, c := range cases {
		actual, err := body.Evaluate(c.Input)
		if err != nil || actual.Int != c.Expected {
			t.Fatal("source-owned body differs", actual, err)
		}
	}
	ordinary, _ := p.NewContractSession(ctx, nil, cases)
	nilModel, err := p.NewSourceRelativeV4ContractSession(ctx, nil, cases)
	if err != nil || !reflect.DeepEqual(ordinary.Ranking(), nilModel.Ranking()) {
		t.Fatal("nil model changed default", err)
	}
	unsupported := branchOutputPlan()
	unsupported.Base.Expressions[5].Left = 4
	u, _ := branchOutputInput(t, unsupported, cases)
	declined, err := u.NewSourceRelativeV4ContractSession(ctx, model, cases)
	if err != nil || !declined.Ranking().Declined || declined.Ranking().Calls != 0 {
		t.Fatal("unsupported source reached the model", err)
	}
}

func TestBranchOutputsPreserveAssignmentAndFallbackMeaning(t *testing.T) {
	for fallback := range 4 {
		direct, assigned := branchOutputPlan(), branchOutputPlan()
		assigned.Base.Expressions = append(assigned.Base.Expressions, bodyplan.Expr{Kind: "local", Name: "result"})
		assigned.Base.Statements[0] = bodyplan.Stmt{Kind: "assign", Name: "result", Expr: 4}
		assigned.Base.Statements[1] = bodyplan.Stmt{Kind: "assign", Name: "result", Expr: 5}
		assigned.Base.Statements = append(assigned.Base.Statements,
			bodyplan.Stmt{Kind: "let", Name: "result", Expr: 0}, bodyplan.Stmt{Kind: "return", Expr: 6})
		assigned.Base.Root = []int{3, 2, 4}
		for i := range direct.Decisions {
			label := direct.Decisions[i].Options[fallback>>i&1].Label
			direct.Decisions[i].Fallback, assigned.Decisions[i].Fallback = label, label
		}
		cases := []TestCase{{-17, -3}, {31, 0}, {49, -18}}
		_, a := branchOutputInput(t, direct, cases)
		_, b := branchOutputInput(t, assigned, cases)
		for _, choice := range direct.Decisions {
			var x, y [528]float32
			if a.SourceRelativeV4FeaturesInto(choice.ID, &x) != nil || b.SourceRelativeV4FeaturesInto(choice.ID, &y) != nil || x != y {
				t.Fatal("assignment changed canonical branch ownership", fallback, choice.ID)
			}
		}
	}
}
