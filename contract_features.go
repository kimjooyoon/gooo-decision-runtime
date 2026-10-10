package decision

import (
	"errors"
	"math"
)

// DeclaredCaseFeatureVersion encodes one caller-declared requirement, before
// execution. It carries no actual output, candidate mask or failure claim.
const DeclaredCaseFeatureVersion = "declared_integer_case_v1"
const DeclaredCaseFeatureDim = 32

// DeclaredCaseFeaturesInto preserves both int64 values as exact two's-complement
// bytes. Consumers can stream every declared case through shared model weights;
// the encoder neither selects a sample nor truncates a suite.
func DeclaredCaseFeaturesInto(input, expected int64, output *[DeclaredCaseFeatureDim]float32) error {
	if output == nil {
		return errors.New("declared case feature destination required")
	}
	var candidate [DeclaredCaseFeatureDim]float32
	flag := func(index int, value bool) {
		if value {
			candidate[index] = 1.0 / 8
		}
	}
	flag(0, true)
	for i, value := range [2]int64{input, expected} {
		flag(1+3*i, value < 0)
		flag(2+3*i, value == 0)
		flag(3+3*i, value > 0)
		for j := range 8 {
			candidate[12+8*i+j] = float32(byte(uint64(value)>>uint(56-8*j))) / 2048
		}
	}
	flag(7, expected == input)
	flag(8, expected < input)
	flag(9, expected > input)
	flag(10, input%2 == 0)
	flag(11, expected%2 == 0)
	flag(28, input != math.MinInt64 && expected == -input)
	flag(29, input != math.MaxInt64 && expected == input+1)
	flag(30, input != math.MinInt64 && expected == input-1)
	// Cell31 is reserved and remains zero in this version.
	*output = candidate
	return nil
}
