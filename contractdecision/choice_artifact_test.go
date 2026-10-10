package contractdecision

import (
	"bytes"
	"context"
	"math"
	"testing"
)

func TestChoiceConditionedArtifactAndReaderContract(t *testing.T) {
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		m, inputs, cases := choiceFixture(t, pooling)
		raw, err := m.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := DecodeChoiceConditioned(raw)
		if err != nil || *loaded != *m || loaded.Fingerprint() != m.Fingerprint() {
			t.Fatal("artifact roundtrip", err)
		}
		if _, err := Decode(raw); err == nil {
			t.Fatal("legacy loader accepted new computation")
		}
		legacy, _ := m.base.Marshal()
		if _, err := DecodeChoiceConditioned(legacy); err == nil {
			t.Fatal("new loader accepted legacy computation")
		}
		for _, bad := range [][]byte{
			append(append([]byte{}, raw...), []byte(" {}")...),
			bytes.Replace(raw, []byte(choiceInteraction), []byte("pool_before_source"), 1),
			bytes.Replace(raw, []byte("\"architecture\":[384,32,8,384,24,2]"), []byte("\"architecture\":[384,31,8,384,24,2]"), 1),
			bytes.Replace(raw, []byte("{\"schema\":"), []byte("{\"schema\":\"duplicate\",\"schema\":"), 1),
		} {
			if _, err := DecodeChoiceConditioned(bad); err == nil {
				t.Fatal("bad artifact accepted")
			}
		}
		var w ChoiceWorkspace
		var p Prediction
		if err := loaded.PredictInto(inputs, cases, []uint16{0, 1}, &w, &p); err != nil {
			t.Fatal(err)
		}
		wantW, wantP := w, p
		if err := loaded.PredictInto(inputs, literalRows{cases}, []uint16{0, 1}, &w, &p); err == nil || w != wantW || p != wantP {
			t.Fatal("wrong reader version", err)
		}
		if model, _, err := FitChoiceConditioned(context.Background(), []Sample{{Inputs: inputs, Cases: literalRows{cases}, Masks: []uint16{0, 1}, Acceptable: 1}}, FitOptions{Epochs: 1, LearningRate: .1}, pooling); err == nil || model != nil {
			t.Fatal("wrong training case ABI")
		}
		m.context[0] = float32(math.NaN())
		if _, err := NewChoiceConditioned(m.base.weights, m.context, pooling); err == nil {
			t.Fatal("nonfinite weight")
		}
	}
}

func TestChoiceConditionedKernelAllocations(t *testing.T) {
	m, inputs, cases := choiceFixture(t, MeanPooling)
	var work ChoiceWorkspace
	var prediction ChoicePrediction
	reader := CaseSource(cases)
	count := testing.AllocsPerRun(20, func() {
		if err := m.PredictChoicesInto(inputs, reader, &work, &prediction); err != nil {
			panic(err)
		}
	})
	if count != 0 {
		t.Fatalf("prediction allocated %g objects", count)
	}
}
