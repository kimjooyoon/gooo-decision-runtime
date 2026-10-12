package contractdecision

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"
)

func TestSourceRelativeV4ArtifactSeparatesBranchABI(t *testing.T) {
	var weights [InteractionRequirementParameterCount]float32
	weights[0], weights[len(weights)-1] = .125, -.25
	m, err := NewSourceRelativeV4(weights)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeSourceRelativeV4(raw)
	if err != nil || decoded.inner.Weights() != weights || decoded.Fingerprint() != fmt.Sprintf("%x", sha256.Sum256(raw)) {
		t.Fatal("V4 tensor identity", err)
	}
	roundtrip, _ := decoded.Marshal()
	if !bytes.Equal(raw, roundtrip) {
		t.Fatal("V4 artifact changed")
	}
	v3, _ := NewSourceRelativeV3ProseNeutral(weights)
	old, _ := v3.Marshal()
	if _, err := DecodeSourceRelativeV4(old); err == nil {
		t.Fatal("old neutral artifact accepted as V4")
	}
	if _, err := DecodeSourceRelativeV3ProseNeutral(raw); err == nil {
		t.Fatal("V4 accepted by old decoder")
	}
	for _, bad := range [][]byte{nil, append(bytes.Clone(raw), []byte(" {}")...),
		append([]byte(`{"intent_policy":"duplicate",`), raw[1:]...), bytes.Repeat([]byte(" "), (512<<10)+1)} {
		if _, err := DecodeSourceRelativeV4(bad); err == nil {
			t.Fatal("invalid bounded JSON accepted")
		}
	}
	for _, mutation := range []struct{ key, value string }{
		{"branch_output_relation_features", `"wrong"`}, {"intent_policy", `null`},
		{"schema", `"wrong"`}, {"source_features", `"wrong"`}, {"weights", `[]`},
		{"weights", `null`}, {"output_relation_features", `"wrong"`},
		{"condition_features", `"wrong"`}, {"pooling", `"wrong"`}, {"unknown", `1`},
	} {
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(raw, &fields)
		fields[mutation.key] = json.RawMessage(mutation.value)
		bad, _ := json.Marshal(fields)
		if _, err := DecodeSourceRelativeV4(bad); err == nil {
			t.Fatal("invalid field accepted", mutation.key)
		}
	}
	for _, key := range []string{"weights", "branch_output_relation_features", "intent_policy"} {
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(raw, &fields)
		delete(fields, key)
		bad, _ := json.Marshal(fields)
		if _, err := DecodeSourceRelativeV4(bad); err == nil {
			t.Fatal("missing field accepted", key)
		}
	}
	nullWeight := bytes.Replace(raw, []byte(`"weights":[0.125,`), []byte(`"weights":[null,`), 1)
	if bytes.Equal(raw, nullWeight) {
		t.Fatal("null mutation did not apply")
	}
	if _, err := DecodeSourceRelativeV4(nullWeight); err == nil {
		t.Fatal("null weight accepted")
	}
	weights[0] = float32(math.Inf(1))
	if _, err := NewSourceRelativeV4(weights); err == nil {
		t.Fatal("nonfinite weight accepted")
	}
}

func TestSourceRelativeV4PreservesDestinationsOnInvalidInputs(t *testing.T) {
	sample := v2SyntheticSample()
	for i := range sample.Inputs {
		sample.Inputs[i][208], sample.Inputs[i][210] = 1, 1
		for branch := range 2 {
			sample.Inputs[i][256+branch*48], sample.Inputs[i][258+branch*48] = 1, 1
		}
	}
	m, _ := NewSourceRelativeV4([InteractionRequirementParameterCount]float32{})
	var good ChoicePrediction
	var work InteractionRequirementWorkspace
	if err := m.PredictChoicesInto(sample.Inputs, sample.Cases, sample.Conditions, &work, &good); err != nil || good.Count != 2 {
		t.Fatal("synthetic V4 prediction", err)
	}
	for _, mutation := range []struct {
		cell  int
		value float32
	}{{64, .25}, {191, 1}, {256, 0}, {258, 0}, {304, 0}, {272, 1}, {351, 1}, {352, 1}, {260, float32(math.NaN())}, {260, -1}} {
		bad := append([][528]float32(nil), sample.Inputs...)
		bad[0][mutation.cell] = mutation.value
		work = InteractionRequirementWorkspace{}
		before := ChoicePrediction{Count: 99}
		out := before
		if err := m.PredictChoicesInto(bad, sample.Cases, sample.Conditions, &work, &out); err == nil || out != before || work != (InteractionRequirementWorkspace{}) {
			t.Fatal("invalid V4 input changed output", mutation)
		}
		pred := Prediction{Count: 99, Selected: 7}
		if err := m.PredictInto(bad, sample.Cases, sample.Conditions, sample.Masks, &work, &pred); err == nil || pred.Count != 99 || pred.Selected != 7 {
			t.Fatal("invalid V4 input changed mask output")
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	options := FitOptions{Epochs: 1, LearningRate: .01}
	if model, history, err := FitSourceRelativeV4(cancelled, []InteractionRequirementSample{sample}, options); !errors.Is(err, context.Canceled) || model != nil || history != nil {
		t.Fatal("cancelled fit entered training", err)
	}
	sample.Inputs[0][64] = 1
	if model, history, err := FitSourceRelativeV4(context.Background(), []InteractionRequirementSample{sample}, options); err == nil || model != nil || history != nil {
		t.Fatal("invalid ABI entered training")
	}
}
