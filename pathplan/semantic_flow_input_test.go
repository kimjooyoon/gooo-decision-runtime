package pathplan

import (
	"reflect"
	"slices"
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
)

func semanticForms() []Plan {
	direct := executionMaxPlan()
	assigned := assignmentFlowPlan(false)
	assigned.Decisions = executionMaxPlan().Decisions
	assigned.Decisions[1].Target = 3
	assigned.ConditionCases = direct.ConditionCases
	alias := executionMaxPlan()
	alias.Base.Expressions = []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 10},
		{Kind: "local", Name: "value"}, {Kind: "local", Name: "limit"}, {Kind: "binary", Operation: "less_than", Left: 2, Right: 3}}
	alias.Base.Statements = []bodyplan.Stmt{{Kind: "let", Name: "value", Expr: 0}, {Kind: "let", Name: "limit", Expr: 1},
		{Kind: "return", Expr: 2}, {Kind: "return", Expr: 3}, {Kind: "if", Expr: 4, Then: []int{2}, Else: []int{3}}}
	alias.Base.Root, alias.Decisions[1].Target = []int{0, 1, 4}, 4
	alias.Decisions[0].Target = 4
	copyWrite := branchLocalPlan("value")
	copyWrite.Base.Expressions = []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 10},
		{Kind: "local", Name: "value"}, {Kind: "local", Name: "saved"}, {Kind: "binary", Operation: "less_than", Left: 3, Right: 2}}
	copyWrite.Base.Statements = []bodyplan.Stmt{{Kind: "let", Name: "value", Expr: 0}, {Kind: "let", Name: "saved", Expr: 2},
		{Kind: "assign", Name: "value", Expr: 1}, {Kind: "return", Expr: 3}, {Kind: "return", Expr: 2},
		{Kind: "if", Expr: 4, Then: []int{3}, Else: []int{4}}}
	copyWrite.Base.Root, copyWrite.Decisions = []int{0, 1, 2, 5}, executionMaxPlan().Decisions
	copyWrite.Decisions[1].Target, copyWrite.ConditionCases = 5, direct.ConditionCases
	copyWrite.Decisions[0].Target = 4
	after := assignmentFlowPlan(false)
	after.Decisions, after.ConditionCases = executionMaxPlan().Decisions, direct.ConditionCases
	after.Decisions[1].Target = 3
	after.Base.Expressions = append(after.Base.Expressions, bodyplan.Expr{Kind: "local", Name: "saved"})
	after.Base.Statements[4] = bodyplan.Stmt{Kind: "let", Name: "saved", Expr: 3}
	after.Base.Statements = append(after.Base.Statements, bodyplan.Stmt{Kind: "assign", Name: "result", Expr: 1}, bodyplan.Stmt{Kind: "return", Expr: 4})
	after.Base.Root = []int{0, 3, 4, 5, 6}
	return []Plan{direct, assigned, alias, copyWrite, after}
}

func TestSemanticFlowUnifiesFormsAndKeepsCommittedFeedback(t *testing.T) {
	ctx := sessionContext(t)
	cases := []TestCase{{-9007199254740995, 10}, {9007199254740993, 9007199254740993}, {18014398509481990, 18014398509481990}}
	var initial, observed [2][384]float32
	var original [384]float32
	for form, plan := range semanticForms() {
		p, err := Prepare(plan)
		if err != nil {
			t.Fatal(form, err)
		}
		before, _ := p.InitialExecutionInput(cases)
		after, err := p.ObserveExecutionInput(ctx, p.Defaults(), cases)
		if err != nil {
			t.Fatal(err)
		}
		for i, choice := range plan.Decisions {
			view, err := p.SemanticBranchContext(choice.ID)
			if err != nil || !view.Normalized || view.Flow.Returns[0].Kind != "input" || view.Flow.Returns[1].Int != 10 {
				t.Fatal("source atoms differ", form, view, err)
			}
			var a, b, old [384]float32
			if err := before.ExecutionSemanticFlowFeaturesInto(choice.ID, &a); err != nil {
				t.Fatal(err)
			}
			if err := after.ExecutionSemanticFlowFeaturesInto(choice.ID, &b); err != nil {
				t.Fatal(err)
			}
			if err := before.ExecutionFlowFeaturesInto(choice.ID, &old); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(a[64:236], old[64:236]) || !slices.Equal(a[256:320], old[256:320]) || a[255] != 2 || a == b || !slices.Equal(a[320:], b[320:]) {
				t.Fatal("normalization altered intent/observation/static boundaries", form)
			}
			if form == 0 {
				initial[i], observed[i] = a, b
			} else if a != initial[i] || b != observed[i] {
				t.Fatal("same value flow has different normalized input", form, choice.ID)
			}
			if i == 1 {
				if form == 0 {
					original = old
				} else if form == 1 && original == old {
					t.Fatal("expected v4 direct/assignment difference absent")
				}
			}
		}
	}
}

