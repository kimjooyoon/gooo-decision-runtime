package executiondecision

import (
	"math"
	"testing"
)

func TestChoiceScoresMatchCompleteCandidateScores(t *testing.T) {
	m, _ := New(initial(17))
	s := fixture()
	var w Workspace
	var choices ChoicePrediction
	var full Prediction
	if err := m.PredictChoicesInto(s.Inputs, &w, &choices); err != nil {
		t.Fatal(err)
	}
	if err := m.PredictInto(s.Inputs, s.Masks, &w, &full); err != nil {
		t.Fatal(err)
	}
	var scores [4]float64
	for i, mask := range s.Masks {
		for c := range s.Inputs {
			scores[i] += float64(choices.Logits[c][mask>>c&1])
		}
	}
	probabilities, _ := distribution(scores[:], 15)
	for i, p := range probabilities[:4] {
		if float32(p) != full.Probabilities[i] {
			t.Fatal("different neural scores", i)
		}
	}
	if choices.Count != 2 {
		t.Fatal(choices.Count)
	}
	if n := testing.AllocsPerRun(20, func() {
		if err := m.PredictChoicesInto(s.Inputs, &w, &choices); err != nil {
			panic(err)
		}
	}); n != 0 {
		t.Fatal(n)
	}
	want, pred := w, choices
	s.Inputs[0][0] = float32(math.NaN())
	if err := m.PredictChoicesInto(s.Inputs, &w, &choices); err == nil || w != want || choices != pred {
		t.Fatal("invalid input changed arrays")
	}
	raw, _ := m.Marshal()
	other, err := Decode(append([]byte("\n"), raw...))
	if err != nil || m.Fingerprint() != other.Fingerprint() {
		t.Fatal("formatting changes model identity", err)
	}
	weights := m.Weights()
	weights[0] += 1
	different, _ := New(weights)
	if different.Fingerprint() == m.Fingerprint() || len(m.Fingerprint()) != 64 {
		t.Fatal("weight identity did not change")
	}
}
