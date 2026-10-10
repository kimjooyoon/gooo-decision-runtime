package decision

import (
	"math"
	"reflect"
	"testing"
)

func TestExecutionFeedbackPreservesV2AndEveryInt64Byte(t *testing.T) {
	source := conditionFeatureSource()
	var roles [BranchReturnRoleDim]byte
	roles[0], roles[19] = 128, 8
	condition := ConditionFeedback{Present: true, ChoiceCount: 2, Choice: 1, ObservedChoice: 0, CandidateMask: 1, Input: -9007199254740995, Expected: true, Reached: true}
	var prefix [FeatureDim]float32
	if err := ConditionBranchFeaturesInto(source, roles, "큰 값 선택 choose larger", condition, &prefix); err != nil {
		t.Fatal(err)
	}
	values := []int64{math.MinInt64, math.MaxInt64, 9007199254740993, 9007199254740995, -9007199254740995, 18014398509481990, 0, -1}
	for i, value := range values {
		feedback := OutputFeedback{Present: true, ChoiceCount: 2, Choice: 1, CandidateMask: 1, Input: value, Expected: values[(i+1)%len(values)], Actual: values[(i+2)%len(values)]}
		var got [ExecutionFeatureDim]float32
		if err := ExecutionFeaturesInto(source, roles, "큰 값 선택 choose larger", condition, feedback, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got[:FeatureDim], prefix[:]) {
			t.Fatal("v2 prefix changed")
		}
		for j, want := range [3]int64{feedback.Input, feedback.Expected, feedback.Actual} {
			var bits uint64
			for _, cell := range got[270+j*8 : 278+j*8] {
				bits = bits<<8 | uint64(cell*2048)
			}
			if int64(bits) != want {
				t.Fatalf("integer %d became %d", want, int64(bits))
			}
		}
		if got[256] != 0.125 || got[257] != 0.125 || got[258] != 0 || got[294] != 0.125 || got[295] != 0 {
			t.Fatal("candidate relation changed")
		}
	}
}

func TestExecutionFeedbackErrorsAreAtomicAndAbsentHasNoTail(t *testing.T) {
	var output [ExecutionFeatureDim]float32
	output[319] = 9
	want := output
	for _, f := range []OutputFeedback{{Input: 1}, {Present: true}, {Present: true, ChoiceCount: 2, Choice: 2, Expected: 1}, {Present: true, ChoiceCount: 2, CandidateMask: 4, Expected: 1}, {Present: true, ChoiceCount: 1, Expected: 2, Actual: 2}} {
		if err := ExecutionFeaturesInto(conditionFeatureSource(), [BranchReturnRoleDim]byte{}, "intent", ConditionFeedback{}, f, &output); err == nil || output != want {
			t.Fatal("invalid feedback changed destination")
		}
	}
	if err := ExecutionFeaturesInto(conditionFeatureSource(), [BranchReturnRoleDim]byte{}, "intent", ConditionFeedback{}, OutputFeedback{}, &output); err != nil {
		t.Fatal(err)
	}
	for _, cell := range output[FeatureDim:] {
		if cell != 0 {
			t.Fatal("unobserved output leaked")
		}
	}
}
