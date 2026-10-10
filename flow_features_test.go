package decision

import (
	"math"
	"testing"
)

func TestFlowFeaturesExactIntegerBytesAndPrefix(t *testing.T) {
	for _, value := range []int64{0, 1, -1, 9007199254740993, 9007199254740995, -9007199254740995, 18014398509481990, math.MinInt64, math.MaxInt64} {
		var prefix [ExecutionFeatureDim]float32
		prefix[12], prefix[319] = .5, .25
		flow := BranchValueFlow{Returns: [2]FlowValue{{true, "int", value}, {true, "input", 0}}}
		var out [ExecutionFlowFeatureDim]float32
		if err := ExecutionFlowFeaturesInto(prefix, flow, &out); err != nil {
			t.Fatal(err)
		}
		for i, x := range prefix {
			if out[i] != x {
				t.Fatal("v3 prefix changed")
			}
		}
		var bits uint64
		for _, x := range out[328:336] {
			bits = bits<<8 | uint64(x*2048)
		}
		if int64(bits) != value || out[320] != .125 || out[322] != .125 || out[337] != .125 {
			t.Fatal("integer lost or input role absent", value)
		}
	}
}

func TestFlowFeaturesAtomicErrorsAndUnknown(t *testing.T) {
	var out [ExecutionFlowFeatureDim]float32
	out[0] = 42
	for _, invalid := range []FlowValue{{false, "input", 0}, {true, "input", 1}, {true, "bool", 2}, {true, "invented", 0}} {
		flow := BranchValueFlow{Returns: [2]FlowValue{invalid}}
		if err := ExecutionFlowFeaturesInto([ExecutionFeatureDim]float32{}, flow, &out); err == nil || out[0] != 42 {
			t.Fatal("invalid input mutated destination")
		}
	}
	var prefix [ExecutionFeatureDim]float32
	prefix[0] = float32(math.NaN())
	if err := ExecutionFlowFeaturesInto(prefix, BranchValueFlow{}, &out); err == nil || out[0] != 42 {
		t.Fatal("nonfinite prefix accepted")
	}
	flow := BranchValueFlow{Returns: [2]FlowValue{{true, "unknown", 0}}}
	if err := ExecutionFlowFeaturesInto([ExecutionFeatureDim]float32{}, flow, &out); err != nil || out[320] != .125 || out[324] != .125 || out[336] != 0 {
		t.Fatal("unknown conflated with absent")
	}
}
