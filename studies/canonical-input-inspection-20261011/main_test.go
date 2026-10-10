package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestRetainedPlansHaveTheSameCanonicalInputWithoutChangingLabels(t *testing.T) {
	r, err := inspect("../multifamily-requirement-learning-20261011/result/sources.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if r.Documents != 256 || r.SourceProjections != 512 || r.ModelCalls != 0 || r.CandidateEvaluations != 0 || len(r.Pairs) != 96 {
		t.Fatal("inspection scope")
	}
	for family, group := range r.Groups {
		legacy := 0
		if family == "bound" {
			legacy = 32
		}
		if group != (counts{32, legacy, 32}) {
			t.Fatal(family, group)
		}
	}
	if r.Information.ConflictingGroups != 0 || r.Information.BestFirstPasses != 256 {
		t.Fatal("normalization lost declared distinctions")
	}
	raw, err := os.ReadFile("result.json")
	if err != nil {
		t.Fatal(err)
	}
	var saved result
	if json.Unmarshal(raw, &saved) != nil || !reflect.DeepEqual(r, saved) {
		t.Fatal("saved source inspection differs")
	}
}
