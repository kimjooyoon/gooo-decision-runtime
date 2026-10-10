package contractdecision

import (
	"encoding/json"
	"math"
	"testing"
)

func TestOrderedRequirementArtifactBindsBothChannelsAndComputation(t *testing.T) {
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		model, inputs, outputs, conditions := orderedRequirementFixture(t, pooling)
		raw, err := model.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := DecodeOrderedRequirementConditioned(raw)
		if err != nil || loaded.Fingerprint() != model.Fingerprint() || loaded.Weights() != model.Weights() {
			t.Fatal(err)
		}
		var a, b OrderedRequirementWorkspace
		var first, second Prediction
		if model.PredictInto(inputs, outputs, conditions, []uint16{0, 1, 7}, &a, &first) != nil ||
			loaded.PredictInto(inputs, outputs, conditions, []uint16{0, 1, 7}, &b, &second) != nil || first != second {
			t.Fatal("artifact changed computation")
		}
		for _, bad := range [][]byte{nil, append(append([]byte(nil), raw...), []byte(" {}")...), append([]byte(`{"schema":"duplicate",`), raw[1:]...)} {
			if _, err := DecodeOrderedRequirementConditioned(bad); err == nil {
				t.Fatal("malformed artifact accepted")
			}
		}
		for _, field := range []string{"schema", "source_feature_version", "case_feature_version", "condition_feature_version",
			"architecture", "bounds", "interaction", "empty_conditions", "activation", "pooling", "weights_fp32", "unknown"} {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			fields[field] = json.RawMessage(`"different"`)
			bad, _ := json.Marshal(fields)
			if _, err := DecodeOrderedRequirementConditioned(bad); err == nil {
				t.Fatal("ABI change accepted", field)
			}
		}
		if _, err := DecodeChoiceConditioned(raw); err == nil {
			t.Fatal("new model interpreted as legacy choice model")
		}
		if _, err := DecodeRequirementConditioned(raw); err == nil {
			t.Fatal("ordered source interpreted through the old requirement ABI")
		}
		if _, err := Decode(raw); err == nil {
			t.Fatal("new model interpreted as legacy contract model")
		}
	}
	var weights [OrderedRequirementParameterCount]float32
	weights[0] = float32(math.NaN())
	if model, err := NewOrderedRequirementConditioned(weights, MeanPooling); err == nil || model != nil {
		t.Fatal("nonfinite model accepted")
	}
	if raw, err := (*OrderedRequirementModel)(nil).Marshal(); err == nil || raw != nil {
		t.Fatal("nil model artifact")
	}
}
