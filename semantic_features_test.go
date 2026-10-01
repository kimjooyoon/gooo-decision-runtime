package decision

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func semanticTestFields() [SplitContextDim]byte {
	var fields [SplitContextDim]byte
	fields[0], fields[5], fields[10], fields[44] = 128, 128, 128, 128
	fields[35] = 64
	return fields
}

func TestSemanticCodecChannelIsolationBoundariesAndZeroAllocations(t *testing.T) {
	m := &Model{metadata: Metadata{FeatureVersion: SemanticContextIntentFeatureVersion}}
	fields := semanticTestFields()
	a, err := EncodeSemanticContextInput(fields, "첫 번째 변수를 사용한다.")
	if err != nil {
		t.Fatal(err)
	}
	b, err := EncodeSemanticContextInput(fields, "Use the first variable.")
	if err != nil {
		t.Fatal(err)
	}
	fields[44], fields[45] = 0, 128
	c, err := EncodeSemanticContextInput(fields, "첫 번째 변수를 사용한다.")
	if err != nil {
		t.Fatal(err)
	}
	var first, second, third [FeatureDim]float32
	for i, text := range []string{a, b, c} {
		if err := m.FeaturesInto(text, []*[FeatureDim]float32{&first, &second, &third}[i]); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(first[:64], second[:64]) || reflect.DeepEqual(first[64:], second[64:]) ||
		!reflect.DeepEqual(first[64:], third[64:]) || reflect.DeepEqual(first[:64], third[:64]) {
		t.Fatal("source/natural channels interfere")
	}
	var norm float64
	for _, value := range first {
		norm += float64(value * value)
	}
	if math.Abs(norm-1) > 1e-6 {
		t.Fatal("semantic union norm", norm)
	}
	if allocations := testing.AllocsPerRun(1000, func() {
		if err := m.FeaturesInto(a, &first); err != nil {
			panic(err)
		}
	}); allocations != 0 {
		t.Fatal("semantic feature hot path allocated", allocations)
	}
	boundary, err := EncodeSemanticContextInput(fields, strings.Repeat("x", InputMaxBytes-semanticContextHeaderBytes))
	if err != nil || len(boundary) != InputMaxBytes {
		t.Fatal("exact byte boundary", err)
	}
	if _, err = EncodeSemanticContextInput(fields, strings.Repeat("x", InputMaxBytes-semanticContextHeaderBytes+1)); err == nil {
		t.Fatal("overflow accepted")
	}
	before := first
	for _, invalid := range []string{"plain intent", a[:semanticContextHeaderBytes], a + strings.Repeat("x", 513), strings.Replace(a, "8000", "8A00", 1), "\xff", strings.Replace(a, "8000", "ff00", 1)} {
		if err = m.FeaturesInto(invalid, &first); err == nil || first != before {
			t.Fatal("invalid semantic input accepted or modified output", invalid)
		}
	}
}

func TestSemanticFeedbackPreservesSourceHeaderAndCompleteOversizeInput(t *testing.T) {
	fields := semanticTestFields()
	initial, err := EncodeSemanticContextInput(fields, "Use the first variable.")
	if err != nil {
		t.Fatal(err)
	}
	feedback, err := SemanticContextFeedbackInput(initial, "feedback: passed=0/12 ci=FAIL")
	if err != nil {
		t.Fatal(err)
	}
	a, natural, err := decodeSemanticContext(feedback)
	if err != nil || a != fields || !strings.Contains(natural, "passed=0/12") || !strings.HasPrefix(natural, "Use the first variable.") {
		t.Fatal("feedback rewrote source/intent", err)
	}
	oversize, err := SemanticContextFeedbackInput(initial, strings.Repeat("x", 512))
	if err != nil || len(oversize) <= 512 || !strings.HasPrefix(oversize, initial) {
		t.Fatal("oversize observation truncated", err)
	}
	m := &Model{metadata: Metadata{FeatureVersion: SemanticContextIntentFeatureVersion}}
	var output [FeatureDim]float32
	if err = m.FeaturesInto(oversize, &output); err == nil {
		t.Fatal("oversized feedback inferred")
	}
}

func TestSemanticFeatureMetadataIsExplicitPathOnly(t *testing.T) {
	name, _ := writeFixtureModel(t, "fp32", 0)
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var meta Metadata
	if err = json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	meta.Schema, meta.FeatureVersion, meta.Labels = PathMetadataSchema, SemanticContextIntentFeatureVersion, append([]string(nil), pathLabels[:]...)
	raw, err = json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(name, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadPath(name); err != nil {
		t.Fatal("explicit v3 path metadata rejected", err)
	}
	if _, err = Load(name); err == nil {
		t.Fatal("path facts reinterpreted as operation model")
	}
}
