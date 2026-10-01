package jointdecision

import (
	"math"
	"strings"
	"testing"
	"unsafe"

	"github.com/kimjooyoon/gooo-decision-runtime"
)

func testParts(t *testing.T) [2]string {
	t.Helper()
	var fields [decision.SplitContextDim]byte
	fields[0], fields[5], fields[35] = 128, 128, 64
	var parts [2]string
	for i, intent := range [2]string{"Read the first declared local.", "원문의 조건 분기 바디를 교환하라."} {
		var err error
		parts[i], err = decision.EncodeSemanticContextInput(fields, intent)
		if err != nil {
			t.Fatal(err)
		}
	}
	return parts
}

func TestCompleteCodecFeaturesBoundsAndAtomicity(t *testing.T) {
	parts := testParts(t)
	text, err := Encode(parts)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := Parts(text)
	if err != nil || recovered != parts {
		t.Fatal("complete byte codec differs", err)
	}
	var output [FeatureDim]float32
	if err = FeaturesInto(text, &output); err != nil {
		t.Fatal(err)
	}
	var norm float64
	for i, part := range parts {
		var single [decision.FeatureDim]float32
		if err = decision.SemanticContextFeaturesInto(part, &single); err != nil {
			t.Fatal(err)
		}
		for j, v := range single {
			if output[i*decision.FeatureDim+j] != v*float32(1/math.Sqrt(2)) {
				t.Fatal("unchanged channel scaling differs")
			}
		}
	}
	for _, v := range output {
		norm += float64(v * v)
	}
	if math.Abs(norm-1) > 1e-6 {
		t.Fatal("joint normalized union", norm)
	}
	before := output
	for _, invalid := range []string{"", text + "x", strings.Replace(text, "|", "|0", 1), text[:len(text)-1], strings.Replace(text, ";sem64=", ";sem64=ff", 1), inputPrefix + "9999:x", text + strings.Repeat("x", InputMaxBytes)} {
		if err = FeaturesInto(invalid, &output); err == nil || output != before {
			t.Fatal("invalid input accepted or altered output")
		}
	}
	if err = FeaturesInto(text, nil); err == nil {
		t.Fatal("nil output accepted")
	}
	if n := testing.AllocsPerRun(1000, func() {
		if e := FeaturesInto(text, &output); e != nil {
			panic(e)
		}
	}); n != 0 {
		t.Fatal("feature allocation", n)
	}
	for i := range parts {
		parts[i] += strings.Repeat("x", 512-len(parts[i]))
	}
	boundary, err := Encode(parts)
	if err != nil || len(boundary) != len(inputPrefix)+2*(512+4) {
		t.Fatal("two full 512-byte parts lost", len(boundary), err)
	}
	if err = FeaturesInto(boundary, &output); err != nil {
		t.Fatal(err)
	}
	parts[1] += "x"
	if _, err = Encode(parts); err == nil {
		t.Fatal("individual bound ignored")
	}
	if unsafe.Sizeof(Workspace{}) != 2160 {
		t.Fatal("workspace layout")
	}
}

func TestFeedbackPreservesOriginalHeadersAndDeclinesWithoutTruncation(t *testing.T) {
	parts := testParts(t)
	text, _ := Encode(parts)
	next, err := Feedback(text, "feedback: tried=1 passed=0/16 remaining=3 ci=PASS")
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := Parts(next)
	if err != nil {
		t.Fatal(err)
	}
	for i, part := range recovered {
		if !strings.HasPrefix(part, parts[i]+"\nfeedback:") {
			t.Fatal("feedback replaced original source or intent")
		}
	}
	full, err := Feedback(text, strings.Repeat("한", 170))
	if err != nil || !strings.Contains(full, strings.Repeat("한", 170)) {
		t.Fatal("overflow evidence truncated", err)
	}
	var features [FeatureDim]float32
	if err = FeaturesInto(full, &features); err == nil {
		t.Fatal("oversize feedback inferred")
	}
}
