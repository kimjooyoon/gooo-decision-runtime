package contractdecision

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type rows [][CaseDim]float32

func (r rows) CaseCount() int                               { return len(r) }
func (r rows) CaseFeatures(i int) ([CaseDim]float32, error) { return r[i], nil }

func paired() []Sample {
	inputs := make([][FeatureDim]float32, 1)
	inputs[0][0] = 0.5
	a, b := rows{{}}, rows{{}}
	a[0][0], b[0][1] = 1, 1
	return []Sample{{inputs, a, []uint16{0, 1}, 1}, {inputs, b, []uint16{0, 1}, 2}}
}

func TestJointTrainingReadsCasesAndRoundTrips(t *testing.T) {
	samples := paired()
	o := FitOptions{Epochs: 300, LearningRate: 0.3, Seed: 17}
	m, h, err := Fit(context.Background(), samples, o)
	if err != nil || len(h) != o.Epochs || h[len(h)-1].Loss >= h[0].Loss*0.4 {
		t.Fatal(h, err)
	}
	second, again, err := Fit(context.Background(), samples, o)
	if err != nil || second.Weights() != m.Weights() || !reflect.DeepEqual(h, again) {
		t.Fatal("training differs", err)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(raw)
	if err != nil || loaded.Fingerprint() != m.Fingerprint() {
		t.Fatal("artifact binding", err)
	}
	for i, s := range samples {
		var w Workspace
		var p, q Prediction
		if err := m.PredictInto(s.Inputs, s.Cases, s.Masks, &w, &p); err != nil || p.Selected != uint16(i) {
			t.Fatal("case contract not learned", i, p, err)
		}
		if err := loaded.PredictInto(s.Inputs, s.Cases, s.Masks, &w, &q); err != nil || q != p {
			t.Fatal(err)
		}
		var choices ChoicePrediction
		if err := m.PredictChoicesInto(s.Inputs, s.Cases, &w, &choices); err != nil || choices.Logits != w.logits {
			t.Fatal(err)
		}
		if n := testing.AllocsPerRun(20, func() {
			if m.PredictInto(s.Inputs, s.Cases, s.Masks, &w, &q) != nil {
				panic("predict")
			}
		}); n != 0 {
			t.Fatal("inference allocation", n)
		}
	}
	weights := m.Weights()
	weights[0] = 99
	if m.Weights()[0] == 99 {
		t.Fatal("mutable weights")
	}
	if ParameterCount != 9746 {
		t.Fatal("documented parameter count", ParameterCount)
	}
}

type countedCases struct{ n, seen, fail int }

func (c *countedCases) CaseCount() int { return c.n }
func (c *countedCases) CaseFeatures(i int) ([CaseDim]float32, error) {
	var out [CaseDim]float32
	c.seen++
	if i == c.fail {
		return out, errors.New("case reader failed")
	}
	if i == 127 {
		out[0] = 1
	}
	return out, nil
}

func TestEveryCaseInfluencesPoolAndReaderErrorsAreAtomic(t *testing.T) {
	var weights [ParameterCount]float32
	weights[0] = 1
	weights[sourceWeights+FeatureDim] = 1
	weights[outputWeights+HiddenDim] = 1
	m, _ := New(weights)
	inputs := make([][FeatureDim]float32, 1)
	c := &countedCases{n: 128, fail: -1}
	var w Workspace
	var p Prediction
	if err := m.PredictInto(inputs, c, []uint16{0, 1}, &w, &p); err != nil || c.seen != 128 || p.Selected != 1 || w.pool[0] != 1.0/128 {
		t.Fatal("tail case ignored", c, p, err)
	}
	wantW, wantP := w, p
	c.fail = 127
	if err := m.PredictInto(inputs, c, []uint16{0, 1}, &w, &p); err == nil || w != wantW || p != wantP {
		t.Fatal("reader error changed destinations", err)
	}
}

func TestBoundsTiesNonfiniteAndConcurrentPrediction(t *testing.T) {
	m, _ := New([ParameterCount]float32{})
	inputs := make([][FeatureDim]float32, 16)
	cases := rows{{}}
	var w Workspace
	var p Prediction
	if err := m.PredictInto(inputs, cases, []uint16{9, 2, 5}, &w, &p); err != nil || p.Selected != 2 {
		t.Fatal("tie", p, err)
	}
	wantW, wantP := w, p
	for _, bad := range []struct {
		inputs [][FeatureDim]float32
		cases  CaseSource
		masks  []uint16
	}{
		{nil, cases, []uint16{0}}, {make([][FeatureDim]float32, 17), cases, []uint16{0}},
		{inputs, nil, []uint16{0}}, {inputs, rows{}, []uint16{0}}, {inputs, make(rows, 129), []uint16{0}},
		{inputs, cases, nil}, {inputs, cases, make([]uint16, 65)}, {inputs, cases, []uint16{1, 1}}, {inputs[:1], cases, []uint16{2}},
	} {
		if err := m.PredictInto(bad.inputs, bad.cases, bad.masks, &w, &p); err == nil || w != wantW || p != wantP {
			t.Fatal(err)
		}
	}
	for _, v := range []float32{float32(math.NaN()), float32(math.Inf(1))} {
		cases[0][0] = v
		if err := m.PredictInto(inputs, cases, []uint16{0}, &w, &p); err == nil || w != wantW || p != wantP {
			t.Fatal(err)
		}
	}
	cases[0][0] = 0
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			var work Workspace
			var got Prediction
			for range 10 {
				if err := m.PredictInto(inputs, cases, []uint16{9, 2, 5}, &work, &got); err != nil || got != wantP {
					t.Error(got, err)
				}
			}
		})
	}
	wg.Wait()
	weights := initial(1)
	weights[0] = float32(math.NaN())
	if _, err := New(weights); err == nil {
		t.Fatal("NaN weights")
	}
	raw, _ := m.Marshal()
	for _, bad := range []string{string(raw) + " {}", strings.Replace(string(raw), Schema, "other", 1), strings.Replace(string(raw), `"pooling":`, `"pooling":"duplicate","pooling":`, 1), strings.Replace(string(raw), pooling, "sum", 1), strings.Replace(string(raw), `"schema":`, `"unknown":0,"schema":`, 1)} {
		if _, err := Decode([]byte(bad)); err == nil {
			t.Fatal("wrong artifact accepted")
		}
	}
}

