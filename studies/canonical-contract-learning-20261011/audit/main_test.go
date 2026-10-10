package main

import (
	"reflect"
	"testing"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
)

// Read the one original execution. Never fit, predict, lower or execute it again.
func TestRetainedOriginal(t *testing.T) {
	got := audit("../result")
	want := decode[Summary](load("../result", "audit.json"))
	if !reflect.DeepEqual(got, want) {
		t.Fatal("retained audit changed")
	}
	for _, mode := range []string{"fixed", "legacy", "canonical"} {
		g := got.Groups[mode+"/sparse"]
		if g.Programs != 64 || g.InitialDeclaredComplete != 64 || g.InitialProbeComplete != 16 || g.FinalProbeComplete != 64 {
			t.Fatalf("%s lost initial missing requirements: %+v", mode, g)
		}
	}
	p := got.Pairs["canonical/in_family"]
	if p != (Pair{Pairs: 64, SameInput: 64, SameScores: 64, SameProposal: 64}) {
		t.Fatalf("equivalent-form consistency: %+v", p)
	}
}

func TestRejectChangedRecordedOutput(t *testing.T) {
	x := lines[r.Source](load("../result", "sources.jsonl"))[0]
	x.Candidates[0].Declared.Outputs[0].Actual++
	defer func() {
		if recover() == nil {
			t.Fatal("changed original output was accepted")
		}
	}()
	checkSource(x, r.Specs()[0])
}
