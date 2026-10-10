package main

import (
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
)

func TestTrainingSplitAndChannelAblationAreOwned(t *testing.T) {
	var x [384]float32
	x[319], x[320], x[383] = 1, 2, 3
	rows := []inputRecord{{Split: "train", Features: [][384]float32{x}, Masks: []uint16{0, 1}, Acceptable: 2}, {Split: "wording", Features: [][384]float32{x}}}
	off, on := trainingSamples(rows, false), trainingSamples(rows, true)
	if len(off) != 1 || len(on) != 1 || off[0].Inputs[0][319] != 1 || off[0].Inputs[0][320] != 0 || on[0].Inputs[0] != x {
		t.Fatal("split/ablation")
	}
	off[0].Inputs[0][0], off[0].Masks[0] = 99, 99
	if rows[0].Features[0] != x || rows[0].Masks[0] != 0 {
		t.Fatal("borrowed training data")
	}
}

func TestPruneChangesOnlyUnusedIncomingWeights(t *testing.T) {
	var weights [flowdecision.ParameterCount]float32
	for i := range weights {
		weights[i] = float32(i + 1)
	}
	m, err := flowdecision.New(weights)
	if err != nil {
		t.Fatal(err)
	}
	p := pruned(m).Weights()
	for i, value := range p {
		want := weights[i]
		if i < 384*24 && i%384 >= 320 {
			want = 0
		}
		if value != want {
			t.Fatal(i, value, want)
		}
	}
	if m.Weights() != weights {
		t.Fatal("changed original artifact")
	}
}
