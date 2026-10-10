package main

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/kimjooyoon/gooo-decision-runtime/executiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func checkPruning(dir string) {
	before, err := executiondecision.Decode(raw(dir, "model-v3_output_off-before-pruning.json"))
	must(err)
	after, err := executiondecision.Decode(raw(dir, "model-v3_output_off.json"))
	must(err)
	a, b := before.Weights(), after.Weights()
	for i, v := range a {
		if i < 320*24 && i%320 >= 256 {
			require(b[i] == 0, "output weight not pruned")
		} else {
			require(v == b[i], "pruning changed an active weight")
		}
	}
}

func firstWrong(c candidate) *pathplan.TestResult {
	for _, row := range c.Cases {
		if !row.Passed {
			return &row
		}
	}
	return nil
}

// Reconstruct the added channel solely from saved exact evaluator values.
func checkOutputTail(in input, p program, s source, choice int) {
	var tail [64]float32
	if in.Context != "initial" {
		var mask int
		_, err := fmt.Sscanf(in.Context, "observed-%d", &mask)
		must(err)
		require(mask >= 0 && mask < len(p.Candidates), "context mask")
		row := firstWrong(p.Candidates[mask])
		if row != nil {
			flag := func(i int, v bool) {
				if v {
					tail[i] = 0.125
				}
			}
			flag(0, true)
			flag(1, mask>>choice&1 == 0)
			flag(2, mask>>choice&1 != 0)
			for j, value := range [3]int64{row.Input, row.Expected, row.Actual} {
				flag(3+j*3, value < 0)
				flag(4+j*3, value == 0)
				flag(5+j*3, value > 0)
				for b := range 8 {
					tail[14+j*8+b] = float32(byte(uint64(value)>>uint(56-b*8))) / 2048
				}
			}
			flag(12, row.Actual < row.Expected)
			flag(13, row.Actual > row.Expected)
			for b := range 16 {
				flag(38+b, mask>>b&1 != 0)
			}
			tail[54], tail[55] = float32(len(s.Document.Plan.Decisions))/128, float32(choice)/128
		}
	}
	for _, version := range []int{0, 1} {
		for _, cell := range in.Features[version][choice][256:] {
			require(cell == 0, "control contains output feedback")
		}
	}
	require(slices.Equal(in.Features[2][choice][256:], tail[:]), "output channel differs from actual candidate")
}

func checkRankingFailure(r pathplan.ConditionRanking, p program) {
	if r.OutputFailure != nil {
		f := r.OutputFailure
		require(int(f.Mask) < len(p.Candidates) && f.Mask == r.ObservedMask, "feedback mask")
		wrong := firstWrong(p.Candidates[f.Mask])
		require(wrong != nil && reflect.DeepEqual(f.Result, *wrong), "feedback output differs from saved execution")
	}
	if r.HasFailure {
		require(int(r.Failure.Mask) < len(p.Candidates), "condition feedback mask")
		found := false
		for _, row := range p.Candidates[r.Failure.Mask].Conditions {
			if !row.Passed {
				require(reflect.DeepEqual(row, r.Failure.Result), "first condition failure")
				found = true
				break
			}
		}
		require(found, "invented condition failure")
	}
}
