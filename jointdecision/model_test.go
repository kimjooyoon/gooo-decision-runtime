package jointdecision

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime"
)

func fixtureModel(t *testing.T, variant string) (string, Metadata, []byte) {
	t.Helper()
	meta := Metadata{Schema: Schema, Feature: FeatureVersion, Variant: variant, FeatureDim: FeatureDim, HiddenDim: HiddenDim, MaxBytes: InputMaxBytes, Labels: []string{"mask_0", "mask_1", "mask_2", "mask_3"}, Temperature: 1, WeightsFile: "weights.bin"}
	var raw []byte
	for i, name := range [4]string{"w1", "b1", "w2", "b2"} {
		rows, cols := [4]int{24, 1, 4, 1}[i], [4]int{512, 24, 24, 4}[i]
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
			binary.LittleEndian.PutUint32(block[8:], math.Float32bits(2))
		}
		raw = append(raw, block...)
	}
	meta.WeightsSHA = digest(raw)
	name := filepath.Join(t.TempDir(), "model.json")
	writeFixture(t, name, meta, raw)
	return name, meta, raw
}

func writeFixture(t *testing.T, name string, meta Metadata, raw []byte) {
	t.Helper()
	encoded, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(name, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(filepath.Dir(name), "weights.bin"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestModelVariantsZeroHeapAtomicPredictionAndConcurrentArrays(t *testing.T) {
	text, _ := Encode(testParts(t))
	for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
		t.Run(variant, func(t *testing.T) {
			name, _, _ := fixtureModel(t, variant)
			model, err := Load(name)
			if err != nil {
				t.Fatal(err)
			}
			packed, resident := 49648, 49648
			if variant != "fp32" {
				packed, resident = 2590, 12496
			}
			if model.PackedFileBytes() != packed || model.ResidentTensorBytes() != resident {
				t.Fatal("weight layouts", model.PackedFileBytes(), model.ResidentTensorBytes())
			}
			var workspace Workspace
			var prediction Prediction
			if err = model.PredictInto(text, &workspace, &prediction); err != nil || prediction.Mask != 2 {
				t.Fatal("prediction", prediction, err)
			}
			before, scratch := prediction, workspace
			if err = model.PredictInto("invalid", &workspace, &prediction); err == nil || prediction != before || workspace != scratch {
				t.Fatal("failed prediction not atomic")
			}
			if n := testing.AllocsPerRun(1000, func() {
				if e := model.PredictInto(text, &workspace, &prediction); e != nil {
					panic(e)
				}
			}); n != 0 {
				t.Fatal("hot prediction allocated", n)
			}
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					var w Workspace
					var p Prediction
					for range 20 {
						if e := model.PredictInto(text, &w, &p); e != nil || p != before {
							t.Error("concurrent caller-owned prediction differs", e)
						}
					}
				})
			}
			wg.Wait()
		})
	}
}

func TestClosedMetadataAndWeightRejections(t *testing.T) {
	for _, kind := range []string{"extra_label", "missing_label", "unknown", "duplicate", "trailing", "temperature", "scale", "offset", "nonfinite", "digest", "padding", "invalid_trit", "trailing_weights", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			variant := "fp32"
			if kind == "padding" || kind == "invalid_trit" || kind == "scale" {
				variant = "qat_ternary"
			}
			name, meta, raw := fixtureModel(t, variant)
			switch kind {
			case "extra_label":
				meta.Labels = append(meta.Labels, "mask_4")
			case "missing_label":
				meta.Labels = meta.Labels[:3]
			case "temperature":
				meta.Temperature = 0
			case "scale":
				meta.Tensors[0].Scale = 0
			case "offset":
				meta.Tensors[1].Offset++
			case "nonfinite":
				binary.LittleEndian.PutUint32(raw, math.Float32bits(float32(math.Inf(1))))
				meta.WeightsSHA = digest(raw)
			case "digest":
				meta.WeightsSHA = "00"
			case "padding":
				raw[meta.Tensors[0].Bytes-1] = 0
				meta.WeightsSHA = digest(raw)
			case "invalid_trit":
				raw[0] = 243
				meta.WeightsSHA = digest(raw)
			case "trailing_weights":
				raw = append(raw, 0)
				meta.WeightsSHA = digest(raw)
			}
			writeFixture(t, name, meta, raw)
			if kind == "unknown" || kind == "duplicate" || kind == "trailing" {
				encoded, _ := os.ReadFile(name)
				if kind == "trailing" {
					encoded = append(encoded, []byte(" {}")...)
				} else {
					key := `"unknown":1,`
					if kind == "duplicate" {
						key = `"schema":"duplicate",`
					}
					encoded = append([]byte("{"+key), encoded[1:]...)
				}
				if err := os.WriteFile(name, encoded, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "symlink" {
				alias := name + ".link"
				if err := os.Symlink(name, alias); err != nil {
					t.Fatal(err)
				}
				name = alias
			}
			if _, err := Load(name); err == nil {
				t.Fatal("invalid model accepted")
			}
		})
	}
}
