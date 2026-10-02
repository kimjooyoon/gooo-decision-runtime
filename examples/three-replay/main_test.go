package main

import (
	"math"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
)

func TestReplayChecksEveryVectorAndTieOrder(t *testing.T) {
	var w jointdecision.ThreeWorkspace
	p := jointdecision.ThreePrediction{Probabilities: [8]float32{.5, .5}}
	o := observation{Features: bitsSHA(w.Features[:]), HiddenSHA: bitsSHA(w.Hidden[:]), LogitsSHA: bitsSHA(p.Logits[:]), ProbsSHA: bitsSHA(p.Probabilities[:]), Hidden: w.Hidden, Prediction: p, Order: [8]int{0, 1, 2, 3, 4, 5, 6, 7}}
	if !matches(w, p, o) {
		t.Fatal("valid reference mismatch")
	}
	bad := o
	bad.Order[0], bad.Order[1] = 1, 0
	if matches(w, p, bad) {
		t.Fatal("tie order ignored")
	}
	bad = o
	bad.Hidden[7] = 1
	if matches(w, p, bad) {
		t.Fatal("hidden observation ignored")
	}
	w.Features[5] = math.Float32frombits(0x80000000)
	if matches(w, p, o) {
		t.Fatal("signed zero bits ignored")
	}
}
