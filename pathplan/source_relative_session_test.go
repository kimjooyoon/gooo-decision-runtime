package pathplan

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func TestSourceRelativeRequirementUsesExportedInputAndChecksOriginalBodies(t *testing.T) {
	searchContext := sessionContext(t)
	var weights [contractdecision.InteractionRequirementParameterCount]float32
	for i := range weights {
		weights[i] = float32(math.Sin(float64(i+1)) * .1)
	}
	{
		model, err := contractdecision.NewSourceRelativeRequirementConditioned(weights)
		if err != nil {
			t.Fatal(err)
		}
		for _, count := range []int{0, 1, 128} {
			var rankings [2]ContractRanking
			for i, plan := range []Plan{orderedArithmeticPlan(), canonicalAssignmentPlan()} {
				plan.ConditionCases = nil
				for j := range count {
					plan.ConditionCases = append(plan.ConditionCases, ConditionCase{ChoiceID: plan.Decisions[0].ID, Input: int64(j) - 64, Expected: j-64 < 10})
				}
				p, err := Prepare(plan)
				if err != nil {
					t.Fatal(err)
				}
				cases := []TestCase{{-5, 15}, {10, 0}, {17, 7}}
				input, err := p.InitialContractInput(cases)
				if err != nil {
					t.Fatal(err)
				}
				var rows [2][528]float32
				for j, c := range plan.Decisions {
					if input.SourceRelativeOrderedSourceFeaturesInto(c.ID, &rows[j]) != nil {
						t.Fatal("source input")
					}
				}
				var work contractdecision.InteractionRequirementWorkspace
				var prediction contractdecision.ChoicePrediction
				if model.PredictChoicesInto(rows[:], input, input, &work, &prediction) != nil {
					t.Fatal("independent exported-input prediction")
				}
				s, err := p.NewSourceRelativeRequirementContractSession(searchContext, model, cases)
				if err != nil {
					t.Fatal(err)
				}
				rankings[i] = s.Ranking()
				r := rankings[i]
				if r.Logits != prediction.Logits || r.Calls != 1 || r.ConditionCount != count || r.SourceFeatures != contractdecision.SourceRelativeOrderedSourceFeatureVersion || r.Schema != "gooo/source-relative-requirement-path-ranking/v1" {
					t.Fatal("source binding", r)
				}
				initial, err := s.Observe()
				if err != nil || initial.Attempted != 0 {
					t.Fatal("prediction executed a candidate", err)
				}
				complete := false
				for range 4 {
					progress, body, err := s.Advance(searchContext, 1)
					if err != nil || len(progress.NewAttempts) != 1 || progress.PredictionsThisAdvance != 0 || s.Ranking() != r {
						t.Fatal("one immediate original candidate without reinference", err)
					}
					if progress.Status == "TRAINING_COMPLETE" {
						if body == nil || progress.Selection.ModelVariant != "source_relative_requirement_contract_fp32" {
							t.Fatal("model route", progress)
						}
						for _, c := range cases {
							value, err := body.Evaluate(c.Input)
							if err != nil || value.Int != c.Expected {
								t.Fatal("original selected body", c, value, err)
							}
						}
						complete = true
						break
					}
					if progress.Exhausted || progress.Status != "PARTIAL" || progress.SelectedPassed >= progress.Cases {
						t.Fatal("incomplete candidate status", progress)
					}
				}
				if !complete {
					t.Fatal("satisfiable fixture did not complete")
				}
			}
			if rankings[0].FeatureSHA != rankings[1].FeatureSHA || rankings[0].Logits != rankings[1].Logits || rankings[0].Proposed != rankings[1].Proposed {
				t.Fatal("equivalent assignments changed actual prediction")
			}
		}
	}
}

func TestSourceRelativeRequirementOmissionDeclineCancellationAndPartial(t *testing.T) {
	searchContext := sessionContext(t)
	p, err := Prepare(orderedArithmeticPlan())
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{-5, 15}, {10, 0}, {17, 7}}
	a, err := p.NewContractSession(searchContext, nil, cases)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.NewSourceRelativeRequirementContractSession(searchContext, nil, cases)
	if err != nil || !reflect.DeepEqual(a.Ranking(), b.Ranking()) {
		t.Fatal("nil changed deterministic path", err)
	}
	model, _ := contractdecision.NewSourceRelativeRequirementConditioned([contractdecision.InteractionRequirementParameterCount]float32{})
	ctx, cancel := context.WithCancel(searchContext)
	cancel()
	if _, err := p.NewSourceRelativeRequirementContractSession(ctx, model, cases); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	plan := orderedArithmeticPlan()
	plan.Base.Expressions[4].Left = 3
	u, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	s, err := u.NewSourceRelativeRequirementContractSession(searchContext, model, cases)
	if err != nil || !s.Ranking().Declined || s.Ranking().Calls != 0 || s.Ranking().PredictNS != 0 {
		t.Fatal("unsupported representation made prediction", err)
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			s, err := p.NewSourceRelativeRequirementContractSession(searchContext, model, []TestCase{{1, 9}, {1, 8}})
			if err != nil {
				t.Error(err)
				return
			}
			progress, body, err := s.Advance(searchContext, 4)
			if err != nil || body == nil || !progress.Exhausted || progress.Status != "PARTIAL" || progress.Attempted != 4 || progress.Cases != 2 || progress.SelectedPassed != 1 || s.Ranking().Calls != 1 {
				t.Error("partial contradiction lost", progress.Status, err)
				return
			}
			value, err := body.Evaluate(1)
			if err != nil || value.Int != 9 {
				t.Error("best partial body lost", value, err)
			}
		})
	}
	wg.Wait()
}

func TestSourceRelativeInputDestinationIsTransactional(t *testing.T) {
	p, err := Prepare(orderedArithmeticPlan())
	if err != nil {
		t.Fatal(err)
	}
	input, err := p.InitialContractInput([]TestCase{{5, 5}})
	if err != nil {
		t.Fatal(err)
	}
	var got [528]float32
	for i := range got {
		got[i] = 7
	}
	before := got
	var missing *ContractInput
	if err := missing.SourceRelativeOrderedSourceFeaturesInto("cmp", &got); err == nil || got != before {
		t.Fatal("nil mutated destination")
	}
	if err := input.SourceRelativeOrderedSourceFeaturesInto("unknown", &got); err == nil || got != before {
		t.Fatal("unknown mutated destination")
	}
	if err := input.SourceRelativeOrderedSourceFeaturesInto(p.plan.Decisions[0].ID, nil); err == nil {
		t.Fatal("nil destination")
	}
	plan := orderedArithmeticPlan()
	plan.Base.Expressions[4].Left = 3
	nested, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	other, err := nested.InitialContractInput([]TestCase{{5, 5}})
	if err != nil {
		t.Fatal(err)
	}
	if err := other.SourceRelativeOrderedSourceFeaturesInto(plan.Decisions[0].ID, &got); err == nil || got != before {
		t.Fatal("unavailable input mutated destination")
	}
}
