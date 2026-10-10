package decision

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestConditionChannelsRequireSeparatelyTrainedModelContract(t *testing.T) {
	name, _ := writeFixtureModel(t, "fp32", 0)
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var metadata Metadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	metadata.Schema, metadata.FeatureVersion = PathMetadataSchema, ConditionChannelFeatureVersion
	metadata.Labels = append([]string(nil), pathLabels[:]...)
	raw, err = json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPath(name); err == nil || !strings.Contains(err.Error(), "feature version") {
		t.Fatal("experimental features silently reused an existing model", err)
	}
}

func conditionFeatureSource() (source [SplitContextDim]byte) {
	source[2], source[5], source[17] = 128, 128, 64
	return source
}

func conditionFeatureObservation() ConditionFeedback {
	return ConditionFeedback{Present: true, ChoiceCount: 3, Choice: 1, ObservedChoice: 1,
		CandidateMask: 2, Input: -9007199254740995, Expected: true, Reached: true}
}

func TestConditionChannelsIncludeOneByteIntentAndFullCandidateMask(t *testing.T) {
	f := ConditionFeedback{Present: true, ChoiceCount: 16, Choice: 15, ObservedChoice: 15,
		CandidateMask: math.MaxUint16, Input: math.MinInt64, Expected: true, Reached: true}
	var lower, upper [FeatureDim]float32
	if err := ConditionFeaturesInto(conditionFeatureSource(), "a", f, &lower); err != nil {
		t.Fatal(err)
	}
	if err := ConditionFeaturesInto(conditionFeatureSource(), "A", f, &upper); err != nil {
		t.Fatal(err)
	}
	if lower != upper {
		t.Fatal("ASCII case folding differs")
	}
	active := false
	for _, value := range lower[64:192] {
		active = active || value != 0
	}
	if !active {
		t.Fatal("single-byte intent disappeared")
	}
	for _, value := range lower[192+25 : 192+41] {
		if value != 0.125 {
			t.Fatal("full mask bit disappeared")
		}
	}
	if lower[192+11] != 0.125 || lower[192+13] != 0.125 {
		t.Fatal("last candidate coordinate changed")
	}
}

func TestConditionChannelsKeepSourceAndFullIntentIndependent(t *testing.T) {
	source, observation := conditionFeatureSource(), conditionFeatureObservation()
	for _, intent := range []string{"음수 입력인지 비교한다. Compare whether input is negative.", strings.Repeat("a", InputMaxBytes)} {
		var absent, present, reversed [FeatureDim]float32
		if err := ConditionFeaturesInto(source, intent, ConditionFeedback{}, &absent); err != nil {
			t.Fatal(err)
		}
		if err := ConditionFeaturesInto(source, intent, observation, &present); err != nil {
			t.Fatal(err)
		}
		observation.Expected, observation.Actual = false, true
		if err := ConditionFeaturesInto(source, intent, observation, &reversed); err != nil {
			t.Fatal(err)
		}
		observation.Expected, observation.Actual = true, false
		for i := range 192 {
			if absent[i] != present[i] || absent[i] != reversed[i] {
				t.Fatalf("feedback moved original feature %d", i)
			}
		}
		for i := 192; i < FeatureDim; i++ {
			if absent[i] != 0 {
				t.Fatal("invented observation")
			}
		}
		if present[198] != 0.125 || present[200] != 0.125 || present[197] != 0 || present[201] != 0 ||
			reversed[197] != 0.125 || reversed[201] != 0.125 || reversed[198] != 0 || reversed[200] != 0 {
			t.Fatal("expected and actual Boolean roles aliased")
		}
	}
}

