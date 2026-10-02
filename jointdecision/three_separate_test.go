package jointdecision

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

func TestSeparateArithmeticHasAnObservableRoundingBarrier(t *testing.T) {
	a := math.Float32frombits(0x3f800001)
	z := -math.Float32frombits(0x3f800002)
	fused := float32(math.FMA(float64(a), float64(a), float64(z)))
	if fused != float32(math.Ldexp(1, -46)) || addProduct32(z, a, a) != 0 {
		t.Fatal("separate/fused reference counterexample differs")
	}
	for _, shared := range []bool{false, true} {
		for _, ternary := range []bool{false, true} {
			width, hidden := ThreeFeatureDim, HiddenDim
			if shared {
				width, hidden = sharedFeatures, SharedHiddenDim
			}
			inner := &Model{arithmetic: SeparateArithmeticVersion, w1Scale: a, w2Scale: a}
			m := &ThreeModel{inner: inner, shared: shared}
			var x [ThreeFeatureDim]float32
			if ternary {
				inner.codes = make([]int8, width*hidden+HiddenDim*ThreeLabelCount)
				inner.biases = make([]float32, HiddenDim+ThreeLabelCount)
				inner.codes[0], inner.biases[0], x[0] = 1, z, a
			} else {
				inner.floatWeights = make([]float32, width*hidden+hidden+HiddenDim*ThreeLabelCount+ThreeLabelCount)
				inner.floatWeights[0], inner.floatWeights[1], x[0], x[1] = z, a, 1, a
			}
			var h [HiddenDim]float32
			m.first(&x, &h)
			if h[0] != 0 {
				t.Fatal("first layer fused", shared, ternary, h[0])
			}
			if !shared {
				var y [ThreeLabelCount]float32
				h = [HiddenDim]float32{1, a}
				if ternary {
					h[0], h[1] = a, 0
					inner.codes[width*hidden], inner.biases[HiddenDim] = 1, z
				} else {
					inner.floatWeights[width*hidden+hidden], inner.floatWeights[width*hidden+hidden+1] = z, a
				}
				m.last(&h, &y)
				if y[0] != 0 {
					t.Fatal("expanded output layer fused", ternary, y[0])
				}
			} else if !ternary {
				inner.floatWeights[sharedW1+SharedHiddenDim], inner.floatWeights[sharedW1+SharedHiddenDim+1] = z, a
				h = [HiddenDim]float32{1, a}
				var y [ThreeLabelCount]float32
				m.last(&h, &y)
				if y[0] != 0 {
					t.Fatal("compact output layer fused", y[0])
				}
			}
		}
	}
}

func TestSeparateMetadataIsExplicitClosedAndPreservedByCompaction(t *testing.T) {
	for _, feature := range []string{ThreeFeatureVersion, ThreeBagFeatureVersion} {
		for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
			name, meta, raw := threeFixture(t, variant)
			meta.Feature, meta.Arithmetic = feature, SeparateArithmeticVersion
			writeFixture(t, name, meta, raw)
			loader := LoadThree
			if feature == ThreeBagFeatureVersion {
				loader = LoadThreeBag
			}
			m, err := loader(name)
			if err != nil || m.ArithmeticVersion() != SeparateArithmeticVersion {
				t.Fatal("expanded arithmetic identity", err)
			}
			meta.Arithmetic = "unknown"
			writeFixture(t, name, meta, raw)
			if _, err = loader(name); err == nil {
				t.Fatal("unknown expanded arithmetic accepted")
			}
			expanded := tiedFixture(t, variant)
			expanded.feature, expanded.inner.arithmetic = feature, SeparateArithmeticVersion
			compact, weights, err := CompactThree(expanded)
			if err != nil || compact.Arithmetic != SeparateArithmeticVersion {
				t.Fatal("compaction lost arithmetic", err)
			}
			writeFixture(t, name, compact, weights)
			loaded, err := LoadSharedThree(name)
			if err != nil || loaded.ArithmeticVersion() != SeparateArithmeticVersion {
				t.Fatal("shared arithmetic identity", err)
			}
			compact.Arithmetic = "unknown"
			writeFixture(t, name, compact, weights)
			if _, err = LoadSharedThree(name); err == nil {
				t.Fatal("unknown shared arithmetic accepted")
			}
		}
	}
	if err := validateArithmetic(SeparateArithmeticVersion, false); err == nil {
		t.Fatal("two-choice ABI accepted an unimplemented arithmetic contract")
	}
	legacy := tiedFixture(t, "fp32")
	meta, _, err := CompactThree(legacy)
	if err != nil || legacy.ArithmeticVersion() != "" || meta.Arithmetic != "" {
		t.Fatal("legacy arithmetic changed", err)
	}
	encoded, err := json.Marshal(meta)
	if err != nil || strings.Contains(string(encoded), "arithmetic_version") {
		t.Fatal("legacy metadata gained a field", err)
	}
}

func TestSeparateExpandedAndCompactRequestsRetainExactArrays(t *testing.T) {
	text, _ := EncodeThree(threeParts(t))
	for _, feature := range []string{ThreeFeatureVersion, ThreeBagFeatureVersion} {
		for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
			expanded := tiedFixture(t, variant)
			expanded.feature, expanded.inner.arithmetic = feature, SeparateArithmeticVersion
			meta, raw, err := CompactThree(expanded)
			if err != nil {
				t.Fatal(err)
			}
			name := filepath.Join(t.TempDir(), "model.json")
			writeFixture(t, name, meta, raw)
			compact, err := LoadSharedThree(name)
			if err != nil {
				t.Fatal(err)
			}
			var a, b ThreeWorkspace
			var pa, pb ThreePrediction
			if expanded.PredictInto(text, &a, &pa) != nil || compact.PredictInto(text, &b, &pb) != nil || a != b || pa != pb {
				t.Fatal("separate expanded/compact mismatch")
			}
			if unsafe.Sizeof(b) != 3200 || testing.AllocsPerRun(100, func() {
				if compact.PredictInto(text, &b, &pb) != nil {
					panic("valid request failed")
				}
			}) != 0 {
				t.Fatal("workspace or allocation contract differs")
			}
			old, scratch := pb, b
			if compact.PredictInto("invalid", &b, &pb) == nil || old != pb || scratch != b {
				t.Fatal("invalid request changed caller arrays")
			}
			if err = os.Remove(name); err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					var w ThreeWorkspace
					var p ThreePrediction
					if compact.PredictInto(text, &w, &p) != nil || w != a || p != pa {
						t.Error("concurrent immutable model changed")
					}
				})
			}
			wg.Wait()
		}
	}
}
