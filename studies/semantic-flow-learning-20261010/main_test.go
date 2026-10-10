package main

import "testing"

func TestOnlyDirectTrainingSamplesAreCopied(t *testing.T) {
	var v4, v5 [384]float32
	v4[0], v5[0] = 1, 2
	rows := []prepared{
		{Split: "future_train", V4: [][384]float32{v4}, V5: [][384]float32{v5}, Acceptable: 4},
		{Split: "representation_transfer"},
		{Split: "constant_transfer"},
		{Split: "control"},
	}
	a, b := training(rows, false), training(rows, true)
	if len(a) != 1 || len(b) != 1 || a[0].Inputs[0] != v4 || b[0].Inputs[0] != v5 || a[0].Acceptable != 4 {
		t.Fatal("training boundary")
	}
	a[0].Inputs[0][0] = 99
	a[0].Masks[0] = 3
	if rows[0].V4[0] != v4 || b[0].Masks[0] != 0 {
		t.Fatal("training aliases original data")
	}
}

func TestFrozenInitialRowsAndSplits(t *testing.T) {
	rows, _ := load("../semantic-flow-normalization-20261010/result")
	for _, row := range rows {
		for _, array := range append(row.V4[:len(row.V4):len(row.V4)], row.V5...) {
			for _, segment := range [][]float32{array[192:236], array[256:320]} {
				for _, v := range segment {
					if v != 0 {
						t.Fatal("future outcome in initial input", row.ID)
					}
				}
			}
		}
	}
	if len(training(rows, false)) != 16 || len(training(rows, true)) != 16 {
		t.Fatal("fixed train split")
	}
}
