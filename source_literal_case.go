package decision

import (
	"errors"
	"math"
)

// SourceLiteralCaseFeatureVersion preserves the exact input/expected bytes and
// describes their relations to every distinct integer literal in the source
// arena. Literals are source facts, not executed candidate outputs.
const SourceLiteralCaseFeatureVersion = "source_literal_integer_case_v1"
const MaxSourceCaseLiterals = 128

// SourceLiteralCaseFeaturesInto has the same 32-cell size as declared-case v1,
// but an explicit different ABI. Bytes in cells12..27 and input/goal comparisons
// in7..9 remain. Sign, parity and successor flags are replaced by source-relative
// predicates. Sorted unique literals avoid representation-dependent repetition.
// All supplied literals are inspected; no prefix sampling or int-to-float loss.
func SourceLiteralCaseFeaturesInto(input, expected int64, literals []int64, output *[DeclaredCaseFeatureDim]float32) error {
	if output == nil || len(literals) > MaxSourceCaseLiterals {
		return errors.New("bounded source literals and destination required")
	}
	for i := 1; i < len(literals); i++ {
		if literals[i] <= literals[i-1] {
			return errors.New("source literals must be sorted and unique")
		}
	}
	var row [DeclaredCaseFeatureDim]float32
	if err := DeclaredCaseFeaturesInto(input, expected, &row); err != nil {
		return err
	}
	slots := [...]int{1, 2, 3, 4, 5, 6, 10, 11, 28, 29, 30, 31}
	for _, slot := range slots {
		row[slot] = 0
	}
	if len(literals) != 0 {
		row[1] = 1.0 / 8
	}
	var counts [11]int
	for _, literal := range literals {
		sum, sumOK := sourceCaseAdd(input, literal)
		difference, differenceOK := sourceCaseSub(input, literal)
		reverse, reverseOK := sourceCaseSub(literal, input)
		product, productOK := sourceCaseMul(input, literal)
		flags := [...]bool{input == literal, input < literal, input > literal,
			expected == literal, expected < literal, expected > literal,
			sumOK && expected == sum, differenceOK && expected == difference,
			reverseOK && expected == reverse, literal != math.MinInt64 && expected == -literal,
			productOK && expected == product}
		for i, matches := range flags {
			if matches {
				counts[i]++
			}
		}
	}
	if len(literals) != 0 {
		for i, count := range counts {
			row[slots[i+1]] = float32(count) / float32(8*len(literals))
		}
	}
	*output = row
	return nil
}

func sourceCaseAdd(a, b int64) (int64, bool) {
	if b > 0 && a > math.MaxInt64-b || b < 0 && a < math.MinInt64-b {
		return 0, false
	}
	return a + b, true
}

func sourceCaseSub(a, b int64) (int64, bool) {
	if b > 0 && a < math.MinInt64+b || b < 0 && a > math.MaxInt64+b {
		return 0, false
	}
	return a - b, true
}

func sourceCaseMul(a, b int64) (int64, bool) {
	if a == math.MinInt64 && b == -1 || b == math.MinInt64 && a == -1 {
		return 0, false
	}
	product := a * b
	if a != 0 && product/a != b {
		return 0, false
	}
	return product, true
}