func TestTrainingErrorsAndCancellation(t *testing.T) {
	s := paired()
	valid := FitOptions{Epochs: 1, LearningRate: 0.1}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if m, _, err := Fit(ctx, s, valid); m != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, o := range []FitOptions{{}, {Epochs: 10001, LearningRate: 1}, {Epochs: 1, LearningRate: math.Inf(1)}, {Epochs: 1, LearningRate: 1, L2: -1}} {
		if m, _, err := Fit(context.Background(), s, o); err == nil || m != nil {
			t.Fatal(err)
		}
	}
	for _, bits := range []uint64{0, 4} {
		s[0].Acceptable = bits
		if m, _, err := Fit(context.Background(), s, valid); err == nil || m != nil {
			t.Fatal(err)
		}
	}
}

func TestSharedEncoderGradientMatchesFiniteDifference(t *testing.T) {
	s := paired()[0]
	s.Inputs[0][0] = 1
	s.Cases = rows{{0.7, 0.2}, {0.3, 0.8}}
	m, _ := New(initial(17))
	var frozen frozenCases
	if err := frozen.capture(s.Cases); err != nil {
		t.Fatal(err)
	}
	var w Workspace
	if err := m.forward(s.Inputs, &frozen, s.Masks, &w); err != nil {
		t.Fatal(err)
	}
	all, _ := distribution(w.scores[:2], 3)
	good, _ := distribution(w.scores[:2], 1)
	var residual [MaxChoices][2]float64
	for i := range 2 {
		residual[0][i] = all[i] - good[i]
	}
	var gradient [ParameterCount]float64
	m.backward(s.Inputs, &frozen, &w, &residual, &gradient)
	loss := func() float64 {
		var scratch Workspace
		if err := m.forward(s.Inputs, &frozen, s.Masks, &scratch); err != nil {
			t.Fatal(err)
		}
		_, a := distribution(scratch.scores[:2], 3)
		_, b := distribution(scratch.scores[:2], 1)
		return a - b
	}
	for _, i := range []int{0, 1, CaseDim, caseBias, sourceWeights, sourceWeights + FeatureDim, hiddenBias, outputWeights, outputWeights + HiddenDim, outputBias} {
		old := m.weights[i]
		m.weights[i] = old + 0.0005
		up := loss()
		m.weights[i] = old - 0.0005
		down := loss()
		m.weights[i] = old
		if math.Abs((up-down)/0.001-gradient[i]) > 0.0001 {
			t.Fatal("shared encoder gradient", i, gradient[i], (up-down)/0.001)
		}
	}
}
