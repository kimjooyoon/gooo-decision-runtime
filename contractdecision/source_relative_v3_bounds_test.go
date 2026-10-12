package contractdecision

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
)

func TestSourceRelativeV3ArtifactIdentityBoundsAndVersionSeparation(t *testing.T) {
	m, err := NewSourceRelativeV3([InteractionRequirementParameterCount]float32{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeSourceRelativeV3(raw)
	if err != nil || decoded.Fingerprint() != m.Fingerprint() {
		t.Fatal("stable V3 identity", err)
	}
	for _, bad := range [][]byte{nil, append(bytes.Clone(raw), []byte(" {}")...),
		bytes.Replace(raw, []byte(SourceRelativeV3Schema), []byte(SourceRelativeV2Schema), 1),
		bytes.Replace(raw, []byte(SourceRelativeV3FeatureVersion), []byte(SourceRelativeV2FeatureVersion), 1),
		append([]byte(`{"schema":"duplicate",`), raw[1:]...),
		bytes.Replace(raw, []byte(`"weights":[0,`), []byte(`"weights":[null,`), 1)} {
		if _, err := DecodeSourceRelativeV3(bad); err == nil {
			t.Fatal("malformed or wrong-ABI V3 accepted")
		}
	}
	var weights [InteractionRequirementParameterCount]float32
	weights[0] = float32(math.Inf(1))
	if _, err := NewSourceRelativeV3(weights); err == nil {
		t.Fatal("nonfinite weights accepted")
	}
	v2, _ := NewSourceRelativeV2([InteractionRequirementParameterCount]float32{})
	old, _ := v2.Marshal()
	if _, err := DecodeSourceRelativeV3(old); err == nil {
		t.Fatal("V2 artifact silently entered V3")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "output_relation_features")
	missing, _ := json.Marshal(fields)
	if _, err := DecodeSourceRelativeV3(missing); err == nil {
		t.Fatal("missing output relation version accepted")
	}
}
