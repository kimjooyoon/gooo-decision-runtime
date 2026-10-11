package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/record"
)

// These synthetic rows do not use the frozen quality corpus.
func fixture() contractdecision.InteractionRequirementSample {
	inputs := make([][528]float32, 2)
	for i := range inputs {
		inputs[i][193], inputs[i][196+i], inputs[i][199] = 1, 1, 1
	}
	return contractdecision.InteractionRequirementSample{Inputs: inputs,
		Cases: r.Rows{{0: 1, 1: .25}}, Conditions: r.Rows{}, Masks: []uint16{0, 1, 2, 3}, Acceptable: 4}
}

func TestFixtureFitAndArtifactRoundTrip(t *testing.T) {
	s := fixture()
	m, history, err := Fit(context.Background(), []contractdecision.InteractionRequirementSample{s},
		contractdecision.FitOptions{Epochs: 2, LearningRate: .01, Seed: 9})
	if err != nil || len(history) != 2 {
		t.Fatal(err, history)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	var a, b contractdecision.ChoicePrediction
	var work contractdecision.InteractionRequirementWorkspace
	if err := m.PredictChoicesInto(s.Inputs, s.Cases, s.Conditions, &work, &a); err != nil {
		t.Fatal(err)
	}
	if err := loaded.PredictChoicesInto(s.Inputs, s.Cases, s.Conditions, &work, &b); err != nil {
		t.Fatal(err)
	}
	if a != b || a.Count != 2 || m.Fingerprint() != loaded.Fingerprint() || m.Fingerprint() != r.Hash(raw) {
		t.Fatal("round trip differs")
	}
	if _, err := contractdecision.DecodeCanonicalInteractionRequirementConditioned(raw); err == nil {
		t.Fatal("old canonical decoder accepted research artifact")
	}
	if _, err := contractdecision.DecodeInteractionRequirementConditioned(raw); err == nil {
		t.Fatal("generic decoder accepted research artifact")
	}
	old, err := m.inner.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(old); err == nil {
		t.Fatal("research decoder accepted old artifact")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, changed := range []struct{ key, value string }{
		{"weights", "[]"}, {"weights", "null"}, {"pooling", `"extreme"`},
		{"source_features", `"old"`}, {"relation_features", `"old"`}, {"unknown", "1"},
	} {
		copyFields := map[string]json.RawMessage{}
		for key, value := range fields {
			copyFields[key] = value
		}
		copyFields[changed.key] = json.RawMessage(changed.value)
		bad, _ := json.Marshal(copyFields)
		if _, err := Decode(bad); err == nil {
			t.Fatal("accepted", changed)
		}
	}
	for _, bad := range [][]byte{nil, append(append([]byte{}, raw...), []byte(" {}")...), bytes.Replace(raw, []byte(`{"schema":`), []byte(`{"schema":"duplicate","schema":`), 1)} {
		if _, err := Decode(bad); err == nil {
			t.Fatal("accepted malformed artifact")
		}
	}
}

func TestInputBoundsAndCancellation(t *testing.T) {
	s := fixture()
	for _, change := range []func(*contractdecision.InteractionRequirementSample){
		func(s *contractdecision.InteractionRequirementSample) { s.Inputs = nil },
		func(s *contractdecision.InteractionRequirementSample) { s.Conditions = nil },
		func(s *contractdecision.InteractionRequirementSample) { s.Inputs[0][200] = .5 },
		func(s *contractdecision.InteractionRequirementSample) { s.Inputs[0][208] = 1 },
		func(s *contractdecision.InteractionRequirementSample) { s.Inputs[0][192] = 1 },
		func(s *contractdecision.InteractionRequirementSample) { s.Inputs[0][193] = .5 },
		func(s *contractdecision.InteractionRequirementSample) { s.Inputs[0][196] = float32(math.NaN()) },
	} {
		bad := fixture()
		change(&bad)
		if err := validate(bad.Inputs, bad.Conditions); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Fit(ctx, []contractdecision.InteractionRequirementSample{s}, contractdecision.FitOptions{Epochs: 2, LearningRate: .01}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var nilModel *Model
	if nilModel.Fingerprint() != "" {
		t.Fatal("nil fingerprint")
	}
	if _, err := nilModel.Marshal(); err == nil {
		t.Fatal("nil marshal")
	}
	if err := nilModel.PredictChoicesInto(s.Inputs, s.Cases, s.Conditions, nil, nil); err == nil {
		t.Fatal("nil prediction")
	}
}
