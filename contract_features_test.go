package decision

import (
	"math"
	"testing"
)

func caseFeatureInteger(features [32]float32, start int) int64 {
	var bits uint64
	for _, value := range features[start : start+8] {
		bits = bits<<8 | uint64(value*2048)
	}
	return int64(bits)
}

func TestDeclaredCaseFeaturesPreserveEveryIntegerBit(t *testing.T) {
	values := []int64{math.MinInt64, -9007199254740995, -1, 0, 1, 9007199254740993, 9007199254740995, 18014398509481990, math.MaxInt64}
	for _, input := range values {
		for _, expected := range values {
			var f [32]float32
			if err := DeclaredCaseFeaturesInto(input, expected, &f); err != nil {
				t.Fatal(err)
			}
			if caseFeatureInteger(f, 12) != input || caseFeatureInteger(f, 20) != expected || f[0] != .125 || f[31] != 0 {
				t.Fatal("exact declared values", input, expected)
			}
			if (f[7] != 0) != (input == expected) || (f[8] != 0) != (expected < input) || (f[9] != 0) != (expected > input) {
				t.Fatal("exact relation")
			}
		}
	}
	var low, high [32]float32
	_ = DeclaredCaseFeaturesInto(9007199254740993, 9007199254740995, &low)
	_ = DeclaredCaseFeaturesInto(9007199254740993, 9007199254740996, &high)
	if low == high {
		t.Fatal("one-unit goal difference lost")
	}
}

func TestDeclaredCaseBoundaryRelationsDoNotWrap(t *testing.T) {
	var f [32]float32
	_ = DeclaredCaseFeaturesInto(math.MinInt64, math.MaxInt64, &f)
	if f[28] != 0 || f[30] != 0 {
		t.Fatal("minimum boundary wrapped")
	}
	_ = DeclaredCaseFeaturesInto(math.MaxInt64, math.MinInt64, &f)
	if f[29] != 0 {
		t.Fatal("maximum boundary wrapped")
	}
	_ = DeclaredCaseFeaturesInto(-1, 1, &f)
	if f[28] != .125 || f[29] != 0 || f[30] != 0 {
		t.Fatal("negation relation")
	}
	_ = DeclaredCaseFeaturesInto(0, 1, &f)
	if f[29] != .125 {
		t.Fatal("successor relation")
	}
	_ = DeclaredCaseFeaturesInto(0, -1, &f)
	if f[30] != .125 {
		t.Fatal("predecessor relation")
	}
	if DeclaredCaseFeaturesInto(0, 0, nil) == nil {
		t.Fatal("nil destination")
	}
}
