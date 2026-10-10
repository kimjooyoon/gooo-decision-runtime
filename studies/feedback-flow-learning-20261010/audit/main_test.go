package main

import (
	"slices"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
)

func TestTrainingRejectsChangedFeedbackAndLabels(t *testing.T) {
	sources := lines[source](read("../../semantic-flow-normalization-20261010/result", "records.jsonl.gz"))
	for i := range sources {
		sources[i].V6 = relational(sources[i])
	}
	for _, name := range []string{"feedback", "label", "initial"} {
		t.Run(name, func(t *testing.T) {
			samples := decode[[]flowdecision.Sample](read("../result", "training-feedback_v6.json.gz"))
			switch name {
			case "feedback":
				samples[16].Inputs[0][270] += 1.0 / 2048
			case "label":
				samples[16].Acceptable ^= 1
			case "initial":
				samples[0].Inputs[0][256] = .125
			}
			defer func() {
				if recover() == nil {
					t.Fatal("changed training accepted")
				}
			}()
			auditTraining("../result", sources, samples)
		})
	}
}

func TestExactPositiveAndNegativeOutputsCannotMoveTogether(t *testing.T) {
	sources := lines[source](read("../../semantic-flow-normalization-20261010/result", "records.jsonl.gz"))
	byID := map[string]source{}
	for _, s := range sources {
		byID[s.ID] = s
	}
	rows := lines[judgment](read("../result", "judgments.jsonl.gz"))
	for _, positive := range []bool{false, true} {
		found := false
		for _, row := range rows {
			for i, output := range row.Selected.Outputs {
				if (positive && output.Actual > 9007199254740992) || (!positive && output.Actual < -9007199254740992) {
					changed := slices.Clone(row.Selected.Outputs)
					changed[i].Actual++
					changed[i].Expected++
					func() {
						defer func() {
							if recover() == nil {
								t.Fatal("coordinated integer mutation accepted")
							}
						}()
						auditOutputs(byID[row.ID], row.Selected.Mask, changed)
					}()
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Fatal("missing exact output sign", positive)
		}
	}
}

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
