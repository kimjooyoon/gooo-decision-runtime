package pathplan

import (
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
)

type savedBranchRow struct {
	Document Document     `json:"document"`
	Features [256]float32 `json:"features"`
}

// Read the immutable observation; do not rerun its original model or producer.
func savedBranchRows(t *testing.T) []savedBranchRow {
	t.Helper()
	f, err := os.Open("../studies/branch-feature-discrimination-20261010/result/report.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var report struct {
		Rows []savedBranchRow `json:"rows"`
	}
	if err := json.NewDecoder(z).Decode(&report); err != nil || len(report.Rows) != 2 {
		t.Fatal("saved pair", err)
	}
	return report.Rows
}

func TestBranchReturnRolesDistinguishPublishedCollision(t *testing.T) {
	rows := savedBranchRows(t)
	var vectors [2][256]float32
	for i, row := range rows {
		p, err := row.Document.Prepare()
		if err != nil {
			t.Fatal(err)
		}
		input, _ := p.InitialConditionInput()
		var old [256]float32
		if err := input.FeaturesInto("branches", &old); err != nil || old != row.Features {
			t.Fatal("v1 input changed", err)
		}
		if err := input.FeaturesIntoVersion("branches", decision.ConditionBranchFeatureVersion, &vectors[i]); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(old[:236], vectors[i][:236]) {
			t.Fatal("v1 prefix changed")
		}
		roles, err := p.BranchReturnRoles("branches")
		var want [20]byte
		want[9], want[19] = 8, 8
		if i == 0 {
			want[1], want[10] = 128, 128
		} else {
			want[0], want[11] = 128, 128
		}
		if err != nil || roles != want {
			t.Fatal("direct return roles differ", roles, err)
		}
		// Authored cases never enter static features.
		row.Document.TestCases[0].Expected++
		row.Document.Plan.ConditionCases = []ConditionCase{{ChoiceID: "branches", Input: -9007199254740995, Expected: false}}
		changed, err := row.Document.Prepare()
		if err != nil {
			t.Fatal(err)
		}
		initial, _ := changed.InitialConditionInput()
		var same [256]float32
		if err := initial.FeaturesIntoVersion("branches", decision.ConditionBranchFeatureVersion, &same); err != nil || same != vectors[i] {
			t.Fatal("label leakage", err)
		}
	}
	if vectors[0] == vectors[1] || rows[0].Features != rows[1].Features {
		t.Fatal("collision not distinguished")
	}
}

func branchLocalPlan(name string) Plan {
	p := interactingPlan()
	p.Base.Expressions = []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 10},
		{Kind: "binary", Operation: "less_than", Left: 0, Right: 1}, {Kind: "local", Name: name}}
	p.Base.Statements = []bodyplan.Stmt{{Kind: "let", Name: name, Expr: 0}, {Kind: "assign", Name: name, Expr: 1},
		{Kind: "return", Expr: 3}, {Kind: "return", Expr: 1}, {Kind: "if", Expr: 2, Then: []int{2}, Else: []int{3}}}
	p.Base.Root = []int{0, 1, 4}
	p.Decisions = []Choice{{ID: "branches", Kind: BranchLayout, Target: 4, Intent: "입력과 상수를 선택한다.",
		Fallback: "layout_forward", Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}}}
	return p
}

func TestBranchReturnRolesLocalBindingRenameFallbackAndNested(t *testing.T) {
	var want [20]byte
	want[2], want[4], want[6], want[7], want[8], want[9] = 128, 128, 128, 128, 128, 8
	want[11], want[19] = 128, 8
	for _, name := range []string{"answer", "renamed"} {
		plan := branchLocalPlan(name)
		p, err := Prepare(plan)
		if err != nil {
			t.Fatal(err)
		}
		got, err := p.BranchReturnRoles("branches")
		if err != nil || got != want {
			t.Fatal("local syntax facts", got, err)
		}
		// Mutating caller data cannot change the retained source snapshot.
		plan.Base.Expressions[0].Kind = "int"
		again, _ := p.BranchReturnRoles("branches")
		if again != got {
			t.Fatal("borrowed source")
		}
		plan = branchLocalPlan(name)
		plan.Decisions[0].Fallback = "layout_reverse"
		reversed, err := Prepare(plan)
		if err != nil {
			t.Fatal(err)
		}
		other, err := reversed.BranchReturnRoles("branches")
		if err != nil || !slices.Equal(other[:10], want[10:]) || !slices.Equal(other[10:], want[:10]) {
			t.Fatal("fallback not normalized", err)
		}
	}
	plan := branchLocalPlan("answer")
	plan.Base.Statements = append(plan.Base.Statements, bodyplan.Stmt{Kind: "if", Expr: 2, Then: []int{2}, Else: []int{6}}, bodyplan.Stmt{Kind: "return", Expr: 1})
	plan.Base.Statements[4].Then = []int{5}
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.BranchReturnRoles("branches")
	want[1], want[9] = 128, 16
	if err != nil || got != want {
		t.Fatal("nested unique return sites", got, err)
	}
}

