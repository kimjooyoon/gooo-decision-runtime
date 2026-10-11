package contractdecision

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
)

func TestSourceRelativeV2ArtifactHasSeparateIdentityAndExactRoundTrip(t *testing.T) {
	var weights [InteractionRequirementParameterCount]float32
	weights[0], weights[len(weights)-1] = .125, -.25
	model, err := NewSourceRelativeV2(weights)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := model.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := DecodeSourceRelativeV2(raw)
	if err != nil {
		t.Fatal(err)
	}
	other, err := loaded.Marshal()
	if err != nil || string(raw) != string(other) {
		t.Fatal("V2 roundtrip changed bytes", err)
	}
	if model.Fingerprint() != fmt.Sprintf("%x", sha256.Sum256(raw)) || loaded.Fingerprint() != model.Fingerprint() {
		t.Fatal("V2 fingerprint differs")
	}
	if _, err := DecodeSourceRelativeRequirementConditioned(raw); err == nil {
		t.Fatal("V1 accepted V2 weights")
	}
	old, err := NewSourceRelativeRequirementConditioned(weights)
	if err != nil {
		t.Fatal(err)
	}
	v1, err := old.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSourceRelativeV2(v1); err == nil {
		t.Fatal("V2 accepted V1 weights")
	}
	for _, mutate := range []func(map[string]any){
		func(m map[string]any) { m["weights"] = []any{0} },
		func(m map[string]any) { m["weights"].([]any)[0] = nil },
		func(m map[string]any) { m["source_features"] = SourceRelativeOrderedSourceFeatureVersion },
		func(m map[string]any) { m["pooling"] = "extreme" },
		func(m map[string]any) { m["extra"] = 1 },
	} {
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		mutate(fields)
		bad, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeSourceRelativeV2(bad); err == nil {
			t.Fatal("invalid V2 artifact accepted")
		}
	}
}

func TestSourceRelativeV2InputFlagsValidateBeforeInference(t *testing.T) {
	sample := sourceRelativeFixture()
	for i := range sample.Inputs {
		for expression := range 3 {
			sample.Inputs[i][384+48*expression] = 1.0 / 8
		}
	}
	sample.Inputs[0][384+48] = 0
	sample.Inputs[0][384+48+9] = 1.0 / 8
	if err := validateSourceRelativeV2(sample.Inputs, sample.Conditions); err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{384 + 48 + 10, 384 + 48 + 1} {
		before := sample.Inputs[0][index]
		sample.Inputs[0][index] = 1.0 / 8
		if err := validateSourceRelativeV2(sample.Inputs, sample.Conditions); err == nil {
			t.Fatal("invalid V2 flags accepted")
		}
		sample.Inputs[0][index] = before
	}
}
