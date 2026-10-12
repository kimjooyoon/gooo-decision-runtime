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

// These API fixtures use new synthetic arrays, not historical fitted weights.
func TestProseNeutralV3ArtifactRepackagesExactTensorsAndSeparatesABI(t *testing.T) {
	var weights [InteractionRequirementParameterCount]float32
	weights[0], weights[len(weights)-1] = .125, -.25
	v3, err := NewSourceRelativeV3(weights)
	if err != nil {
		t.Fatal(err)
	}
	original, err := v3.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := RepackageSourceRelativeV3ProseNeutral(original)
	if err != nil {
		t.Fatal(err)
	}
	model, err := DecodeSourceRelativeV3ProseNeutral(raw)
	if err != nil {
		t.Fatal(err)
	}
	if model.inner.inner.Weights() != weights || model.SourceV3Fingerprint() != v3.Fingerprint() || model.Fingerprint() == v3.Fingerprint() || model.Fingerprint() != fmt.Sprintf("%x", sha256.Sum256(raw)) {
		t.Fatal("repackaging changed tensors or lost distinct artifact identity")
	}
	roundtrip, err := model.Marshal()
	if err != nil || !bytes.Equal(raw, roundtrip) {
		t.Fatal("roundtrip changed bytes", err)
	}
	direct, err := NewSourceRelativeV3ProseNeutral(weights)
	if err != nil || direct.Fingerprint() != model.Fingerprint() {
		t.Fatal("constructor identity", err)
	}
	if _, err := DecodeSourceRelativeV3(raw); err == nil {
		t.Fatal("ordinary V3 accepted neutral ABI")
	}
	if _, err := DecodeSourceRelativeV3ProseNeutral(original); err == nil {
		t.Fatal("neutral decoder accepted ordinary V3")
	}
	for _, bad := range [][]byte{nil, append(bytes.Clone(raw), []byte(" {}")...), append([]byte(`{"intent_policy":"duplicate",`), raw[1:]...), bytes.Repeat([]byte(" "), (512<<10)+1)} {
		if _, err := DecodeSourceRelativeV3ProseNeutral(bad); err == nil {
			t.Fatal("invalid bounded JSON accepted")
		}
	}
	for _, mutation := range []struct{ key, value string }{
		{"schema", `"wrong"`}, {"source_features", `"wrong"`}, {"intent_policy", `null`},
		{"source_v3_fingerprint", `"wrong"`}, {"weights", `[]`}, {"weights", `null`},
		{"output_relation_features", `"wrong"`}, {"condition_features", `"wrong"`}, {"pooling", `"wrong"`}, {"unknown", `1`},
	} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		fields[mutation.key] = json.RawMessage(mutation.value)
		bad, _ := json.Marshal(fields)
		if _, err := DecodeSourceRelativeV3ProseNeutral(bad); err == nil {
			t.Fatal("invalid artifact accepted", mutation)
		}
	}
	for _, key := range []string{"intent_policy", "source_v3_fingerprint", "weights"} {
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(raw, &fields)
		delete(fields, key)
		bad, _ := json.Marshal(fields)
		if _, err := DecodeSourceRelativeV3ProseNeutral(bad); err == nil {
			t.Fatal("missing field accepted", key)
		}
	}
	bad := bytes.Replace(raw, []byte(`"weights":[0.125,`), []byte(`"weights":[null,`), 1)
	if bytes.Equal(raw, bad) {
		t.Fatal("null mutation did not apply")
	}
	if _, err := DecodeSourceRelativeV3ProseNeutral(bad); err == nil {
		t.Fatal("null weight accepted")
	}
	weights[0] = float32(math.Inf(1))
	if _, err := NewSourceRelativeV3ProseNeutral(weights); err == nil {
		t.Fatal("nonfinite tensor accepted")
	}
	var missing *SourceRelativeV3ProseNeutralModel
	if missing.Fingerprint() != "" || missing.SourceV3Fingerprint() != "" {
		t.Fatal("nil identity")
	}
	if _, err := missing.Marshal(); err == nil {
		t.Fatal("nil artifact")
	}
}

func TestProseNeutralV3RejectsPopulatedIntentBeforePredictionOrFit(t *testing.T) {
	s := v2SyntheticSample()
	for i := range s.Inputs {
		s.Inputs[i][208], s.Inputs[i][210] = 1, 1
	}
	model, err := NewSourceRelativeV3ProseNeutral([InteractionRequirementParameterCount]float32{})
	if err != nil {
		t.Fatal(err)
	}
	var work InteractionRequirementWorkspace
	var good ChoicePrediction
	if err := model.PredictChoicesInto(s.Inputs, s.Cases, s.Conditions, &work, &good); err != nil || good.Count != 2 {
		t.Fatal("neutral synthetic prediction", err)
	}
	for _, slot := range []int{64, 191} {
		for _, value := range []float32{.25, float32(math.NaN())} {
			bad := append([][528]float32(nil), s.Inputs...)
			bad[0][slot] = value
			before := ChoicePrediction{Count: 99}
			out := before
			work = InteractionRequirementWorkspace{}
			if err := model.PredictChoicesInto(bad, s.Cases, s.Conditions, &work, &out); err == nil || out != before || work != (InteractionRequirementWorkspace{}) {
				t.Fatal("intent mismatch changed caller output", slot)
			}
			old := Prediction{Count: 9, Selected: 7}
			pred := old
			if err := model.PredictInto(bad, s.Cases, s.Conditions, s.Masks, &work, &pred); err == nil || pred != old {
				t.Fatal("complete-mask mismatch changed output")
			}
		}
	}
	options := FitOptions{Epochs: 1, LearningRate: .01}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if m, h, err := FitSourceRelativeV3ProseNeutral(cancelled, []InteractionRequirementSample{s}, options); !errors.Is(err, context.Canceled) || m != nil || h != nil {
		t.Fatal("cancelled fit entered learning", err)
	}
	s.Inputs[0][64] = 1
	if m, h, err := FitSourceRelativeV3ProseNeutral(context.Background(), []InteractionRequirementSample{s}, options); err == nil || m != nil || h != nil {
		t.Fatal("populated intent entered learning")
	}
	if m, h, err := FitSourceRelativeV3ProseNeutral(nil, []InteractionRequirementSample{s}, options); err == nil || m != nil || h != nil {
		t.Fatal("nil context entered learning")
	}
	if m, h, err := FitSourceRelativeV3ProseNeutral(context.Background(), nil, options); err == nil || m != nil || h != nil {
		t.Fatal("empty fit entered learning")
	}
}
