package main

import (
	"encoding/json"
	"math"
	"testing"
)

func TestNewV2RetainedAuditHasZeroNewExecutions(t *testing.T) {
	got, err := audit("../result")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"new_fits", "new_predictions", "new_candidate_executions"} {
		if got[key] != 0 {
			t.Fatal("reader executed a model or candidate", got)
		}
	}
	if got["source_documents"] != 100 || got["contract_groups"] != 25 {
		t.Fatal("reader narrowed corpus")
	}
}

func TestV2RetainedReaderRejectsSourceAndFeatureMutations(t *testing.T) {
	sources, _, err := corpus("../result")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(sources[60])
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*source){
		func(s *source) { s.Split = "train" },
		func(s *source) { s.Inputs[0][441] = 1.0 / 8 },
		func(s *source) { s.Candidates[0].Outputs[0].Passed = !s.Candidates[0].Outputs[0].Passed },
		func(s *source) { s.Probes[0].Input = s.Document.TestCases[0].Input },
		func(s *source) { s.Document.Plan.Decisions = s.Document.Plan.Decisions[:1] },
	} {
		var s source
		if err = json.Unmarshal(raw, &s); err != nil {
			t.Fatal(err)
		}
		change(&s)
		if err = auditSource(s, 60); err == nil {
			t.Fatal("altered retained source accepted")
		}
	}
}

func TestV2RetainedReaderRejectsProbabilityAndCompletionMutations(t *testing.T) {
	sources, _, err := corpus("../result")
	if err != nil {
		t.Fatal(err)
	}
	var original []observation
	if err = decode("../result", "observations.json", &original); err != nil {
		t.Fatal(err)
	}
	var r report
	if err = decode("../result", "report.json", &r); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func([]observation){
		func(rows []observation) { rows[0].First.Complete++ },
		func(rows []observation) { rows[0].Probabilities[0] = float32(math.NaN()) },
		func(rows []observation) { rows[0].Attempts++ },
		func(rows []observation) { rows[0].CheckedMask = (rows[0].CheckedMask + 1) % 4 },
	} {
		rows := append([]observation(nil), original...)
		change(rows)
		if err = auditObservations(sources, rows, r.Splits); err == nil {
			t.Fatal("altered prediction/completion accepted")
		}
	}
}
