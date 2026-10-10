package flowdecision

import (
	"math"
	"sync"
	"testing"
)

func TestExplanationUsesActualForwardPassAndOwnedStorage(t *testing.T) {
	var weights [ParameterCount]float32
	weights[65] = 2
	weights[FeatureDim*HiddenDim+1] = -1
	weights[w2Start] = 3
	weights[w2Start+HiddenDim] = -2
	m, err := New(weights)
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([][FeatureDim]float32, 2)
	inputs[0][65], inputs[1][65] = .5, 1
	masks := []uint16{3, 1, 0, 2}
	var w, normal Workspace
	var p, want Prediction
	var e Explanation
	if err := m.ExplainInto(inputs, masks, &w, &p, &e); err != nil {
		t.Fatal(err)
	}
	if err := m.PredictInto(inputs, masks, &normal, &want); err != nil || p != want || w != normal {
		t.Fatal("prediction path changed", err)
	}
	if e.ChoiceCount != 2 || e.CandidateCount != 4 || e.Hidden[0][0] != 1 || e.Hidden[1][0] != 2 || e.Hidden[0][1] != 0 {
		t.Fatal(e)
	}
	if e.OptionScores[0] != [2]float32{3, -2} || e.OptionScores[1] != [2]float32{6, -4} || e.CandidateScores[0] != -6 || e.CandidateScores[2] != 9 {
		t.Fatal("actual scores", e)
	}
	w.hidden[0][0] = 99
	if e.Hidden[0][0] != 1 {
		t.Fatal("explanation aliases workspace")
	}
	var workers sync.WaitGroup
	for range 3 {
		workers.Go(func() {
			var local Workspace
			var prediction Prediction
			var explanation Explanation
			if err := m.ExplainInto(inputs, masks, &local, &prediction, &explanation); err != nil || prediction != want || explanation != e {
				t.Error(err)
			}
		})
	}
	workers.Wait()
}

func TestExplanationInvalidInputPreservesEveryOutput(t *testing.T) {
	m, _ := New([ParameterCount]float32{})
	var w Workspace
	w.hidden[0][0] = 7
	p := Prediction{Count: 9}
	e := Explanation{ChoiceCount: 11}
	wantW, wantP, wantE := w, p, e
	input := make([][FeatureDim]float32, 1)
	input[0][0] = float32(math.NaN())
	if m.ExplainInto(input, []uint16{0}, &w, &p, &e) == nil || w != wantW || p != wantP || e != wantE {
		t.Fatal("nonfinite input changed output")
	}
	input[0][0] = 0
	for _, masks := range [][]uint16{nil, {0, 0}, {2}} {
		if m.ExplainInto(input, masks, &w, &p, &e) == nil || w != wantW || p != wantP || e != wantE {
			t.Fatal("invalid masks changed output")
		}
	}
	if m.ExplainInto(input, []uint16{0}, &w, &p, nil) == nil || w != wantW || p != wantP {
		t.Fatal("nil explanation")
	}
	var absent *Model
	if absent.ExplainInto(input, []uint16{0}, &w, &p, &e) == nil || w != wantW || p != wantP || e != wantE {
		t.Fatal("nil model")
	}
}
