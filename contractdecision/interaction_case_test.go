package contractdecision

import (
	"math"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestInteractionCasesBindExpectedPolarityToExactInputs(t *testing.T) {
	for _, values := range [][2]int64{{math.MinInt64, math.MaxInt64}, {9007199254740993, 9007199254740995}} {
		var original [2][2][32]float32
		var means [2][interactionCaseDim]float32
		for goal := range 2 {
			for i, value := range values {
				if err := decision.DeclaredConditionFeaturesInto(decision.DeclaredConditionFeatureInput{
					Input: value, Expected: i == goal, TargetChoice: 0, ChoiceCount: 2, Index: i, Count: 2}, &original[goal][i]); err != nil {
					t.Fatal(err)
				}
				before := original[goal][i]
				associated := interactionCase(&original[goal][i], 1)
				if original[goal][i] != before {
					t.Fatal("encoding changed the original input")
				}
				for j, cell := range associated {
					means[goal][j] += cell / 2
				}
				for j := range 32 {
					if associated[j] != before[j]*8 {
						t.Fatal("original field lost", j)
					}
				}
				polarity := float32(-1)
				if i == goal {
					polarity = 1
				}
				for j := range 8 {
					want := float32(byte(uint64(value)>>uint(56-8*j))) / 256 * polarity
					if associated[35+j] != want {
						t.Fatal("input bit lost or associated with the wrong target", value, j)
					}
				}
			}
		}
		for j := range 32 {
			if means[0][j] != means[1][j] {
				t.Fatal("fixture raw means should collide", j)
			}
		}
		if means[0] == means[1] {
			t.Fatal("opposite input-condition associations disappeared")
		}
		// The added channels describe input/target association, not authored row order.
		changed := original[0][0]
		changed[13], changed[14], changed[15], changed[16], changed[17] = 1, 2, 3, 0, 1
		a, b := interactionCase(&original[0][0], 1), interactionCase(&changed, 1)
		for j := 32; j < interactionCaseDim; j++ {
			if a[j] != b[j] {
				t.Fatal("ordinal/choice metadata entered the association products", j)
			}
		}
	}
}

func TestInteractionOutputAssociationCoversNegativeZeroAndPositive(t *testing.T) {
	for _, expected := range []int64{-7, 0, 7} {
		var row [32]float32
		if err := decision.DeclaredCaseFeaturesInto(9007199254740993, expected, &row); err != nil {
			t.Fatal(err)
		}
		got := interactionCase(&row, 0)
		polarity := float32(0)
		if expected < 0 {
			polarity = -1
		} else if expected > 0 {
			polarity = 1
		}
		if got[34] != polarity || got[42] != polarity/256 {
			t.Fatal("signed expected value is not bound to the exact input", expected, got)
		}
	}
}
