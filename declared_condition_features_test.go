package decision

import (
	"math"
	"testing"
)

func TestDeclaredConditionFeaturesPreserveEveryIntegerBitAndTarget(t *testing.T) {
	values := []int64{math.MinInt64, -9007199254740995, -1, 0, 1, 9007199254740992, 9007199254740993, math.MaxInt64}
	for _, value := range values {
		for target := range 16 {
			input := DeclaredConditionFeatureInput{Input: value, Expected: true, TargetChoice: target,
				ChoiceCount: 16, Index: 127, Count: 128}
			var row [DeclaredConditionFeatureDim]float32
			if err := DeclaredConditionFeaturesInto(input, &row); err != nil {
				t.Fatal(err)
			}
			var bits uint64
			for _, cell := range row[5:13] {
				bits = bits<<8 | uint64(cell*2048)
			}
			if int64(bits) != value || row[0] != 1.0/8 || row[1] != 0 || row[2] != 1.0/8 ||
				(row[3] != 0) != (value == 0) || (row[4] != 0) != (value < 0) ||
				row[13] != 127.0/1024 || row[14] != 128.0/1024 || row[15] != 16.0/128 {
				t.Fatal("condition data changed", input, row)
			}
			for choice, cell := range row[16:] {
				if (cell != 0) != (choice == target) || cell != 0 && cell != 1.0/8 {
					t.Fatal("condition target changed", target, row)
				}
			}
			input.Expected = false
			var opposite [DeclaredConditionFeatureDim]float32
			if err := DeclaredConditionFeaturesInto(input, &opposite); err != nil {
				t.Fatal(err)
			}
			row[1], row[2] = 1.0/8, 0
			if row != opposite {
				t.Fatal("Boolean goal changed unrelated cells")
			}
		}
	}
}

func TestDeclaredConditionFeatureErrorsAreAtomic(t *testing.T) {
	valid := DeclaredConditionFeatureInput{ChoiceCount: 1, Count: 1}
	if DeclaredConditionFeaturesInto(valid, nil) == nil {
		t.Fatal("nil destination accepted")
	}
	for _, input := range []DeclaredConditionFeatureInput{
		{}, {ChoiceCount: 17, Count: 1}, {ChoiceCount: 1, Count: 129},
		{ChoiceCount: 1, Count: 1, TargetChoice: -1}, {ChoiceCount: 1, Count: 1, TargetChoice: 1},
		{ChoiceCount: 1, Count: 1, Index: -1}, {ChoiceCount: 1, Count: 1, Index: 1},
	} {
		var row [DeclaredConditionFeatureDim]float32
		row[0], row[31] = 99, 42
		before := row
		if DeclaredConditionFeaturesInto(input, &row) == nil || row != before {
			t.Fatal("invalid input changed destination", input, row)
		}
	}
}
