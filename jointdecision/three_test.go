package jointdecision

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"unsafe"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func threeParts(t *testing.T) [3]string {
	t.Helper()
	var parts [3]string
	for i, intent := range [3]string{"Read the first local.", "조건 분기를 교환하라.", "Assign the last local before returning."} {
		var fields [decision.SplitContextDim]byte
		fields[i], fields[5], fields[35] = 128, 128, byte(32+i*8)
		var err error
		parts[i], err = decision.EncodeSemanticContextInput(fields, intent)
		if err != nil {
			t.Fatal(err)
		}
	}
	return parts
}

func threeFixture(t *testing.T, variant string) (string, Metadata, []byte) {
	t.Helper()
	meta := Metadata{Schema: ThreeSchema, Feature: ThreeFeatureVersion, Variant: variant, FeatureDim: ThreeFeatureDim, HiddenDim: HiddenDim, MaxBytes: ThreeInputMaxBytes, Temperature: 1, WeightsFile: "weights.bin"}
	for _, label := range []string{"mask_0", "mask_1", "mask_2", "mask_3", "mask_4", "mask_5", "mask_6", "mask_7"} {
		meta.Labels = append(meta.Labels, label)
	}
	var raw []byte
	for i, name := range [4]string{"w1", "b1", "w2", "b2"} {
		rows, cols := [4]int{24, 1, 8, 1}[i], [4]int{768, 24, 24, 8}[i]
		count := rows * cols
		encoding, size := "float32_le", count*4
		if variant != "fp32" && (i == 0 || i == 2) {
			encoding, size = "ternary_base3_5", (count+4)/5
		}
		meta.Tensors = append(meta.Tensors, decision.TensorMetadata{Name: name, Rows: rows, Cols: cols, Count: count, Encoding: encoding, Offset: int64(len(raw)), Bytes: int64(size), Scale: 1})
		block := make([]byte, size)
		if encoding == "ternary_base3_5" {
			for j := range block {
				block[j] = 121
			}
		}
		if i == 3 {
			binary.LittleEndian.PutUint32(block[7*4:], math.Float32bits(2))
		}
		raw = append(raw, block...)
	}
	meta.WeightsSHA = digest(raw)
	name := filepath.Join(t.TempDir(), "model.json")
	writeFixture(t, name, meta, raw)
	return name, meta, raw
}

func TestThreeFullInputsFeaturesAndAtomicBounds(t *testing.T) {
	parts := threeParts(t)
	text, err := EncodeThree(parts)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ThreeParts(text)
	if err != nil || got != parts {
		t.Fatal("three full parts lost", err)
	}
	var output [ThreeFeatureDim]float32
	if err = FeaturesIntoThree(text, &output); err != nil {
		t.Fatal(err)
	}
	var norm float64
	for i, part := range parts {
		var single [decision.FeatureDim]float32
		if err = decision.SemanticContextFeaturesInto(part, &single); err != nil {
			t.Fatal(err)
		}
		for j, v := range single {
			if output[i*decision.FeatureDim+j] != v*float32(1/math.Sqrt(3)) {
				t.Fatal("three channels changed or reordered")
			}
		}
	}
	for _, v := range output {
		norm += float64(v * v)
	}
	if math.Abs(norm-1) > 1e-6 {
		t.Fatal("three vector norm", norm)
	}
	before := output
	two, _ := Encode([2]string{parts[0], parts[1]})
	for _, bad := range []string{"", two, text + "x", text[:len(text)-1], strings.Replace(text, "|", "|0", 1), text + strings.Repeat("x", ThreeInputMaxBytes), strings.Replace(text, ";sem64=", ";sem64=ff", 1)} {
		if FeaturesIntoThree(bad, &output) == nil || output != before {
			t.Fatal("bad three input accepted or mutated output")
		}
	}
	if FeaturesIntoThree(text, nil) == nil {
		t.Fatal("nil feature output accepted")
	}
	if n := testing.AllocsPerRun(1000, func() {
		if e := FeaturesIntoThree(text, &output); e != nil {
			panic(e)
		}
	}); n != 0 {
		t.Fatal("three projection allocates", n)
	}
	for i := range parts {
		parts[i] += strings.Repeat("x", 512-len(parts[i]))
	}
	boundary, err := EncodeThree(parts)
	if err != nil || len(boundary) != 1560 {
		t.Fatal("three complete boundary parts lost", len(boundary), err)
	}
	if FeaturesIntoThree(boundary, &output) != nil {
		t.Fatal("valid maximum parts rejected")
	}
	parts[2] += "x"
	if _, err = EncodeThree(parts); err == nil {
		t.Fatal("third individual bound ignored")
	}
	if unsafe.Sizeof(ThreeWorkspace{}) != 3200 {
		t.Fatal("three workspace layout")
	}
	if _, err = Parts(text); err == nil {
		t.Fatal("v1 accepted three-choice framing")
	}
}

