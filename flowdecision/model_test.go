package flowdecision

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func fixture() Sample {
	inputs := make([][FeatureDim]float32, 2)
	inputs[0][0], inputs[1][1] = 1, 1
	return Sample{Inputs: inputs, Masks: []uint16{0, 1, 2, 3}, Acceptable: 1<<1 | 1<<2}
}

func TestFitUsesAcceptableCompleteMasksAndArtifactRoundTrip(t *testing.T) {
	sample := fixture()
	options := FitOptions{Epochs: 160, LearningRate: 0.3, Seed: 17}
	m, history, err := Fit(context.Background(), []Sample{sample}, options)
	if err != nil || len(history) != 160 || history[159].Loss >= history[0].Loss*0.2 {
		t.Fatal(history, err)
	}
	var work Workspace
	var got Prediction
	if err := m.PredictInto(sample.Inputs, sample.Masks, &work, &got); err != nil {
		t.Fatal(err)
	}
	if got.Selected != 1 && got.Selected != 2 {
		t.Fatal("independently valid bits formed an invalid complete mask", got)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	var replay Prediction
	if err := loaded.PredictInto(sample.Inputs, sample.Masks, &work, &replay); err != nil || got != replay {
		t.Fatal(replay, err)
	}
	second, secondHistory, err := Fit(context.Background(), []Sample{sample}, options)
	if err != nil || second.Weights() != m.Weights() || !reflect.DeepEqual(history, secondHistory) {
		t.Fatal("fixed training is nondeterministic", err)
	}
	copy := m.Weights()
	copy[0] = 99
	if m.Weights()[0] == 99 {
		t.Fatal("weights escape immutable model")
	}
	if n := testing.AllocsPerRun(20, func() {
		if err := m.PredictInto(sample.Inputs, sample.Masks, &work, &got); err != nil {
			panic(err)
		}
	}); n != 0 {
		t.Fatal("inference allocates", n)
	}
}

func TestPredictionBoundsTiesAtomicityAndConcurrency(t *testing.T) {
	m, _ := New([ParameterCount]float32{})
	inputs := make([][FeatureDim]float32, 16)
	masks := make([]uint16, 64)
	for i := range masks {
		masks[i] = uint16(i)
	}
	masks[63] = 65535
	var work Workspace
	var got Prediction
	if err := m.PredictInto(inputs, masks, &work, &got); err != nil || got.Selected != 0 || got.Count != 64 {
		t.Fatal(got, err)
	}
	for _, p := range got.Probabilities {
		if p != 1.0/64 {
			t.Fatal(p)
		}
	}
	if err := m.PredictInto(inputs, []uint16{9, 7, 2}, &work, &got); err != nil || got.Selected != 2 {
		t.Fatal("tie depends on list ordering", got, err)
	}
	wantWork, wantGot := work, got
	for _, bad := range []struct {
		inputs [][FeatureDim]float32
		masks  []uint16
	}{
		{nil, []uint16{0}}, {inputs, nil}, {inputs, make([]uint16, 65)}, {inputs, []uint16{1, 1}},
		{inputs[:1], []uint16{2}}, {make([][FeatureDim]float32, 17), []uint16{0}},
	} {
		if err := m.PredictInto(bad.inputs, bad.masks, &work, &got); err == nil || work != wantWork || got != wantGot {
			t.Fatal("invalid input changed result", err)
		}
	}
	inputs[0][0] = float32(math.NaN())
	if err := m.PredictInto(inputs, masks, &work, &got); err == nil || work != wantWork || got != wantGot {
		t.Fatal(err)
	}
	inputs[0][0] = 0
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			var w Workspace
			var p Prediction
			for range 10 {
				if err := m.PredictInto(inputs, masks, &w, &p); err != nil || p.Selected != 0 {
					t.Error(p, err)
				}
			}
		})
	}
	workers.Wait()
}

func TestFitInvalidAndCancelledRequests(t *testing.T) {
	sample := fixture()
	valid := FitOptions{Epochs: 1, LearningRate: 0.1, Seed: 1}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if m, _, err := Fit(ctx, []Sample{sample}, valid); m != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(m, err)
	}
	for _, options := range []FitOptions{{}, {Epochs: 10001, LearningRate: 0.1}, {Epochs: 1, LearningRate: math.Inf(1)}, {Epochs: 1, LearningRate: 0.1, L2: -1}} {
		if m, _, err := Fit(context.Background(), []Sample{sample}, options); err == nil || m != nil {
			t.Fatal(m, err)
		}
	}
	for _, bits := range []uint64{0, 1 << 4} {
		sample.Acceptable = bits
		if m, _, err := Fit(context.Background(), []Sample{sample}, valid); err == nil || m != nil {
			t.Fatal(m, err)
		}
	}
	var invalid [ParameterCount]float32
	invalid[2] = float32(math.Inf(1))
	if _, err := New(invalid); err == nil {
		t.Fatal("nonfinite weights accepted")
	}
	m, _ := New([ParameterCount]float32{})
	raw, _ := m.Marshal()
	for _, bad := range []string{string(raw) + " {}", strings.Replace(string(raw), `"schema":`, `"unknown":0,"schema":`, 1), strings.Replace(string(raw), Schema, "wrong", 1), strings.Replace(string(raw), `"schema":`, `"schema":"duplicate","schema":`, 1)} {
		if _, err := Decode([]byte(bad)); err == nil {
			t.Fatal("invalid artifact accepted")
		}
	}
}

func TestDistributionRetainsLowScoringAcceptableSet(t *testing.T) {
	scores := []float64{1000, -1000, -1001}
	all, logAll := distribution(scores, 7)
	good, logGood := distribution(scores, 6)
	if all[0] != 1 || good[0] != 0 || good[1] < 0.73 || good[2] < 0.26 || math.IsInf(logAll-logGood, 0) || math.IsNaN(logAll-logGood) {
		t.Fatal(all, good, logAll, logGood)
	}
}

func TestBackwardMatchesFiniteDifferenceOfJointSetLoss(t *testing.T) {
	sample := fixture()
	m, _ := New(initial(17))
	var w Workspace
	if err := m.forward(sample.Inputs, sample.Masks, &w); err != nil {
		t.Fatal(err)
	}
	all, _ := distribution(w.scores[:4], 15)
	good, _ := distribution(w.scores[:4], sample.Acceptable)
	var residual [MaxChoices][2]float64
	var gradient [ParameterCount]float64
	for i, mask := range sample.Masks {
		for c := range sample.Inputs {
			residual[c][mask>>c&1] += all[i] - good[i]
		}
	}
	m.backward(sample.Inputs, &w, &residual, &gradient)
	loss := func() float64 {
		if err := m.forward(sample.Inputs, sample.Masks, &w); err != nil {
			t.Fatal(err)
		}
		_, a := distribution(w.scores[:4], 15)
		_, b := distribution(w.scores[:4], sample.Acceptable)
		return a - b
	}
	for _, i := range []int{0, 1, 256, FeatureDim * HiddenDim, w2Start, w2Start + 24, b2Start} {
		old := m.weights[i]
		m.weights[i] = old + 0.001
		up := loss()
		m.weights[i] = old - 0.001
		down := loss()
		m.weights[i] = old
		if math.Abs((up-down)/0.002-gradient[i]) > 0.0001 {
			t.Fatal("joint gradient differs", i, gradient[i], (up-down)/0.002)
		}
	}
}
