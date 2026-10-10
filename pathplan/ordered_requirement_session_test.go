package pathplan

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func orderedTrainingPair(t *testing.T) ([2]*PreparedPlan, []TestCase, []contractdecision.OrderedRequirementSample) {
	t.Helper()
	var prepared [2]*PreparedPlan
	cases := []TestCase{{-2, 13}, {11, 0}, {14, 3}}
	samples := make([]contractdecision.OrderedRequirementSample, 2)
	var old [2][384]float32
	for i := range 2 {
		plan := orderedArithmeticPlan()
		plan.Base.Expressions[1].Int = 11
		plan.Decisions = plan.Decisions[1:]
		plan.ConditionCases = []ConditionCase{{ChoiceID: plan.Decisions[0].ID, Input: -2, Expected: true}}
		if i == 1 {
			plan.Base.Statements[0].Expr, plan.Base.Statements[1].Expr = 4, 3
		}
		p, err := Prepare(plan)
		if err != nil {
			t.Fatal(err)
		}
		prepared[i] = p
		input, err := p.InitialContractInput(cases)
		if err != nil {
			t.Fatal(err)
		}
		s := contractdecision.OrderedRequirementSample{Inputs: make([][528]float32, 1), Cases: input, Conditions: input, Masks: []uint16{0, 1}}
		id := plan.Decisions[0].ID
		if input.OrderedSourceFeaturesInto(id, &s.Inputs[0]) != nil || input.RelationalSourceFeaturesInto(id, &old[i]) != nil {
			t.Fatal("source projection")
		}
		for mask, option := range plan.Decisions[0].Options {
			body, err := p.Compile(map[string]string{id: option.Label})
			if err != nil {
				t.Fatal(err)
			}
			passed := true
			for _, c := range cases {
				value, err := body.Evaluate(c.Input)
				passed = passed && err == nil && value.Int == c.Expected
			}
			if passed {
				s.Acceptable |= 1 << mask
			}
		}
		if s.Acceptable != uint64(1<<(1-i)) {
			t.Fatal("fresh source-labelled pair", s.Acceptable)
		}
		samples[i] = s
	}
	if old[0] != old[1] || samples[0].Inputs[0] == samples[1].Inputs[0] {
		t.Fatal("ordered arithmetic distinction missing")
	}
	return prepared, cases, samples
}

func TestOrderedRequirementLearnsSourceOrderAndImmediatelyChecksBodies(t *testing.T) {
	ctx := sessionContext(t)
	plans, cases, samples := orderedTrainingPair(t)
	audit, err := contractdecision.AuditOrderedRequirements(context.Background(), samples)
	if err != nil || audit.BestFirstPasses != 2 || len(audit.Groups) != 2 {
		t.Fatal(audit, err)
	}
	model, history, err := contractdecision.FitOrderedRequirementConditioned(context.Background(), samples,
		contractdecision.FitOptions{Epochs: 2000, LearningRate: .3, Seed: 17}, contractdecision.MeanPooling)
	if err != nil || len(history) != 2000 || history[1999].Loss >= history[0].Loss {
		t.Fatal("ordered training", err)
	}
	for i, plan := range plans {
		session, err := plan.NewOrderedRequirementContractSession(ctx, model, cases)
		if err != nil {
			t.Fatal(err)
		}
		initial, _ := session.Observe()
		ranking := session.Ranking()
		if ranking.Calls != 1 || !ranking.Applied || ranking.Proposed != uint16(1-i) || initial.Attempted != 0 ||
			ranking.SourceFeatures != contractdecision.OrderedSourceFeatureVersion || ranking.ConditionCount != 1 || ranking.ConditionFeatureSHA == "" {
			t.Fatal("new source input was not used before construction", i, ranking)
		}
		result, body, err := session.Advance(ctx, 2)
		if err != nil || body == nil || result.Attempted != 1 || result.Status != "TRAINING_COMPLETE" || session.Ranking() != ranking {
			t.Fatal("learned path or fixed inference count", i, result, err)
		}
	}
	t.Logf("two training fixtures: first loss %.6f, final loss %.6f; learned accuracy outside fixtures unmeasured", history[0].Loss, history[1999].Loss)
}

func TestOrderedRequirementDeclinesUnsupportedAndNilKeepsDeterminism(t *testing.T) {
	ctx := sessionContext(t)
	plans, cases, _ := orderedTrainingPair(t)
	plan := plans[0]
	a, err := plan.NewContractSession(ctx, nil, cases)
	if err != nil {
		t.Fatal(err)
	}
	b, err := plan.NewOrderedRequirementContractSession(ctx, nil, cases)
	if err != nil || !reflect.DeepEqual(a.Ranking(), b.Ranking()) {
		t.Fatal("nil model changed deterministic ranking", err)
	}
	unavailable := orderedArithmeticPlan()
	unavailable.Base.Expressions[4].Left = 3 // Nested arithmetic remains valid Gooo.
	p, err := Prepare(unavailable)
	if err != nil {
		t.Fatal(err)
	}
	model, _ := contractdecision.NewOrderedRequirementConditioned([contractdecision.OrderedRequirementParameterCount]float32{}, contractdecision.MeanPooling)
	s, err := p.NewOrderedRequirementContractSession(ctx, model, cases)
	if err != nil || !s.Ranking().Declined || s.Ranking().Calls != 0 || !strings.Contains(s.Ranking().Error, "ORDERED_SOURCE_CONTEXT_UNAVAILABLE") {
		t.Fatal("unsupported source was silently encoded", err)
	}
	input, _ := p.InitialContractInput(cases)
	var output [528]float32
	output[527] = 42
	before := output
	if input.OrderedSourceFeaturesInto(unavailable.Decisions[0].ID, &output) == nil || output != before {
		t.Fatal("declined representation changed destination")
	}
}
