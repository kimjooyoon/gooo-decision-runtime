package main

import "testing"

func TestOriginalHeldoutRepairRecords(t *testing.T) {
	audit("../result", "../../semantic-flow-normalization-20261010/result")
}

func TestChangedObservationAndInputsAreRejected(t *testing.T) {
	sources := lines[source](read("../../semantic-flow-normalization-20261010/result", "records.jsonl.gz"))
	byID := map[string]source{}
	for _, s := range sources {
		s.V6 = relational(s)
		byID[s.ID] = s
	}
	for _, kind := range []string{"exact_failure", "source_channel", "feedback_channel", "identity"} {
		t.Run(kind, func(t *testing.T) {
			o := lines[observation](read("../result", "observations.jsonl.gz"))[0]
			s := byID[o.ID]
			switch kind {
			case "exact_failure":
				if !o.OutputPresent || o.Output.Result.Actual >= -9007199254740992 {
					t.Fatal("large exact failure fixture")
				}
				o.Output.Result.Actual++
			case "source_channel":
				o.Inputs[0][0]++
			case "feedback_channel":
				o.Inputs[0][270] += 1.0 / 2048
			case "identity":
				o.CaseSHA = "changed"
			}
			defer func() {
				if recover() == nil {
					t.Fatal("changed observation accepted")
				}
			}()
			auditObservation(s, o)
		})
	}
}
