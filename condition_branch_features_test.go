package decision

import (
	"math"
	"slices"
	"testing"
)

func TestConditionBranchChannelsPreserveV1AndExactObservation(t *testing.T) {
	var roles [BranchReturnRoleDim]byte
	roles[1], roles[10], roles[9], roles[19] = 128, 128, 8, 8
	for _, value := range []int64{math.MinInt64, math.MaxInt64, 9007199254740993,
		-9007199254740995, 9007199254740995, 18014398509481990} {
		feedback := conditionFeatureObservation()
		feedback.Input = value
		var v1, v2, initial [FeatureDim]float32
		if err := ConditionFeaturesInto(conditionFeatureSource(), "큰 값을 반환한다. Return the larger value.", feedback, &v1); err != nil {
			t.Fatal(err)
		}
		if err := ConditionBranchFeaturesInto(conditionFeatureSource(), roles, "큰 값을 반환한다. Return the larger value.", feedback, &v2); err != nil {
			t.Fatal(err)
		}
		if err := ConditionBranchFeaturesInto(conditionFeatureSource(), roles, "큰 값을 반환한다. Return the larger value.", ConditionFeedback{}, &initial); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(v1[:236], v2[:236]) || !slices.Equal(initial[:192], v2[:192]) || !slices.Equal(initial[236:], v2[236:]) {
			t.Fatal("independent channels changed")
		}
		var bits uint64
		for _, cell := range v2[209:217] {
			bits = bits<<8 | uint64(cell*2048)
		}
		if int64(bits) != value || v2[237] != 1 || v2[246] != 1 || v2[245] != 0.0625 || v2[255] != 0.0625 {
			t.Fatal("roles or exact integer lost")
		}
	}
}

func TestConditionBranchChannelsValidateAtomically(t *testing.T) {
	var output [FeatureDim]float32
	output[0] = 37
	want := output
	for _, invalid := range [][BranchReturnRoleDim]byte{{0: 1}, {9: 7}, {19: 136}, {10: 129}} {
		if err := ConditionBranchFeaturesInto(conditionFeatureSource(), invalid, "intent", ConditionFeedback{}, &output); err == nil || output != want {
			t.Fatal("invalid roles changed destination", err)
		}
	}
	var roles [BranchReturnRoleDim]byte
	if err := ConditionBranchFeaturesInto(conditionFeatureSource(), roles, "", ConditionFeedback{}, &output); err == nil || output != want {
		t.Fatal("invalid intent changed destination")
	}
	if err := ConditionBranchFeaturesInto(conditionFeatureSource(), roles, "intent", ConditionFeedback{}, nil); err == nil {
		t.Fatal("nil accepted")
	}
}