func TestConditionChannelsPreserveExactSignedIntegerBytes(t *testing.T) {
	for _, value := range []int64{math.MinInt64, -9007199254740995, -1, 0, 1, 9007199254740993, 9007199254740995, 18014398509481990, math.MaxInt64} {
		observation := conditionFeatureObservation()
		observation.Input = value
		var features [FeatureDim]float32
		if err := ConditionFeaturesInto(conditionFeatureSource(), "Compare the condition.", observation, &features); err != nil {
			t.Fatal(err)
		}
		var bits uint64
		for i := range 8 {
			b := features[192+17+i] * 2048
			if b != float32(byte(b)) {
				t.Fatal("integer byte was rounded", value, i, b)
			}
			bits = bits<<8 | uint64(byte(b))
		}
		if int64(bits) != value {
			t.Fatal("integer round trip changed", value, int64(bits))
		}
	}
}

func TestConditionChannelsRepresentUnreachedAndUnrelatedChoices(t *testing.T) {
	observation := conditionFeatureObservation()
	observation.Reached = false
	observation.Choice = 0
	var features [FeatureDim]float32
	if err := ConditionFeaturesInto(conditionFeatureSource(), "Observe an inner condition.", observation, &features); err != nil {
		t.Fatal(err)
	}
	f := features[192:]
	if f[1] != 0 || f[2] != 0.125 || f[3] != 0 || f[4] != 0.125 || f[7] != 0.125 || f[8] != 0 || f[9] != 0 ||
		f[10] != 0.125 || f[11] != 0 || f[12] != 0 || f[13] != 0.125 {
		t.Fatal("unobserved condition or candidate coordinate changed", f)
	}
}

func TestConditionChannelsDeclineAtomically(t *testing.T) {
	for _, edit := range []func(*ConditionFeedback){
		func(f *ConditionFeedback) { f.Present = false },
		func(f *ConditionFeedback) { f.ChoiceCount = 0 },
		func(f *ConditionFeedback) { f.ChoiceCount = 17 },
		func(f *ConditionFeedback) { f.Choice = 3 },
		func(f *ConditionFeedback) { f.ObservedChoice = 3 },
		func(f *ConditionFeedback) { f.CandidateMask = 8 },
		func(f *ConditionFeedback) { f.Reached = false; f.Actual = true },
	} {
		f := conditionFeatureObservation()
		edit(&f)
		var before [FeatureDim]float32
		before[0] = 7
		after := before
		if err := ConditionFeaturesInto(conditionFeatureSource(), "Complete intent.", f, &after); err == nil || before != after {
			t.Fatal("invalid observation changed destination", f, err)
		}
	}
	for _, text := range []string{"", string([]byte{0xff}), strings.Repeat("a", InputMaxBytes+1)} {
		var before [FeatureDim]float32
		before[2] = 5
		after := before
		if err := ConditionFeaturesInto(conditionFeatureSource(), text, ConditionFeedback{}, &after); err == nil || before != after {
			t.Fatal("invalid intent changed destination", err)
		}
	}
	var features [FeatureDim]float32
	if ConditionFeaturesInto([SplitContextDim]byte{}, "Intent.", ConditionFeedback{}, &features) == nil ||
		ConditionFeaturesInto(conditionFeatureSource(), "Intent.", ConditionFeedback{}, nil) == nil {
		t.Fatal("invalid source or nil destination accepted")
	}
}

func TestConditionChannelsAllocateNoHeapAndHaveNoSharedScratch(t *testing.T) {
	source, observation := conditionFeatureSource(), conditionFeatureObservation()
	var expected [FeatureDim]float32
	if err := ConditionFeaturesInto(source, "조건을 비교한다. Compare the condition.", observation, &expected); err != nil {
		t.Fatal(err)
	}
	var output [FeatureDim]float32
	if n := testing.AllocsPerRun(100, func() {
		_ = ConditionFeaturesInto(source, "조건을 비교한다. Compare the condition.", observation, &output)
	}); n != 0 {
		t.Fatal("valid features allocated", n)
	}
	var tasks sync.WaitGroup
	for range 8 {
		tasks.Go(func() {
			var local [FeatureDim]float32
			for range 20 {
				if err := ConditionFeaturesInto(source, "조건을 비교한다. Compare the condition.", observation, &local); err != nil || local != expected {
					t.Error("shared feature state", err)
				}
			}
		})
	}
	tasks.Wait()
}
