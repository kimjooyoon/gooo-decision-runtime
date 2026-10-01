package decision

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func splitModel() *Model {
	return &Model{metadata: Metadata{FeatureVersion: SplitContextIntentFeatureVersion}}
}

func TestSplitChannelsAreIndependentAndBounded(t *testing.T) {
	m := splitModel()
	var a, b, c [FeatureDim]float32
	for i, text := range []string{
		"activity Choose computes return input\nintent: Use the first variable.",
		"activity Choose computes return input\nintent: 첫 번째 변수를 사용한다.",
		"activity Other computes input + 7\nintent: Use the first variable.",
	} {
		target := []*[FeatureDim]float32{&a, &b, &c}[i]
		if err := m.FeaturesInto(text, target); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(a[:64], b[:64]) || reflect.DeepEqual(a[64:], b[64:]) ||
		!reflect.DeepEqual(a[64:], c[64:]) || reflect.DeepEqual(a[:64], c[:64]) {
		t.Fatal("context and intent channels interfere")
	}
	var norm float64
	for _, value := range a {
		norm += float64(value * value)
	}
	if math.Abs(norm-1) > 1e-6 {
		t.Fatal("union is not normalized", norm)
	}
	if allocations := testing.AllocsPerRun(1000, func() {
		if err := m.FeaturesInto("gooo body\nintent: 첫 변수 갱신", &a); err != nil {
			panic(err)
		}
	}); allocations != 0 {
		t.Fatal("valid fixed-array feature encoding allocated", allocations)
	}
	for _, text := range []string{"x", "context\nintent: ", "입력", strings.Repeat("x", 512)} {
		if err := m.FeaturesInto(text, &a); err != nil {
			t.Fatal(err)
		}
		for _, value := range a {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				t.Fatal("nonfinite empty/short channel")
			}
		}
	}
	before := a
	for _, text := range []string{"", strings.Repeat("x", 513), "\xff"} {
		if err := m.FeaturesInto(text, &a); err == nil || a != before {
			t.Fatal("invalid input accepted or output modified")
		}
	}
}

func TestSplitFeatureModelMetadataIsExplicitAndPathOnly(t *testing.T) {
	filename, _ := writeFixtureModel(t, "fp32", 0.5)
	raw, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var meta Metadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	meta.FeatureVersion = SplitContextIntentFeatureVersion
	if err := validateMetadata(meta); err == nil {
		t.Fatal("operation model accepted path-only feature version")
	}
	meta.Schema = PathMetadataSchema
	labels := PathLabels()
	meta.Labels = append([]string(nil), labels[:]...)
	raw, err = json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if model, err := LoadPath(filename); err != nil || model.FeatureVersion() != SplitContextIntentFeatureVersion {
		t.Fatal("explicit split contract not loaded", err)
	}
	meta.FeatureVersion = "split_context_intent_ngrams_v999"
	if err := validateMetadataContract(meta, PathMetadataSchema, pathLabels); err == nil {
		t.Fatal("unknown future version accepted")
	}
}

func TestSplitGoPythonFeatureParity(t *testing.T) {
	filename := "testdata/split-context-features-parity.json"
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		filename = "../../studies/split-context-features-v2/parity.json"
	}
	raw, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Schema string `json:"schema"`
		Rows   []struct {
			Text     string       `json:"text"`
			Features [256]float32 `json:"features"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil || fixture.Schema != "gooo/split-context-features-parity/v1" || len(fixture.Rows) != 8 {
		t.Fatal("fixed parity fixture differs", err)
	}
	for _, row := range fixture.Rows {
		var actual [256]float32
		if err := splitModel().FeaturesInto(row.Text, &actual); err != nil {
			t.Fatal(err)
		}
		for i, value := range actual {
			if math.Abs(float64(value-row.Features[i])) > 1e-6 {
				t.Fatal("feature parity differs", i, value, row.Features[i])
			}
		}
	}
}
