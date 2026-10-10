package pathplan

import (
	"reflect"
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
)

func TestRelationalFlowEquivalentFormsAndCommittedObservations(t *testing.T) {
	ctx := sessionContext(t)
	cases := []TestCase{{-9007199254740995, 10}, {9007199254740993, 9007199254740993}}
	var reference [2][2][384]float32
	for form, plan := range semanticForms() {
		p, err := Prepare(plan)
		if err != nil {
			t.Fatal(err)
		}
		initial, err := p.InitialExecutionInput(cases)
		if err != nil {
			t.Fatal(err)
		}
		observed, err := p.ObserveExecutionInput(ctx, p.Defaults(), cases)
		if err != nil {
			t.Fatal(err)
		}
		for state, input := range []*ConditionInput{initial, observed} {
			for i, choice := range plan.Decisions {
				var old, next [384]float32
				if err := input.ExecutionSemanticFlowFeaturesInto(choice.ID, &old); err != nil {
					t.Fatal(err)
				}
				if err := input.ExecutionRelationalFlowFeaturesInto(choice.ID, &next); err != nil {
					t.Fatal(err)
				}
				for j, v := range old {
					if (j < 236 || j >= 256) && v != next[j] {
						t.Fatal("changed intent, evidence or exact source atoms", j)
					}
				}
				if next[255] != 3 || next[239] != 1 || next[248] != 1 {
					t.Fatal("missing source relations")
				}
				if form == 0 {
					reference[state][i] = next
				} else if reference[state][i] != next {
					t.Fatal("form changed relation input", form, state, i)
				}
			}
		}
	}
	if reference[0] == reference[1] {
		t.Fatal("committed failure disappeared")
	}
}

func TestRelationalFlowUnsupportedInputKeepsV5AndErrorsKeepOutput(t *testing.T) {
	plan := executionMaxPlan()
	plan.Base.Statements = append(plan.Base.Statements, bodyplan.Stmt{Kind: "if", Expr: 2, Then: []int{0}, Else: []int{4}}, bodyplan.Stmt{Kind: "return", Expr: 1})
	plan.Base.Statements[2].Then = []int{3}
	plan.ConditionCases = nil
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	input, err := p.InitialExecutionInput([]TestCase{{0, 10}})
	if err != nil {
		t.Fatal(err)
	}
	var old, next [384]float32
	if err := input.ExecutionSemanticFlowFeaturesInto("branches", &old); err != nil {
		t.Fatal(err)
	}
	if err := input.ExecutionRelationalFlowFeaturesInto("branches", &next); err != nil || old != next {
		t.Fatal("raw fallback", err)
	}
	if input.ExecutionRelationalFlowFeaturesInto("missing", &next) == nil || old != next {
		t.Fatal("transactional error")
	}
	if input.ExecutionRelationalFlowFeaturesInto("branches", nil) == nil {
		t.Fatal("nil destination")
	}
}

func TestRelationalFlowModelUsesExactInputsConcurrently(t *testing.T) {
	ctx := sessionContext(t)
	m, err := flowdecision.NewForFeatures([flowdecision.ParameterCount]float32{}, decision.RelationalFlowFeatureVersion)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Prepare(semanticForms()[3])
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{-1, 10}, {20, 20}}
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			s, err := p.NewFlowSession(ctx, m, cases, "")
			if err != nil {
				t.Error(err)
				return
			}
			input, err := s.ExecutionInput()
			if err != nil {
				t.Error(err)
				return
			}
			r, err := s.Observe()
			if err != nil {
				t.Error(err)
				return
			}
			for i, c := range p.plan.Decisions {
				var f [384]float32
				if input.ExecutionRelationalFlowFeaturesInto(c.ID, &f) != nil || flowDigest(f) != r.Ranking.FeatureSHA[i] {
					t.Error("runtime did not consume v6")
				}
			}
			if _, _, err := s.Advance(ctx, 1); err != nil {
				t.Error(err)
			}
			if _, err := s.ReconsiderFlow(ctx, m); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	a, _, _, e1 := p.SearchBatches(ctx, nil, cases, 4, 1, "")
	b, _, _, _, e2 := p.SearchFlowBatches(ctx, nil, cases, 4, 1, "", 0)
	if e1 != nil || e2 != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("no-model behavior", e1, e2)
	}
}
