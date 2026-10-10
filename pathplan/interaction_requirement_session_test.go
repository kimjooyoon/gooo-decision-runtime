package pathplan

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func interactionTrainingCube(t *testing.T) ([]*PreparedPlan, [][]TestCase, []contractdecision.InteractionRequirementSample) {
	t.Helper()
	ctx := sessionContext(t)
	var plans []*PreparedPlan
	var suites [][]TestCase
	var samples []contractdecision.InteractionRequirementSample
	for reversed := range 2 {
		for outputGoal := range 2 {
			for conditionGoal := range 2 {
				plan := orderedArithmeticPlan()
				plan.Base.Expressions[1].Int = 29
				plan.Decisions[0].Intent = "작성한 중간 비교 조건을 맞춘다"
				plan.Decisions[1].Intent = "출력 예시에 맞는 거리값을 반환한다"
				if reversed == 1 {
					plan.Base.Statements[0].Expr, plan.Base.Statements[1].Expr = 4, 3
				}
				var cases []TestCase
				plan.ConditionCases = nil
				for _, x := range []int64{-9007199254740995, 29, 9007199254740993} {
					expected := x - 29
					if x >= 29 {
						expected = 29 - x
					}
					if outputGoal == 1 {
						expected = -expected
					}
					cases = append(cases, TestCase{Input: x, Expected: expected})
					want := x < 29
					if conditionGoal == 1 {
						want = 29 < x
					}
					plan.ConditionCases = append(plan.ConditionCases, ConditionCase{ChoiceID: plan.Decisions[0].ID, Input: x, Expected: want})
				}
				p, err := Prepare(plan)
				if err != nil {
					t.Fatal(err)
				}
				input, err := p.InitialContractInput(cases)
				if err != nil {
					t.Fatal(err)
				}
				sample := contractdecision.InteractionRequirementSample{Inputs: make([][528]float32, 2),
					Cases: input, Conditions: input, Masks: []uint16{0, 1, 2, 3}}
				for i, c := range plan.Decisions {
					if err := input.OrderedSourceFeaturesInto(c.ID, &sample.Inputs[i]); err != nil {
						t.Fatal(err)
					}
				}
				for mask := range 4 {
					choices := map[string]string{}
					for i, c := range plan.Decisions {
						choices[c.ID] = c.Options[mask>>i&1].Label
					}
					observation, err := p.ObserveExecutionInput(ctx, choices, cases)
					if err != nil {
						t.Fatal(err)
					}
					_, failedOutput := observation.OutputFailure()
					_, failedCondition := observation.Failure()
					if !failedOutput && !failedCondition {
						sample.Acceptable |= 1 << mask
					}
				}
				if sample.Acceptable == 0 {
					t.Fatal("training contract has no valid candidate")
				}
				plans, suites, samples = append(plans, p), append(suites, cases), append(samples, sample)
			}
		}
	}
	return plans, suites, samples
}

func TestInteractionRequirementLearnsJointSourceOutputConditionCube(t *testing.T) {
	plans, cases, samples := interactionTrainingCube(t)
	model, history, err := contractdecision.FitInteractionRequirementConditioned(context.Background(), samples,
		contractdecision.FitOptions{Epochs: 2000, LearningRate: .3, L2: .0001, Seed: 17}, contractdecision.MeanPooling)
	if err != nil || len(history) != 2000 {
		t.Fatal("interaction fit", err)
	}
	raw, err := model.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	model, err = contractdecision.DecodeInteractionRequirementConditioned(raw)
	if err != nil {
		t.Fatal(err)
	}
	firstComplete := 0
	ctx := sessionContext(t)
	for i, plan := range plans {
		session, err := plan.NewInteractionRequirementContractSession(ctx, model, cases[i])
		if err != nil {
			t.Fatal(err)
		}
		ranking := session.Ranking()
		initial, _ := session.Observe()
		if ranking.Calls != 1 || !ranking.Applied || initial.Attempted != 0 || ranking.SourceFeatures != contractdecision.OrderedSourceFeatureVersion {
			t.Fatal("model did not rank before candidate execution", ranking)
		}
		result, body, err := session.Advance(ctx, 4)
		if err != nil || body == nil || result.Status != "TRAINING_COMPLETE" || session.Ranking() != ranking {
			t.Fatal("finite checking or fixed model calls changed", i, result, err)
		}
		if result.Attempted == 1 {
			firstComplete++
		}
	}
	t.Logf("training cube first %d/8, loss %.6f -> %.6f; held-out accuracy unmeasured", firstComplete, history[0].Loss, history[1999].Loss)
	if firstComplete != 8 || history[1999].Loss > .1 {
		t.Fatal("input-goal-source relationships were not learned")
	}
	permutations := [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
	for i, original := range plans {
		for _, order := range permutations {
			plan := original.plan
			plan.ConditionCases = make([]ConditionCase, 3)
			reorderedCases := make([]TestCase, 3)
			for j, index := range order {
				plan.ConditionCases[j] = original.plan.ConditionCases[index]
				reorderedCases[j] = cases[i][index]
			}
			p, err := Prepare(plan)
			if err != nil {
				t.Fatal(err)
			}
			s, err := p.NewInteractionRequirementContractSession(ctx, model, reorderedCases)
			if err != nil {
				t.Fatal(err)
			}
			result, body, err := s.Advance(ctx, 4)
			if err != nil || body == nil || result.Attempted != 1 || result.Status != "TRAINING_COMPLETE" {
				t.Fatal("case permutation changed the learned decision", i, order, result, err)
			}
		}
	}
}

func TestInteractionRequirementDeterminismAndUnsupportedSource(t *testing.T) {
	ctx := sessionContext(t)
	plan, err := Prepare(orderedArithmeticPlan())
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{Input: -2, Expected: -12}}
	a, err := plan.NewContractSession(ctx, nil, cases)
	if err != nil {
		t.Fatal(err)
	}
	b, err := plan.NewInteractionRequirementContractSession(ctx, nil, cases)
	if err != nil || !reflect.DeepEqual(a.Ranking(), b.Ranking()) {
		t.Fatal("nil model changed deterministic behavior", err)
	}
	unsupported := orderedArithmeticPlan()
	unsupported.Base.Expressions[4].Left = 3
	p, err := Prepare(unsupported)
	if err != nil {
		t.Fatal(err)
	}
	model, _ := contractdecision.NewInteractionRequirementConditioned([contractdecision.InteractionRequirementParameterCount]float32{}, contractdecision.MeanPooling)
	s, err := p.NewInteractionRequirementContractSession(ctx, model, cases)
	if err != nil || !s.Ranking().Declined || s.Ranking().Calls != 0 || !strings.Contains(s.Ranking().Error, "ORDERED_SOURCE_CONTEXT_UNAVAILABLE") {
		t.Fatal("unsupported source did not preserve deterministic fallback", err)
	}
}
