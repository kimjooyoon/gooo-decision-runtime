package contractdecision

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestGoalPairTrainingAndInferenceContract(t *testing.T) {
	samples := paired()
	pairs := []GoalPair{{0, 1}}
	options := FitOptions{Epochs: 200, LearningRate: .3, Seed: 17}
	pairOptions := GoalPairOptions{Weight: .5, Margin: 2}
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		model, history, err := FitWithGoalPairs(context.Background(), samples, options, pooling, pairs, pairOptions)
		if err != nil || len(history) != options.Epochs || history[len(history)-1].Loss >= history[0].Loss/2 {
			t.Fatal(pooling, history, err)
		}
		again, repeated, err := FitWithGoalPairs(context.Background(), samples, options, pooling, pairs, pairOptions)
		if err != nil || again.Weights() != model.Weights() || !reflect.DeepEqual(history, repeated) {
			t.Fatal("same-runtime pair training differs", err)
		}
		raw, err := model.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := Decode(raw)
		if err != nil || loaded.Fingerprint() != model.Fingerprint() || loaded.Pooling() != pooling {
			t.Fatal("pair training changed artifact contract", err)
		}
		for i, sample := range samples {
			var workspace Workspace
			var prediction Prediction
			if err := loaded.PredictInto(sample.Inputs, sample.Cases, sample.Masks, &workspace, &prediction); err != nil || prediction.Selected != uint16(i) {
				t.Fatal("goal not selected", i, prediction, err)
			}
		}
	}
	if ParameterCount != 9746 {
		t.Fatal("parameter budget changed")
	}
}

