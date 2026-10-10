package contractdecision

import (
	"bytes"
	"context"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestExtremePoolingKeepsRareTailAndIgnoresDuplicateWeight(t *testing.T) {
	var weights [ParameterCount]float32
	weights[0], weights[sourceWeights+FeatureDim], weights[outputWeights+HiddenDim] = 1, 1, 1
	m, err := NewForPooling(weights, ExtremePooling)
	if err != nil {
		t.Fatal(err)
	}
	input := make([][FeatureDim]float32, 1)
	cases := make(rows, 128)
	cases[127][0] = 1
	var w Workspace
	var p ChoicePrediction
	if err := m.PredictChoicesInto(input, cases, &w, &p); err != nil || w.pool[0] != 1 || w.winner[0] != 127 {
		t.Fatal("decisive tail diluted", err, w.pool)
	}
	want := p
	for _, values := range []rows{{cases[127], cases[0]}, {cases[0], cases[127]}, {cases[127], cases[127]}} {
		if err := m.PredictChoicesInto(input, values, &w, &p); err != nil || p != want {
			t.Fatal("permutation or duplicated case changed extreme scores", err)
		}
	}
	// Match ContractInput callers: box the reader once, outside the kernel.
	var reader CaseSource = cases
	if n := testing.AllocsPerRun(20, func() {
		if err := m.PredictChoicesInto(input, reader, &w, &p); err != nil {
			panic(err)
		}
	}); n != 0 {
		t.Fatal("kernel heap allocation", n)
	}
	bad := &countedCases{n: 128, fail: 127}
	oldW, oldP := w, p
	if err := m.PredictChoicesInto(input, bad, &w, &p); err == nil || bad.seen != 128 || w != oldW || p != oldP {
		t.Fatal("unread tail or non-atomic reader error")
	}
}

func TestExtremePoolingSignsAndTieOrder(t *testing.T) {
	var weights [ParameterCount]float32
	weights[0] = 1
	m, _ := NewForPooling(weights, ExtremePooling)
	input := make([][FeatureDim]float32, 1)
	for _, test := range []struct {
		cases rows
		want  float32
	}{{rows{{-300}, {1}}, -3}, {rows{{1}, {-100}}, 1}, {rows{{-100}, {1}}, 1}} {
		var w Workspace
		var p ChoicePrediction
		if err := m.PredictChoicesInto(input, test.cases, &w, &p); err != nil || w.pool[0] != test.want {
			t.Fatal("signed magnitude/tie", w.pool, err)
		}
	}
}

func TestPoolingArtifactIdentityAndLegacyBytes(t *testing.T) {
	raw, err := os.ReadFile("../studies/contract-goals-20261010/result/model.json")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := Decode(raw)
	if err != nil || legacy.Fingerprint() != "7533d7eb6889e03f6b9901be75dd66777a945a693f061b40af1f550015826254" {
		t.Fatal("legacy fingerprint changed", err)
	}
	again, err := legacy.Marshal()
	if err != nil || !bytes.Equal(again, raw) {
		t.Fatal("legacy artifact bytes changed", err)
	}
	m, _ := NewForPooling(legacy.Weights(), ExtremePooling)
	newRaw, err := m.Marshal()
	if err != nil || m.ArtifactSchema() != PoolingSchema || m.Fingerprint() == legacy.Fingerprint() {
		t.Fatal("computation identity missing", err)
	}
	loaded, err := Decode(newRaw)
	if err != nil || loaded.Fingerprint() != m.Fingerprint() || loaded.Pooling() != ExtremePooling {
		t.Fatal("extreme model round trip", err)
	}
	for _, bad := range []string{strings.Replace(string(newRaw), PoolingSchema, Schema, 1), strings.Replace(string(raw), Schema, PoolingSchema, 1), strings.Replace(string(newRaw), ExtremePooling, "maximum", 1)} {
		if _, err := Decode([]byte(bad)); err == nil {
			t.Fatal("schema/computation mismatch accepted")
		}
	}
	if _, err := NewForPooling(legacy.Weights(), "unknown"); err == nil {
		t.Fatal("unknown pooling")
	}
}

func TestExtremeTrainingAndFiniteDifference(t *testing.T) {
	samples := paired()
	m, history, err := FitForPooling(context.Background(), samples, FitOptions{Epochs: 300, LearningRate: 0.3, Seed: 17}, ExtremePooling)
	if err != nil || len(history) != 300 {
		t.Fatal(err)
	}
	for i, s := range samples {
		var w Workspace
		var p Prediction
		if err := m.PredictInto(s.Inputs, s.Cases, s.Masks, &w, &p); err != nil || p.Selected != uint16(i) {
			t.Fatal("opposite goals", i, p, err)
		}
	}
	checkExtremeGradient(t)
}

func checkExtremeGradient(t *testing.T) {
	t.Helper()
	s := paired()[0]
	s.Inputs[0][0] = 1
	s.Cases = rows{{0.7, 0.2}, {0.3, 0.8}}
	m, _ := NewForPooling(initial(17), ExtremePooling)
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
	residual[0][0], residual[0][1] = all[0]-good[0], all[1]-good[1]
	var gradient [ParameterCount]float64
	m.backward(s.Inputs, &frozen, &w, &residual, &gradient)
	for _, index := range []int{0, 1, CaseDim, caseBias, sourceWeights, sourceWeights + FeatureDim, hiddenBias, outputWeights, outputWeights + HiddenDim, outputBias} {
		checkExtremeParameter(t, m, s, &frozen, index, gradient[index], w.winner)
	}
}

func checkExtremeParameter(t *testing.T, m *Model, s Sample, cases *frozenCases, index int, gradient float64, winners [PoolDim]int) {
	t.Helper()
	loss := func() float64 {
		var w Workspace
		if err := m.forward(s.Inputs, cases, s.Masks, &w); err != nil || !reflect.DeepEqual(w.winner, winners) {
			t.Fatal("finite-difference point crossed pooling tie", err)
		}
		_, a := distribution(w.scores[:2], 3)
		_, b := distribution(w.scores[:2], 1)
		return a - b
	}
	old := m.weights[index]
	m.weights[index] = old + .0005
	plus := loss()
	m.weights[index] = old - .0005
	minus := loss()
	m.weights[index] = old
	if numeric := (plus - minus) / .001; math.Abs(numeric-gradient) > .0002 {
		t.Fatal("extreme gradient", index, gradient, numeric)
	}
}
