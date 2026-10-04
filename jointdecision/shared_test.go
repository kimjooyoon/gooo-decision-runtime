package jointdecision

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime"
)

// Construct every expanded tensor independently of the compactor's indexing.
func tiedFixture(t *testing.T, variant string) *ThreeModel {
	t.Helper()
	meta := Metadata{Schema: ThreeSchema, Feature: ThreeFeatureVersion, Variant: variant, FeatureDim: 768, HiddenDim: 24, MaxBytes: 1600, Temperature: .5, WeightsFile: "weights.bin"}
	for i := range 8 {
		meta.Labels = append(meta.Labels, "mask_"+string(rune('0'+i)))
	}
	var raw []byte
	for tensor, name := range [4]string{"w1", "b1", "w2", "b2"} {
		rows, cols := [4]int{24, 1, 8, 1}[tensor], [4]int{768, 24, 24, 8}[tensor]
		values := make([]float32, rows*cols)
		for row := range rows {
			for col := range cols {
				v := float32(0)
				switch tensor {
				case 0:
					if row/8 == col/256 {
						v = float32((row%8*13+col%256*7)%3 - 1)
					}
				case 1:
					v = float32(col%8-3) / 8
				case 2:
					v = float32((((row>>(col/8))&1)*11+(col%8)*3)%3 - 1)
				}
				values[row*cols+col] = v
			}
		}
		encoding, scale := "float32_le", float64(1)
		var block []byte
		if variant != "fp32" && (tensor == 0 || tensor == 2) {
			encoding = "ternary_base3_5"
			scale = .125
			if tensor == 2 {
				scale = .75
			}
			block = make([]byte, (len(values)+4)/5)
			for j := range block {
				power := 1
				for k := range 5 {
					digit := 1
					if j*5+k < len(values) {
						digit = int(values[j*5+k]) + 1
					}
					block[j] += byte(digit * power)
					power *= 3
				}
			}
		} else {
			block = make([]byte, len(values)*4)
			for j, v := range values {
				binary.LittleEndian.PutUint32(block[4*j:], math.Float32bits(v))
			}
		}
		meta.Tensors = append(meta.Tensors, decision.TensorMetadata{Name: name, Rows: rows, Cols: cols, Count: len(values), Encoding: encoding, Offset: int64(len(raw)), Bytes: int64(len(block)), Scale: scale})
		raw = append(raw, block...)
	}
	meta.WeightsSHA = digest(raw)
	name := filepath.Join(t.TempDir(), "model.json")
	writeFixture(t, name, meta, raw)
	m, err := LoadThree(name)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func compactFixture(t *testing.T, variant string) (*ThreeModel, string, Metadata, []byte) {
	t.Helper()
	expanded := tiedFixture(t, variant)
	meta, raw, err := CompactThree(expanded)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "model.json")
	writeFixture(t, name, meta, raw)
	return expanded, name, meta, raw
}

func equalBits(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if math.Float32bits(v) != math.Float32bits(b[i]) {
			return false
		}
	}
	return true
}

func TestSharedThreeExactArithmeticStorageAndConcurrentPrediction(t *testing.T) {
	text, err := EncodeThree(threeParts(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
		t.Run(variant, func(t *testing.T) {
			expanded, name, _, _ := compactFixture(t, variant)
			m, e := LoadSharedThree(name)
			if e != nil {
				t.Fatal(e)
			}
			packed, resident, scales := 8288, 8288, 0
			if variant != "fp32" {
				packed, resident, scales = 446, 2096, 8
			}
			if m.Schema() != SharedThreeSchema || m.PackedFileBytes() != packed || m.ResidentTensorBytes() != resident || m.MatrixScaleBytes() != scales || m.MetadataSHA256() == expanded.MetadataSHA256() || m.WeightsSHA256() == expanded.WeightsSHA256() {
				t.Fatal("storage/identity differs")
			}
			var a, b ThreeWorkspace
			var pa, pb ThreePrediction
			if e = expanded.PredictInto(text, &a, &pa); e != nil {
				t.Fatal(e)
			}
			if e = m.PredictInto(text, &b, &pb); e != nil {
				t.Fatal(e)
			}
			if !equalBits(a.Features[:], b.Features[:]) || !equalBits(a.Hidden[:], b.Hidden[:]) || !equalBits(pa.Logits[:], pb.Logits[:]) || !equalBits(pa.Probabilities[:], pb.Probabilities[:]) || pa.Mask != pb.Mask {
				t.Fatal("expanded arithmetic differs")
			}
			if n := testing.AllocsPerRun(100, func() {
				if e := m.PredictInto(text, &b, &pb); e != nil {
					panic(e)
				}
			}); n != 0 {
				t.Fatal("valid kernel allocates", n)
			}
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					var w ThreeWorkspace
					var p ThreePrediction
					for range 10 {
						if e := m.PredictInto(text, &w, &p); e != nil || !equalBits(p.Logits[:], pa.Logits[:]) {
							t.Error("concurrent prediction differs", e)
						}
					}
				})
			}
			wg.Wait()
			before, old := b, pb
			for _, bad := range []string{"", text + "x", strings.Repeat("x", 1601)} {
				if m.PredictInto(bad, &b, &pb) == nil || b != before || pb != old {
					t.Fatal("invalid prediction changed caller storage")
				}
			}
			if _, e := LoadThree(name); e == nil {
				t.Fatal("old ABI accepted compact")
			}
			if _, e := Load(name); e == nil {
				t.Fatal("two ABI accepted compact")
			}
			if _, _, e := CompactThree(m); e == nil {
				t.Fatal("compact claimed expanded identity")
			}
		})
	}
}

