package main

import (
	"path/filepath"
	"testing"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
)

func rejected(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("altered observation accepted")
		}
	}()
	fn()
}
func TestOriginalResultsAndIndependentArithmetic(t *testing.T) {
	root, err := filepath.Abs("../result")
	if err != nil {
		t.Fatal(err)
	}
	s, err := audit(root)
	if err != nil || s.Improved != 49 || s.Regressed != 21 || s.Unchanged != 126 || s.Groups["model/constant"].FirstValid != 50 {
		t.Fatal(s, err)
	}
	if s.Pairs["wording"].SameProposal != 12 || s.Pairs["rare_tail"].DifferentScores != 1 {
		t.Fatal("lost incomplete discrimination", s.Pairs)
	}
}
func TestTamperedCasesLabelsAndReceiptsAreRejected(t *testing.T) {
	sources := lines[r.Source](load("../result", "sources.jsonl.gz"))
	x := sources[0]
	for _, change := range []func(*r.Source){
		func(x *r.Source) { x.Candidates[0].Outputs[0].Actual++ },
		func(x *r.Source) { x.Candidates[0].Outputs[0].Actual++; x.Candidates[0].Outputs[0].Expected++ },
		func(x *r.Source) { x.CaseRows[0][12] += 1.0 / 2048 },
		func(x *r.Source) { x.Inputs[0][0] += 1 },
		func(x *r.Source) { x.Acceptable ^= 1 },
		func(x *r.Source) { x.Document.Plan.ConditionCases[0].Expected = false },
	} {
		v := decode[r.Source](r.Encode(x))
		change(&v)
		rejected(t, func() { checkSource(v, x.Spec) })
	}
	searches := lines[r.Search](load("../result", "searches.jsonl.gz"))
	base := searches[1]
	for _, change := range []func(*r.Search){
		func(x *r.Search) { x.Ranking.Calls++ },
		func(x *r.Search) { x.Progress[1].NewAttempts[0].Results[0].Actual++ },
		func(x *r.Search) { x.Ranking.Proposed ^= 2 },
		func(x *r.Search) { x.Progress[1].PreviousSHA = "changed" },
	} {
		v := decode[r.Search](r.Encode(base))
		change(&v)
		rejected(t, func() { checkSearch(v, x, base.Ranking.ModelFingerprint) })
	}
}
