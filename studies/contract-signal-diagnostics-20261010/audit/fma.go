package main

import (
	"math"
	"math/big"
)

// fused32 reconstructs the recorded arm64 FMADDS operation independently of
// the audit host's instruction selection. Float64 FMA has enough precision
// except when its rounded result lands exactly on a float32 midpoint. In that
// case, use exact arithmetic before the single float32 rounding. This avoids
// double rounding; no tolerance is applied to the recorded trace comparison.
func fused32(x, y, z float32) float32 {
	wide := math.FMA(float64(x), float64(y), float64(z))
	rounded := float32(wide)
	base := float64(rounded)
	if wide == base {
		return rounded
	}
	toward := float32(math.Inf(1))
	if wide < base {
		toward = float32(math.Inf(-1))
	}
	next := float64(math.Nextafter32(rounded, toward))
	midpoint := (base + next) / 2
	if math.IsInf(base, 0) || math.IsInf(next, 0) {
		midpoint = math.Copysign(math.Ldexp(1, 128)-math.Ldexp(1, 103), wide)
	}
	if wide != midpoint {
		return rounded
	}
	return exactFused32(x, y, z)
}

// Finite binary32 operands require at most 427 significant bits for their exact
// product-plus-addend across the entire exponent range. 512 bits retain it all.
func exactFused32(x, y, z float32) float32 {
	var a, b, c big.Float
	a.SetPrec(512).SetFloat64(float64(x))
	b.SetPrec(512).SetFloat64(float64(y))
	c.SetPrec(512).SetFloat64(float64(z))
	a.Mul(&a, &b).Add(&a, &c)
	result, _ := a.Float32()
	return result
}
