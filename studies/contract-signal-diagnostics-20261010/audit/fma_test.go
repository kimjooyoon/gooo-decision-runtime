package main

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestRecordedFMAArithmeticAcrossRoundingBoundaries(t *testing.T) {
	triples := [][3]float32{
		{1 + 0x1p-23, 1 - 0x1p-23, -1},
		{(1 + 0x1p-23) * 0x1p-24, 1 - 0x1p-23, 1 + 0x1p-23},
		{math.MaxFloat32, 2, -math.MaxFloat32},
		{math.SmallestNonzeroFloat32, 0.5, 0},
		{math.SmallestNonzeroFloat32, 0.5, math.SmallestNonzeroFloat32},
		{math.MaxFloat32, 1, 0x1p103},
		{-math.MaxFloat32, 1, -0x1p103},
	}
	rng := rand.New(rand.NewPCG(17, 19))
	for range 4096 {
		var triple [3]float32
		for i := range triple {
			for {
				x := math.Float32frombits(rng.Uint32())
				if !math.IsNaN(float64(x)) && !math.IsInf(float64(x), 0) {
					triple[i] = x
					break
				}
			}
		}
		triples = append(triples, triple)
	}
	for _, v := range triples {
		got, want := fused32(v[0], v[1], v[2]), exactFused32(v[0], v[1], v[2])
		if got != want {
			t.Fatalf("%v: got %08x want %08x", v, math.Float32bits(got), math.Float32bits(want))
		}
	}
	v := triples[1]
	naive := float32(math.FMA(float64(v[0]), float64(v[1]), float64(v[2])))
	if naive == fused32(v[0], v[1], v[2]) {
		t.Fatal("fixture must catch float64-to-float32 double rounding")
	}
}