func TestGoalPairValidation(t *testing.T) {
	valid := FitOptions{Epochs: 1, LearningRate: .1}
	pairs := []GoalPair{{0, 1}}
	pairOptions := GoalPairOptions{Weight: .5, Margin: 2}
	for _, bad := range [][]GoalPair{nil, {{0, 0}}, {{-1, 1}}, {{0, 2}}, {{0, 1}, {1, 0}}} {
		if model, _, err := FitWithGoalPairs(context.Background(), paired(), valid, MeanPooling, bad, pairOptions); err == nil || model != nil {
			t.Fatal("bad pairs accepted", bad, err)
		}
	}
	for _, bad := range []GoalPairOptions{{}, {Weight: -1}, {Weight: math.NaN()}, {Weight: math.Inf(1)}, {Weight: 1, Margin: -1}, {Weight: 1, Margin: math.NaN()}, {Weight: 1, Margin: math.Inf(1)}} {
		if model, _, err := FitWithGoalPairs(context.Background(), paired(), valid, MeanPooling, pairs, bad); err == nil || model != nil {
			t.Fatal("bad pair options accepted", bad, err)
		}
	}
	for _, change := range []func([]Sample){
		func(s []Sample) {
			s[1].Inputs = append([][FeatureDim]float32(nil), s[1].Inputs...)
			s[1].Inputs[0][0]++
		},
		func(s []Sample) { s[1].Masks = []uint16{1, 0} },
		func(s []Sample) { s[1].Acceptable = s[0].Acceptable },
	} {
		samples := paired()
		change(samples)
		if model, _, err := FitWithGoalPairs(context.Background(), samples, valid, MeanPooling, pairs, pairOptions); err == nil || model != nil {
			t.Fatal("mismatched goals accepted", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if model, _, err := FitWithGoalPairs(ctx, paired(), valid, MeanPooling, pairs, pairOptions); model != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled fit", err)
	}
	if model, _, err := FitWithGoalPairs(nil, paired(), valid, MeanPooling, pairs, pairOptions); model != nil || err == nil {
		t.Fatal("nil context accepted", err)
	}
}

func TestGoalPairGradientMatchesFiniteDifference(t *testing.T) {
	samples := paired()
	samples[0].Cases = rows{{.7, .2}, {.3, .8}}
	samples[1].Cases = rows{{.1, .4}, {.9, .6}}
	pairs := []GoalPair{{0, 1}}
	options := GoalPairOptions{Weight: .5, Margin: 2}
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		model, _ := NewForPooling(initial(17), pooling)
		var gradient [ParameterCount]float64
		loss, err := model.goalPairGradient(context.Background(), samples, pairs, options, &gradient)
		if err != nil || loss <= 0 {
			t.Fatal(loss, err)
		}
		for _, i := range []int{0, 1, CaseDim, caseBias, sourceWeights, sourceWeights + FeatureDim, hiddenBias, outputWeights, outputWeights + HiddenDim, outputBias} {
			old := model.weights[i]
			var ignored [ParameterCount]float64
			model.weights[i] = old + .0005
			up, err := model.goalPairGradient(context.Background(), samples, pairs, options, &ignored)
			if err != nil {
				t.Fatal(err)
			}
			model.weights[i] = old - .0005
			down, err := model.goalPairGradient(context.Background(), samples, pairs, options, &ignored)
			if err != nil {
				t.Fatal(err)
			}
			model.weights[i] = old
			if math.Abs((up-down)/.001-gradient[i]) > .0002 {
				t.Fatal("paired encoder gradient", pooling, i, gradient[i], (up-down)/.001)
			}
		}
	}
}

func TestGoalPairSetResidualAndStableLoss(t *testing.T) {
	logits := [2][2][2]float64{{{.3, -.2}, {.1, .4}}, {{-.1, .7}, {.6, -.4}}}
	masks := []uint16{0, 1, 2, 3}
	compute := func() (float64, [2][MaxChoices][2]float64) {
		var work [2]Workspace
		for goal := range 2 {
			for i, mask := range masks {
				for choice := range 2 {
					work[goal].scores[i] += logits[goal][choice][mask>>choice&1]
				}
			}
		}
		return goalPairResidual(&work[0], &work[1], masks, 2, 3, 12, 2, .7)
	}
	_, residual := compute()
	for goal := range 2 {
		for choice := range 2 {
			for option := range 2 {
				original := logits[goal][choice][option]
				logits[goal][choice][option] = original + .00001
				up, _ := compute()
				logits[goal][choice][option] = original - .00001
				down, _ := compute()
				logits[goal][choice][option] = original
				if math.Abs((up-down)/.00002-residual[goal][choice][option]) > 1e-8 {
					t.Fatal("set gradient", goal, choice, option)
				}
			}
		}
	}
	for _, x := range []float64{-1000, 0, 1000} {
		loss, slope := softplusSlope(x)
		if math.IsNaN(loss) || math.IsInf(loss, 0) || slope < 0 || slope > 1 {
			t.Fatal(x, loss, slope)
		}
	}
}

func TestGoalPairReadsEveryCaseAndPreservesLegacyFit(t *testing.T) {
	samples := paired()
	cases := &countedCases{n: 128, fail: -1}
	samples[0].Cases = cases
	options := FitOptions{Epochs: 1, LearningRate: .1, Seed: 17}
	pairs := []GoalPair{{0, 1}}
	pairOptions := GoalPairOptions{Weight: .5, Margin: 2}
	if _, _, err := FitWithGoalPairs(context.Background(), samples, options, MeanPooling, pairs, pairOptions); err != nil || cases.seen != 256 {
		t.Fatal("case rows were truncated or reread inside a pass", cases.seen, err)
	}
	cases.seen, cases.fail = 0, 127
	if model, _, err := FitWithGoalPairs(context.Background(), samples, options, MeanPooling, pairs, pairOptions); err == nil || model != nil || cases.seen != 128 {
		t.Fatal("case reader failure not propagated", cases.seen, err)
	}
	samples = paired()
	legacy, history, err := Fit(context.Background(), samples, options)
	if err != nil {
		t.Fatal(err)
	}
	explicit, explicitHistory, err := FitForPooling(context.Background(), samples, options, MeanPooling)
	if err != nil || legacy.Weights() != explicit.Weights() || !reflect.DeepEqual(history, explicitHistory) {
		t.Fatal("legacy fit routes diverged", err)
	}
	model, _ := New(initial(17))
	var first, swapped [ParameterCount]float64
	a, err := model.goalPairGradient(context.Background(), samples, pairs, pairOptions, &first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := model.goalPairGradient(context.Background(), samples, []GoalPair{{1, 0}}, pairOptions, &swapped)
	if err != nil || a != b {
		t.Fatal("pair orientation changed loss", a, b, err)
	}
	for i := range first {
		if math.Abs(first[i]-swapped[i]) > 1e-14 {
			t.Fatal("pair orientation changed gradient", i)
		}
	}
}
