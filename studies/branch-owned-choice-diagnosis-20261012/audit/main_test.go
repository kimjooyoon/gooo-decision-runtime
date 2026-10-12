package main

import (
	"math"
	"testing"
)

func TestSavedChoicesAndConfusionHaveExplicitBitOrder(t *testing.T) {
	for want := range uint16(4) {
		for got := range uint16(4) {
			o := observation{FirstMask: got, CheckedMask: want, Probabilities: []float64{.25, .25, .25, .25}}
			o.First.Complete = want == got
			var s totals
			if err := s.add(o, 1<<want); err != nil {
				t.Fatal(err)
			}
			if s.Rows != 1 || s.ErrorBits[want^got] != 1 || s.SelectedMasks[got] != 1 || s.AcceptableMasks[want] != 1 {
				t.Fatalf("wrong row accounting: %+v", s)
			}
			for choice := range 2 {
				w, g := want>>choice&1, got>>choice&1
				if s.ChoiceConfusion[choice][w][g] != 1 || (s.ChoiceCorrect[choice] == 1) != (w == g) {
					t.Fatalf("wrong choice accounting: %+v", s)
				}
			}
			if s.MeanEntropyBits != 2 || s.MaxProbabilityFactorizationResidual != 0 {
				t.Fatal(s)
			}
		}
	}
}

func TestRejectsAmbiguousAcceptanceAndInvalidSavedProbabilities(t *testing.T) {
	o := observation{FirstMask: 0, CheckedMask: 1, Probabilities: []float64{.4, .3, .2, .1}}
	for _, acceptable := range []uint16{0, 3, 16} {
		var s totals
		if s.add(o, acceptable) == nil || s.Rows != 0 {
			t.Fatal("invalid acceptance changed totals")
		}
	}
	for _, p := range [][]float64{{1}, {.4, .3, .2, -.1}, {.4, .3, .2, math.NaN()}, {.4, .3, .2, math.Inf(1)}, {.4, .3, .2, .2}, {.1, .4, .3, .2}} {
		broken := o
		broken.Probabilities = p
		var s totals
		if s.add(broken, 2) == nil || s.Rows != 0 {
			t.Fatal("invalid distribution changed totals")
		}
	}
}
