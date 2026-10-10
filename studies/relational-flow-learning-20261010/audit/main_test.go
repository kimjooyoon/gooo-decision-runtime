package main

import "testing"

func TestOriginalLearningAndSearchRecords(t *testing.T) {
	audit("../result", "../../semantic-flow-normalization-20261010/result")
}

func TestRecountRejectsOneUnitErrorAboveFloatIntegerRange(t *testing.T) {
	sources := lines[source](read("../../semantic-flow-normalization-20261010/result", "records.jsonl.gz"))
	s := sources[0]
	j := lines[judgment](read("../result", "judgments.jsonl.gz"))[0]
	for i := range j.Selected.Outputs {
		if j.Selected.Outputs[i].Actual > 9007199254740992 || j.Selected.Outputs[i].Actual < -9007199254740992 {
			j.Selected.Outputs[i].Actual++
			defer func() {
				if recover() == nil {
					t.Fatal("one-unit mismatch was not rejected")
				}
			}()
			auditOutputs(s, j.Selected.Mask, j.Selected.Outputs)
			return
		}
	}
	t.Fatal("large exact output missing from fixture")
}
