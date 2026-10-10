package main

import (
	"reflect"
	"testing"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/multifamily-requirement-learning-20261011/record"
)

func rejected(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("altered record accepted")
		}
	}()
	fn()
}

// These checks consume retained original records. They never train, predict,
// lower Gooo or execute a candidate, including when CI repeats the tests.
func TestOriginalObservationAndCohortDenominators(t *testing.T) {
	s, err := audit("../result")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, decode[Summary](load("../result", "audit.json"))) {
		t.Fatal("published recount differs")
	}
	for mode, expected := range map[string][3]int{
		"deterministic": {16, 32, 16}, "released": {40, 72, 33}, "multifamily": {64, 117, 34},
	} {
		for i, cohort := range []string{"train", "in_family_evaluation", "family_evaluation"} {
			g := s.Groups[mode+"/"+cohort]
			n := []int{64, 128, 64}[i]
			if g.Programs != n || g.FirstValid != expected[i] || g.Complete != n || g.FirstOutputTotal != n*8 || g.FirstConditionTotal != n*3 {
				t.Fatal(mode, cohort, g)
			}
		}
	}
	if s.Comparisons["released->multifamily/in_family_evaluation"].Regressed != 4 || s.Comparisons["released->multifamily/family_evaluation"].Regressed != 8 {
		t.Fatal("lost regressions")
	}
	if s.Sources != 256 || s.TrainingRows != 64 || s.Searches != 768 || s.ModelCalls != 512 || s.InputCollisions["both_models"].ConflictingGroups != 0 {
		t.Fatal("scope")
	}
	x := int64(18014398509481990)
	v, c := actual(r.Spec{Family: "distance", K: -37}, 2, x)
	if v != 18014398509482027 || c {
		t.Fatal("exact integer arithmetic", v, c)
	}
}

func TestChangedSourceArraysOutcomesAndLabelsAreRejected(t *testing.T) {
	x := lines[r.Source](load("../result", "sources.jsonl.gz"))[0]
	for _, change := range []func(*r.Source){
		func(x *r.Source) { x.Spec.Split = "heldout" },
		func(x *r.Source) { x.Candidates[0].Outputs[0].Actual++ },
		func(x *r.Source) { x.Candidates[0].Outputs[0].Expected++ },
		func(x *r.Source) {
			x.Candidates[0].Conditions[0].Observation.Value = !x.Candidates[0].Conditions[0].Observation.Value
		},
		func(x *r.Source) { x.Acceptable ^= 1 },
		func(x *r.Source) { x.ConditionRows[0][12] += 1.0 / 2048 },
		func(x *r.Source) { x.CaseRows[0][12] += 1.0 / 2048 },
		func(x *r.Source) {
			x.OrderedInputs[0][400] += 1.0 / 2048
			x.OrderedSHA[0] = r.OrderedFeatureSHA(x.OrderedInputs[0])
		},
	} {
		v := decode[r.Source](r.Encode(x))
		change(&v)
		rejected(t, func() { checkSource(v, x.Spec) })
	}
	base := lines[r.Search](load("../result", "searches.jsonl.gz"))[2]
	for _, change := range []func(*r.Search){
		func(x *r.Search) { x.Ranking.Calls++ },
		func(x *r.Search) { x.Ranking.Proposed ^= 2 },
		func(x *r.Search) { x.Progress[1].PreviousSHA = "changed" },
		func(x *r.Search) { x.Progress[1].NewAttempts[0].Results[0].Actual++ },
	} {
		v := decode[r.Search](r.Encode(base))
		change(&v)
		rejected(t, func() { checkSearch(v, x, base.Ranking.ModelFingerprint) })
	}
}

func TestComparatorIsNotReportedAsANewFit(t *testing.T) {
	report := decode[r.Report](load("../result", "report.json"))
	m := report.Models["released"]
	m.FitInThisStudy = true
	report.Models["released"] = m
	rejected(t, func() { checkModels("../result", report) })
	report = decode[r.Report](load("../result", "report.json"))
	m = report.Models["multifamily"]
	m.SHA = "changed"
	report.Models["multifamily"] = m
	rejected(t, func() { checkModels("../result", report) })
}
