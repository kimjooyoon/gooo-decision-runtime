package pathplan

import (
	"context"
	"reflect"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func TestChoiceContractImmediateConstructionAndDeterministicAbsence(t *testing.T) {
	ctx := sessionContext(t)
	prepared, err := Prepare(executionMaxPlan())
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{-9007199254740995, 10}, {9007199254740993, 9007199254740993}, {18014398509481990, 18014398509481990}}
	old, err := prepared.NewContractSession(ctx, nil, cases)
	if err != nil {
		t.Fatal(err)
	}
	absent, err := prepared.NewChoiceContractSession(ctx, nil, cases)
	if err != nil {
		t.Fatal(err)
	}
	if old.Ranking() != absent.Ranking() {
		t.Fatal("nil-model ranking changed")
	}
	a, ab, ae := old.Advance(ctx, 4)
	b, bb, be := absent.Advance(ctx, 4)
	// Elapsed fields differ between independent searches; compare semantic
	// outcomes, choices, counters and actual constructed source instead.
	if ae != nil || be != nil || a.Status != b.Status || a.Attempted != b.Attempted || !reflect.DeepEqual(a.BestCases, b.BestCases) || ab.GoSource() != bb.GoSource() {
		t.Fatal("deterministic continuation", ae, be)
	}
	var weights [contractdecision.ParameterCount]float32
	var contextWeights [contractdecision.ChoiceContextParameterCount]float32
	model, err := contractdecision.NewChoiceConditioned(weights, contextWeights, contractdecision.ExtremePooling)
	if err != nil {
		t.Fatal(err)
	}
	s, err := prepared.NewChoiceContractSession(ctx, model, cases)
	if err != nil {
		t.Fatal(err)
	}
	r := s.Ranking()
	initial, err := s.Observe()
	if err != nil || r.Calls != 1 || !r.Applied || r.ModelFingerprint != model.Fingerprint() || r.CaseFeatures != decision.DeclaredCaseFeatureVersion || initial.Attempted != 0 {
		t.Fatal("initial source-bound ranking", r, err)
	}
	progress, body, err := s.Advance(ctx, 4)
	if err != nil || body == nil || progress.Status != "TRAINING_COMPLETE" || progress.Selection.ModelCalls != 1 || progress.FeedbackPredictions != 0 || progress.Selection.ModelVariant != "choice_conditioned_contract_fp32" {
		t.Fatal("finite construction", progress, err)
	}
	for _, c := range cases {
		v, err := body.Evaluate(c.Input)
		if err != nil || v.Int != c.Expected {
			t.Fatal("exact authored output", c, v, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if s, err := prepared.NewChoiceContractSession(cancelled, model, cases); err == nil || s != nil {
		t.Fatal("cancelled initialization exposed a session")
	}
}

func TestChoiceContractLearnsOppositeGoals(t *testing.T) {
	ctx := sessionContext(t)
	p, err := Prepare(pairedContractPlan())
	if err != nil {
		t.Fatal(err)
	}
	suites := [][]TestCase{
		{{-9007199254740995, 10}, {9007199254740993, 9007199254740993}, {18014398509481990, 18014398509481990}},
		{{-9007199254740995, -9007199254740995}, {9007199254740993, 10}, {18014398509481990, 10}},
	}
	var samples []contractdecision.Sample
	for _, cases := range suites {
		input, err := p.InitialContractInput(cases)
		if err != nil {
			t.Fatal(err)
		}
		features := make([][contractdecision.FeatureDim]float32, 1)
		choice := p.plan.Decisions[0]
		if err := input.RelationalSourceFeaturesInto(choice.ID, &features[0]); err != nil {
			t.Fatal(err)
		}
		sample := contractdecision.Sample{Inputs: features, Cases: input, Masks: []uint16{0, 1}}
		for mask := range 2 {
			body, err := p.Compile(map[string]string{choice.ID: choice.Options[mask].Label})
			if err != nil {
				t.Fatal(err)
			}
			valid := true
			for _, c := range cases {
				value, err := body.Evaluate(c.Input)
				valid = valid && err == nil && value.Int == c.Expected
			}
			if valid {
				sample.Acceptable |= 1 << mask
			}
		}
		if sample.Acceptable == 0 {
			t.Fatal("no valid compiled training label")
		}
		samples = append(samples, sample)
	}
	if !reflect.DeepEqual(samples[0].Inputs, samples[1].Inputs) || samples[0].Acceptable&samples[1].Acceptable != 0 {
		t.Fatal("same source and opposite goals required")
	}
	model, _, err := contractdecision.FitChoiceConditioned(ctx, samples, contractdecision.FitOptions{Epochs: 2000, LearningRate: .3, Seed: 17}, contractdecision.MeanPooling)
	if err != nil {
		t.Fatal(err)
	}
	for i, cases := range suites {
		s, err := p.NewChoiceContractSession(ctx, model, cases)
		if err != nil {
			t.Fatal(err)
		}
		if samples[i].Acceptable>>s.Ranking().Proposed&1 == 0 {
			t.Fatal("learned goal selected wrong path", i, s.Ranking())
		}
		progress, body, err := s.Advance(ctx, 1)
		if err != nil || body == nil || progress.Status != "TRAINING_COMPLETE" || progress.Attempted != 1 {
			t.Fatal("immediate learned construction", progress, err)
		}
	}
}
