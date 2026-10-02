package jointdecision

import (
	"path/filepath"
	"testing"
	"unsafe"
)

func TestBagThreeExplicitLoaderAndPreservedLegacyContract(t *testing.T) {
	text, err := EncodeThree(threeParts(t))
	if err != nil {
		t.Fatal(err)
	}
	name, meta, raw := threeFixture(t, "fp32")
	if _, err = LoadThreeBag(name); err == nil {
		t.Fatal("v4 loader accepted v3 metadata")
	}
	meta.Feature = ThreeBagFeatureVersion
	writeFixture(t, name, meta, raw)
	if _, err = LoadThree(name); err == nil {
		t.Fatal("frozen expanded-v3 loader accepted v4")
	}
	model, err := LoadThreeBag(name)
	if err != nil {
		t.Fatal(err)
	}
	var w ThreeWorkspace
	var p ThreePrediction
	if err = model.PredictInto(text, &w, &p); err != nil {
		t.Fatal(err)
	}
	var want [ThreeFeatureDim]float32
	if err = FeaturesIntoThreeBag(text, &want); err != nil || w.Features != want {
		t.Fatal("explicit v4 projection", err)
	}
	if model.FeatureVersion() != ThreeBagFeatureVersion || unsafe.Sizeof(w) != 3200 {
		t.Fatal("feature identity/workspace changed")
	}
	meta.Feature = "triple_semantic_context_bag_v5_joint_v1"
	writeFixture(t, name, meta, raw)
	if _, err = LoadThreeBag(name); err == nil {
		t.Fatal("future feature version accepted")
	}
}

func TestBagThreeCompactPredictionParityAndFailureAtomicity(t *testing.T) {
	text, err := EncodeThree(threeParts(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
		t.Run(variant, func(t *testing.T) {
			expanded := tiedFixture(t, variant)
			expanded.feature = ThreeBagFeatureVersion
			meta, raw, err := CompactThree(expanded)
			if err != nil || meta.Feature != ThreeBagFeatureVersion {
				t.Fatal("compaction lost feature contract", err)
			}
			name := filepath.Join(t.TempDir(), "model.json")
			writeFixture(t, name, meta, raw)
			model, err := LoadSharedThree(name)
			if err != nil {
				t.Fatal(err)
			}
			var a, b ThreeWorkspace
			var pa, pb ThreePrediction
			if err = expanded.PredictInto(text, &a, &pa); err != nil {
				t.Fatal(err)
			}
			if err = model.PredictInto(text, &b, &pb); err != nil {
				t.Fatal(err)
			}
			if a != b || pa != pb || model.FeatureVersion() != ThreeBagFeatureVersion {
				t.Fatal("compact v4 changed arithmetic/identity")
			}
			if n := testing.AllocsPerRun(100, func() {
				if err := model.PredictInto(text, &b, &pb); err != nil {
					panic(err)
				}
			}); n != 0 {
				t.Fatal("v4 inference allocates", n)
			}
			before, old := b, pb
			for _, bad := range []string{"", text + "x", text[:len(text)-1]} {
				if model.PredictInto(bad, &b, &pb) == nil || b != before || pb != old {
					t.Fatal("invalid v4 inference changed output")
				}
			}
			meta.Feature = "unknown"
			writeFixture(t, name, meta, raw)
			if _, err = LoadSharedThree(name); err == nil {
				t.Fatal("shared unknown feature accepted")
			}
		})
	}
}
