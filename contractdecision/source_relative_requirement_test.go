package contractdecision

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"testing"
)

// These synthetic rows do not use the frozen quality corpus.
func sourceRelativeFixture() InteractionRequirementSample {
	inputs := make([][528]float32, 2)
	for i := range inputs {
		inputs[i][193], inputs[i][196+i], inputs[i][199] = 1, 1, 1
	}
	return InteractionRequirementSample{Inputs: inputs,
		Cases: rows{{0: 1, 1: .25}}, Conditions: conditionRows{}, Masks: []uint16{0, 1, 2, 3}, Acceptable: 4}
}

func TestSourceRelativeFixtureFitAndArtifactRoundTrip(t *testing.T) {
	s := sourceRelativeFixture()
	m, history, err := FitSourceRelativeRequirementConditioned(context.Background(), []InteractionRequirementSample{s},
		FitOptions{Epochs: 2, LearningRate: .01, Seed: 9})
	if err != nil || len(history) != 2 {
		t.Fatal(err, history)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := DecodeSourceRelativeRequirementConditioned(raw)
	if err != nil {
		t.Fatal(err)
	}
	var a, b ChoicePrediction
	var work InteractionRequirementWorkspace
	if err := m.PredictChoicesInto(s.Inputs, s.Cases, s.Conditions, &work, &a); err != nil {
		t.Fatal(err)
	}
	if err := loaded.PredictChoicesInto(s.Inputs, s.Cases, s.Conditions, &work, &b); err != nil {
		t.Fatal(err)
	}
	if a != b || a.Count != 2 || m.Fingerprint() != loaded.Fingerprint() || m.Fingerprint() != fmt.Sprintf("%x", sha256.Sum256(raw)) {
		t.Fatal("round trip differs")
	}
	var ranked, reloaded Prediction
	if err := m.PredictInto(s.Inputs, s.Cases, s.Conditions, s.Masks, &work, &ranked); err != nil {
		t.Fatal(err)
	}
	if err := loaded.PredictInto(s.Inputs, s.Cases, s.Conditions, s.Masks, &work, &reloaded); err != nil {
		t.Fatal(err)
	}
	if ranked != reloaded || ranked.Count != 4 || ranked.Selected > 3 {
		t.Fatal("complete-mask prediction changed after reload")
	}
	if _, err := DecodeCanonicalInteractionRequirementConditioned(raw); err == nil {
		t.Fatal("old canonical decoder accepted research artifact")
	}
	if _, err := DecodeInteractionRequirementConditioned(raw); err == nil {
		t.Fatal("generic decoder accepted research artifact")
	}
	old, err := m.inner.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSourceRelativeRequirementConditioned(old); err == nil {
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
		if _, err := DecodeSourceRelativeRequirementConditioned(bad); err == nil {
			t.Fatal("accepted", changed)
		}
	}
	for _, bad := range [][]byte{nil, append(append([]byte{}, raw...), []byte(" {}")...), bytes.Replace(raw, []byte(`{"schema":`), []byte(`{"schema":"duplicate","schema":`), 1)} {
		if _, err := DecodeSourceRelativeRequirementConditioned(bad); err == nil {
			t.Fatal("accepted malformed artifact")
		}
	}
	var nullable []json.RawMessage
	if err := json.Unmarshal(fields["weights"], &nullable); err != nil {
		t.Fatal(err)
	}
	nullable[0] = json.RawMessage("null")
	fields["weights"], _ = json.Marshal(nullable)
	bad, _ := json.Marshal(fields)
	if _, err := DecodeSourceRelativeRequirementConditioned(bad); err == nil {
		t.Fatal("null silently became a zero weight")
	}
}

func TestSourceRelativeInputBoundsAndCancellation(t *testing.T) {
	s := sourceRelativeFixture()
	for _, change := range []func(*InteractionRequirementSample){
		func(s *InteractionRequirementSample) { s.Inputs = nil },
		func(s *InteractionRequirementSample) { s.Conditions = nil },
		func(s *InteractionRequirementSample) { s.Inputs[0][200] = .5 },
		func(s *InteractionRequirementSample) { s.Inputs[0][208] = 1 },
		func(s *InteractionRequirementSample) { s.Inputs[0][192] = 1 },
		func(s *InteractionRequirementSample) { s.Inputs[0][193] = .5 },
		func(s *InteractionRequirementSample) { s.Inputs[0][196] = float32(math.NaN()) },
	} {
		bad := sourceRelativeFixture()
		change(&bad)
		if err := validateSourceRelativeConditions(bad.Inputs, bad.Conditions); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := FitSourceRelativeRequirementConditioned(ctx, []InteractionRequirementSample{s}, FitOptions{Epochs: 2, LearningRate: .01}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var nilModel *SourceRelativeRequirementModel
	if nilModel.Fingerprint() != "" {
		t.Fatal("nil fingerprint")
	}
	if _, err := nilModel.Marshal(); err == nil {
		t.Fatal("nil marshal")
	}
	if err := nilModel.PredictChoicesInto(s.Inputs, s.Cases, s.Conditions, nil, nil); err == nil {
		t.Fatal("nil prediction")
	}
	if err := nilModel.PredictInto(s.Inputs, s.Cases, s.Conditions, s.Masks, nil, nil); err == nil {
		t.Fatal("nil complete-mask prediction")
	}
	var invalid [InteractionRequirementParameterCount]float32
	invalid[0] = float32(math.Inf(1))
	if _, err := NewSourceRelativeRequirementConditioned(invalid); err == nil {
		t.Fatal("nonfinite constructor weights")
	}
}

// Compatibility observation only: no prediction or fitting on the study corpus.
func TestSourceRelativePublishedArtifactIdentity(t *testing.T) {
	raw, err := os.ReadFile("../studies/source-relative-learning-20261011/result/model-relative.json")
	if err != nil {
		t.Fatal(err)
	}
	model, err := DecodeSourceRelativeRequirementConditioned(raw)
	if err != nil {
		t.Fatal(err)
	}
	next, err := model.Marshal()
	if err != nil || !bytes.Equal(raw, next) || model.Fingerprint() != "cae32bdad17ab818498c6099066918cd9c9c591eb118afe1bc6a7ef82dc83cc4" {
		t.Fatal("published weight identity changed", err)
	}
	if model.ArtifactSchema() != SourceRelativeRequirementSchema || model.SourceFeatureVersion() != SourceRelativeOrderedSourceFeatureVersion || model.Pooling() != MeanPooling {
		t.Fatal("published computation contract changed")
	}
}