func TestSemanticFlowDifferentMeaningAndUnsupportedShape(t *testing.T) {
	base := semanticForms()[0]
	p, _ := Prepare(base)
	input, _ := p.InitialExecutionInput([]TestCase{{0, 10}})
	var original [384]float32
	input.ExecutionSemanticFlowFeaturesInto("branches", &original)
	for _, constant := range []int64{9007199254740993, 9007199254740995, -9007199254740995, 18014398509481990} {
		plan := semanticForms()[0]
		plan.Base.Expressions[1].Int = constant
		other, err := Prepare(plan)
		if err != nil {
			t.Fatal(err)
		}
		x, _ := other.InitialExecutionInput([]TestCase{{0, 10}})
		var a [384]float32
		if err := x.ExecutionSemanticFlowFeaturesInto("branches", &a); err != nil || a == original {
			t.Fatal("exact source constant disappeared", err)
		}
		view, _ := other.SemanticBranchContext("branches")
		if view.Flow.Returns[1].Int != constant {
			t.Fatal("source integer rounded")
		}
	}
	nested := executionMaxPlan()
	nested.Base.Statements = append(nested.Base.Statements, bodyplan.Stmt{Kind: "if", Expr: 2, Then: []int{0}, Else: []int{4}}, bodyplan.Stmt{Kind: "return", Expr: 1})
	nested.Base.Statements[2].Then = []int{3}
	nested.ConditionCases = nil
	other, err := Prepare(nested)
	if err != nil {
		t.Fatal(err)
	}
	view, err := other.SemanticBranchContext("branches")
	if err != nil || view.Normalized {
		t.Fatal("nested shape asserted normalized", err)
	}
	x, _ := other.InitialExecutionInput([]TestCase{{0, 10}})
	var old, next [384]float32
	if x.ExecutionFlowFeaturesInto("branches", &old) != nil || x.ExecutionSemanticFlowFeaturesInto("branches", &next) != nil || old != next {
		t.Fatal("ineligible shape lost v4 information")
	}
	next = original
	if x.ExecutionSemanticFlowFeaturesInto("missing", &next) == nil || next != original {
		t.Fatal("error changed destination")
	}
}

func TestSemanticFlowModelRoutesActualInputConcurrently(t *testing.T) {
	ctx := sessionContext(t)
	m, err := flowdecision.NewForFeatures([flowdecision.ParameterCount]float32{}, decision.SemanticFlowFeatureVersion)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := m.Marshal()
	loaded, err := flowdecision.Decode(raw)
	if err != nil || loaded.Fingerprint() != m.Fingerprint() || loaded.FeatureVersion() != decision.SemanticFlowFeatureVersion {
		t.Fatal("v5 artifact identity", err)
	}
	p, err := Prepare(semanticForms()[3])
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{-1, 10}, {20, 20}}
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			s, err := p.NewFlowSession(ctx, loaded, cases, "")
			if err != nil {
				t.Error(err)
				return
			}
			input, _ := s.ExecutionInput()
			before, _ := s.Observe()
			for i, choice := range p.plan.Decisions {
				var features [384]float32
				if input.ExecutionSemanticFlowFeaturesInto(choice.ID, &features) != nil || flowDigest(features) != before.Ranking.FeatureSHA[i] {
					t.Error("model did not consume the explicit v5 array")
				}
			}
			if _, _, err := s.Advance(ctx, 1); err != nil {
				t.Error(err)
			}
			if _, err := s.ReconsiderFlow(ctx, loaded); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	want, _, _, err := p.SearchBatches(ctx, nil, cases, 4, 1, "")
	got, _, _, _, otherErr := p.SearchFlowBatches(ctx, nil, cases, 4, 1, "", 0)
	if err != nil || otherErr != nil || !reflect.DeepEqual(want, got) {
		t.Fatal("no-model continuation differs", err, otherErr)
	}
}
