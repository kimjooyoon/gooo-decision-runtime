package main

import "testing"

func TestDecodeKeepsExactInteger(t *testing.T) {
	const want int64 = -9007199254740995
	var fields [16]float32
	fields[0], fields[2], fields[5] = .125, .125, .125
	x := want
	for i := range 8 {
		fields[8+i] = float32(byte(uint64(x)>>uint(56-8*i))) / 2048
	}
	if got := decode(fields[:]); got != (value{Present: true, Kind: "int", Int: want}) {
		t.Fatal(got)
	}
}

func TestCollisionsUseDisjointSets(t *testing.T) {
	rows := []row{{BeforeSHA: "same", AfterSHA: "a", Acceptable: 1, Split: "assignment"},
		{BeforeSHA: "same", AfterSHA: "b", Acceptable: 2, Split: "assignment"},
		{BeforeSHA: "same", AfterSHA: "a", Acceptable: 3, Split: "train"}}
	before, after, groups := collisions(rows)
	if before != 1 || after != 0 || groups["assignment/assignment"] != 1 || len(groups) != 1 {
		t.Fatal(before, after, groups)
	}
}