func TestThreeModelVariantsZeroHeapConcurrencyAndABIBoundaries(t *testing.T) {
	text, _ := EncodeThree(threeParts(t))
	for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
		t.Run(variant, func(t *testing.T) {
			name, _, _ := threeFixture(t, variant)
			m, err := LoadThree(name)
			if err != nil {
				t.Fatal(err)
			}
			packed, resident, scales := 74624, 74624, 0
			if variant != "fp32" {
				packed, resident, scales = 3854, 18752, 8
			}
			if m.PackedFileBytes() != packed || m.ResidentTensorBytes() != resident || m.MatrixScaleBytes() != scales || m.Schema() != ThreeSchema || m.FeatureVersion() != ThreeFeatureVersion {
				t.Fatal("three tensor or metadata layout differs")
			}
			var w ThreeWorkspace
			var p ThreePrediction
			if err = m.PredictInto(text, &w, &p); err != nil || p.Mask != 7 {
				t.Fatal("eight-mask prediction", p, err)
			}
			before, scratch := p, w
			if m.PredictInto("invalid", &w, &p) == nil || p != before || w != scratch {
				t.Fatal("failed prediction mutated output")
			}
			if n := testing.AllocsPerRun(1000, func() {
				if e := m.PredictInto(text, &w, &p); e != nil {
					panic(e)
				}
			}); n != 0 {
				t.Fatal("three warmed kernel allocates", n)
			}
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					var w ThreeWorkspace
					var p ThreePrediction
					for range 20 {
						if e := m.PredictInto(text, &w, &p); e != nil || p != before {
							t.Error("concurrent caller storage differs", e)
						}
					}
				})
			}
			wg.Wait()
			if _, err = Load(name); err == nil {
				t.Fatal("v1 loader accepted new ABI")
			}
		})
	}
	name, _, _ := fixtureModel(t, "fp32")
	if _, err := LoadThree(name); err == nil {
		t.Fatal("three loader accepted v1 ABI")
	}
}

func TestThreeClosedMetadataWeightsAndFullOverflowEvidence(t *testing.T) {
	for _, kind := range []string{"dimension", "labels", "schema", "temperature", "offset", "scale", "nonfinite", "padding", "invalid_trit", "trailing_weights", "weight_bound", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			variant := "fp32"
			if kind == "scale" || kind == "padding" || kind == "invalid_trit" {
				variant = "qat_ternary"
			}
			name, meta, raw := threeFixture(t, variant)
			switch kind {
			case "dimension":
				meta.FeatureDim = 512
			case "labels":
				meta.Labels[7] = "mask_8"
			case "schema":
				meta.Schema = Schema
			case "temperature":
				meta.Temperature = 0
			case "offset":
				meta.Tensors[1].Offset++
			case "scale":
				meta.Tensors[0].Scale = 0
			case "nonfinite":
				binary.LittleEndian.PutUint32(raw, math.Float32bits(float32(math.Inf(1))))
				meta.WeightsSHA = digest(raw)
			case "padding":
				raw[meta.Tensors[0].Bytes-1] = 0
				meta.WeightsSHA = digest(raw)
			case "invalid_trit":
				raw[0] = 243
				meta.WeightsSHA = digest(raw)
			case "trailing_weights":
				raw = append(raw, 0)
				meta.WeightsSHA = digest(raw)
			case "weight_bound":
				raw = make([]byte, (128<<10)+1)
				meta.WeightsSHA = digest(raw)
			}
			writeFixture(t, name, meta, raw)
			if kind == "symlink" {
				alias := name + ".link"
				if err := os.Symlink(name, alias); err != nil {
					t.Fatal(err)
				}
				name = alias
			}
			if _, err := LoadThree(name); err == nil {
				t.Fatal("invalid three ABI accepted")
			}
		})
	}
	text, _ := EncodeThree(threeParts(t))
	next, err := FeedbackThree(text, "feedback: tried=1 passed=7/16 remaining=7")
	parts, e := ThreeParts(next)
	if err != nil || e != nil {
		t.Fatal(err, e)
	}
	original, _ := ThreeParts(text)
	for i, p := range parts {
		if !strings.HasPrefix(p, original[i]+"\nfeedback:") {
			t.Fatal("three original source or intent replaced")
		}
	}
	full, err := FeedbackThree(text, strings.Repeat("한", 170))
	if err != nil || !strings.Contains(full, strings.Repeat("한", 170)) {
		t.Fatal("overflow evidence shortened", err)
	}
	var output [ThreeFeatureDim]float32
	if FeaturesIntoThree(full, &output) == nil {
		t.Fatal("oversize three feedback inferred")
	}
}