func TestBranchFeaturesReuseObservationAndVersionedSession(t *testing.T) {
	ctx := sessionContext(t)
	p, err := Prepare(conditionContractPlan())
	if err != nil {
		t.Fatal(err)
	}
	initial, _ := p.InitialConditionInput()
	observed, err := p.ObserveConditionInput(context.Background(), p.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	var before, after [256]float32
	if err := initial.FeaturesIntoVersion("branches", decision.ConditionBranchFeatureVersion, &before); err != nil {
		t.Fatal(err)
	}
	if err := observed.FeaturesIntoVersion("branches", decision.ConditionBranchFeatureVersion, &after); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before[:192], after[:192]) || !slices.Equal(before[236:], after[236:]) {
		t.Fatal("feedback altered static source")
	}
	want := after
	if err := observed.FeaturesIntoVersion("branches", "unknown", &after); err == nil || after != want {
		t.Fatal("invalid version changed output")
	}
	if n := testing.AllocsPerRun(20, func() {
		if err := observed.FeaturesIntoVersion("branches", decision.ConditionBranchFeatureVersion, &after); err != nil {
			panic(err)
		}
	}); n != 0 {
		t.Fatal("feature projection allocates", n)
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			var out [256]float32
			if err := observed.FeaturesIntoVersion("branches", decision.ConditionBranchFeatureVersion, &out); err != nil || out != want {
				t.Error("concurrent input", err)
			}
		})
	}
	workers.Wait()
	m, err := conditiondecision.NewForFeatures([conditiondecision.ParameterCount]float32{}, decision.ConditionBranchFeatureVersion)
	if err != nil {
		t.Fatal(err)
	}
	s, err := p.NewConditionSession(ctx, m, []TestCase{{-9007199254740995, -9007199254740995}}, "")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := s.Observe()
	var raw [1024]byte
	for i, x := range before {
		binary.LittleEndian.PutUint32(raw[4*i:], math.Float32bits(x))
	}
	if r.Ranking.FeatureVersion != m.FeatureVersion() || r.Ranking.FeatureSHA[1] != hash(raw[:]) || r.Ranking.Calls != 1 {
		t.Fatal("session did not use versioned features")
	}
	_, _, _ = s.Advance(ctx, 1)
	if _, err := s.Reconsider(ctx, zeroConditionModel(t)); err == nil {
		t.Fatal("same weights with wrong ABI accepted")
	}
	feedback, err := s.Reconsider(ctx, m)
	if err != nil || feedback.FeatureVersion != m.FeatureVersion() || !feedback.HasFailure || feedback.Failure.Result.Case.Input != -9007199254740995 {
		t.Fatal("versioned feedback lost", err)
	}
	for i, x := range after {
		binary.LittleEndian.PutUint32(raw[4*i:], math.Float32bits(x))
	}
	if feedback.FeatureSHA[1] != hash(raw[:]) {
		t.Fatal("feedback session projection differs")
	}
}

func TestBranchReturnRolesOtherChoicesAndExpressionKinds(t *testing.T) {
	for _, kind := range []string{LocalReference, AssignmentTarget, OperandOrder, RootOrder} {
		plan := structuralFixture(kind)
		id := "structure"
		if kind == LocalReference {
			plan = interactingPlan()
			id = "reference"
		}
		p, err := Prepare(plan)
		if err != nil {
			t.Fatal(err)
		}
		roles, err := p.BranchReturnRoles(id)
		if err != nil || roles != [20]byte{} {
			t.Fatal("non-branch fabricated roles", err)
		}
	}
	plan := branchLocalPlan("value")
	plan.Base.Statements[0].Expr = 1
	plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "binary", Operation: "add", Left: 0, Right: 1})
	plan.Base.Statements[3].Expr = 4
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	roles, err := p.BranchReturnRoles("branches")
	if err != nil || roles[4] != 0 || roles[5] != 128 || roles[13] != 128 || roles[11] != 0 {
		t.Fatal("initializer or composed return", roles, err)
	}
	if _, err := p.BranchReturnRoles("missing"); err == nil {
		t.Fatal("undeclared choice accepted")
	}
	var absent *PreparedPlan
	if _, err := absent.BranchReturnRoles("branches"); err == nil {
		t.Fatal("nil plan accepted")
	}
}

func TestBranchRolesAmbiguousNamesDeclineWithDeterministicFallback(t *testing.T) {
	plan := structuralFixture(BranchLayout)
	plan.Base.Statements[2] = bodyplan.Stmt{Kind: "let", Name: "inside", Expr: 2}
	plan.Base.Statements[3] = bodyplan.Stmt{Kind: "let", Name: "inside", Expr: 3}
	firstRead := len(plan.Base.Expressions)
	plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "local", Name: "inside"}, bodyplan.Expr{Kind: "local", Name: "inside"})
	firstWrite := len(plan.Base.Statements)
	plan.Base.Statements = append(plan.Base.Statements, bodyplan.Stmt{Kind: "assign", Name: "first", Expr: firstRead}, bodyplan.Stmt{Kind: "assign", Name: "first", Expr: firstRead + 1})
	plan.Base.Statements[5].Then, plan.Base.Statements[5].Else = []int{2, firstWrite}, []int{3, firstWrite + 1}
	p, err := Prepare(pruneStructuralFixture(plan))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.BranchReturnRoles("structure"); err == nil || !strings.Contains(err.Error(), "MULTIPLE_DECLARATIONS") {
		t.Fatal("ambiguous name silently bound", err)
	}
	model, err := conditiondecision.NewForFeatures([conditiondecision.ParameterCount]float32{}, decision.ConditionBranchFeatureVersion)
	if err != nil {
		t.Fatal(err)
	}
	session, err := p.NewConditionSession(sessionContext(t), model, []TestCase{{4, 16}}, "")
	if err != nil {
		t.Fatal(err)
	}
	initial, err := session.Observe()
	if err != nil || !initial.Ranking.Declined || initial.Ranking.Calls != 0 {
		t.Fatal("decline called model", err)
	}
	result, body, err := session.Advance(sessionContext(t), 4)
	if err != nil || body == nil || result.Search.Status != "TRAINING_COMPLETE" {
		t.Fatal("fallback lost", err)
	}
	if value, err := body.Evaluate(4); err != nil || value.Int != 16 {
		t.Fatal("deterministic value changed", err)
	}
}