func TestSharedThreeRejectsUntiedOrNoncanonicalExpandedModels(t *testing.T) {
	for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
		for _, kind := range []string{"offblock", "diagonal", "bias", "output", "output_bias", "negative_zero"} {
			t.Run(variant+"/"+kind, func(t *testing.T) {
				m := tiedFixture(t, variant)
				switch kind {
				case "offblock":
					if variant == "fp32" {
						m.inner.floatWeights[256] = 1
					} else {
						m.inner.codes[256] = 1
					}
				case "diagonal":
					if variant == "fp32" {
						m.inner.floatWeights[8*768+256] = .5
					} else {
						m.inner.codes[8*768+256] = 0
					}
				case "bias":
					if variant == "fp32" {
						m.inner.floatWeights[768*24+8] = .5
					} else {
						m.inner.biases[8] = .5
					}
				case "output":
					if variant == "fp32" {
						m.inner.floatWeights[768*24+24+2*24+8] = .5
					} else {
						m.inner.codes[768*24+2*24+8] = 0
					}
				case "output_bias":
					if variant == "fp32" {
						m.inner.floatWeights[len(m.inner.floatWeights)-1] = .5
					} else {
						m.inner.biases[31] = .5
					}
				case "negative_zero":
					if variant == "fp32" {
						m.inner.floatWeights[256] = math.Float32frombits(1 << 31)
					} else {
						m.inner.biases[31] = math.Float32frombits(1 << 31)
					}
				}
				if _, raw, e := CompactThree(m); e == nil || raw != nil {
					t.Fatal("untied model exported")
				}
			})
		}
	}
	if _, _, e := CompactThree(nil); e == nil {
		t.Fatal("nil accepted")
	}
}

func TestSharedThreeStrictLoading(t *testing.T) {
	for _, kind := range []string{"schema", "feature", "hidden", "labels", "tensor_count", "variant", "filename", "temperature", "underflow_temperature", "scale", "offset", "rows", "encoding", "digest", "nonfinite", "trailing_weights", "padding", "invalid_trit", "unknown", "duplicate", "trailing_json", "symlink", "metadata_symlink", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			_, name, meta, raw := compactFixture(t, "qat_ternary")
			switch kind {
			case "schema":
				meta.Schema = ThreeSchema
			case "feature":
				meta.Feature = "unknown"
			case "hidden":
				meta.HiddenDim = 24
			case "labels":
				meta.Labels[7] = "other"
			case "tensor_count":
				meta.Tensors = append(meta.Tensors, meta.Tensors[0])
			case "variant":
				meta.Variant = "int8"
			case "filename":
				meta.WeightsFile = "../weights.bin"
			case "temperature":
				meta.Temperature = 0
			case "underflow_temperature":
				meta.Temperature = 1e-200
			case "scale":
				meta.Tensors[0].Scale = 1e-200
			case "offset":
				meta.Tensors[1].Offset++
			case "rows":
				meta.Tensors[0].Rows = 24
			case "encoding":
				meta.Tensors[0].Encoding = "float32_le"
			case "digest":
				meta.WeightsSHA = "00"
			case "nonfinite":
				binary.LittleEndian.PutUint32(raw[410:], 0x7f800000)
				meta.WeightsSHA = digest(raw)
			case "trailing_weights":
				raw = append(raw, 0)
				meta.WeightsSHA = digest(raw)
			case "padding":
				raw[409] = 0
				meta.WeightsSHA = digest(raw)
			case "invalid_trit":
				raw[0] = 243
				meta.WeightsSHA = digest(raw)
			case "oversize":
				raw = make([]byte, 16385)
				meta.WeightsSHA = digest(raw)
			}
			writeFixture(t, name, meta, raw)
			if kind == "unknown" || kind == "duplicate" || kind == "trailing_json" {
				b, e := json.Marshal(meta)
				if e != nil {
					t.Fatal(e)
				}
				switch kind {
				case "unknown":
					b = append([]byte(`{"extra":1,`), b[1:]...)
				case "duplicate":
					b = append([]byte(`{"schema":"x",`), b[1:]...)
				case "trailing_json":
					b = append(b, []byte(` {}`)...)
				}
				if e = os.WriteFile(name, b, 0600); e != nil {
					t.Fatal(e)
				}
			}
			if kind == "symlink" || kind == "metadata_symlink" {
				p := filepath.Join(filepath.Dir(name), "weights.bin")
				if kind == "metadata_symlink" {
					p = name
				}
				if e := os.Rename(p, p+".actual"); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(p+".actual", p); e != nil {
					t.Fatal(e)
				}
			}
			if _, e := LoadSharedThree(name); e == nil {
				t.Fatal("invalid compact model accepted")
			}
		})
	}
}

func TestSharedThreeNilAndOverflowAreAtomic(t *testing.T) {
	_, name, _, _ := compactFixture(t, "fp32")
	m, e := LoadSharedThree(name)
	if e != nil {
		t.Fatal(e)
	}
	text, _ := EncodeThree(threeParts(t))
	var w ThreeWorkspace
	var p ThreePrediction
	if e = m.PredictInto(text, &w, &p); e != nil {
		t.Fatal(e)
	}
	before, old := w, p
	for _, model := range []*ThreeModel{nil, {}} {
		if model.PredictInto(text, &w, &p) == nil || w != before || p != old {
			t.Fatal("nil model changed output")
		}
	}
	if m.PredictInto(text, nil, &p) == nil || m.PredictInto(text, &w, nil) == nil {
		t.Fatal("nil caller storage accepted")
	}
	for i := range m.inner.floatWeights {
		m.inner.floatWeights[i] = math.MaxFloat32
	}
	if m.PredictInto(text, &w, &p) == nil || w != before || p != old {
		t.Fatal("numerical failure changed output")
	}
}
