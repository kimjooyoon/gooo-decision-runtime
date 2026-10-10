package contractdecision

import (
	"math"
	"testing"
)

func TestInteractionAffineHasExplicitRounding(t *testing.T) {
	// (1+2^-23)*(1-2^-23)-1 = -2^-46. Rounding the product
	// separately to float32 would instead erase the residual.
	x, y := float32(1+0x1p-23), float32(1-0x1p-23)
	if got := interactionAffine(-1, x, y); got != -0x1p-46 {
		t.Fatal("affine rule changed", got)
	}
	if got := interactionAffine(0, math.MaxFloat32, 8); !math.IsInf(float64(got), 1) {
		t.Fatal("overflow was hidden", got)
	}
}
