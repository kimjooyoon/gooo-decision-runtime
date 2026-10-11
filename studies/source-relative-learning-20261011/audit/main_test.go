package main

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/record"
)

func panics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("invalid record accepted")
		}
	}()
	fn()
}
func fixtureObservation() r.Observations {
	var x r.Observations
	must(json.Unmarshal([]byte(`{"Outputs":[{"Input":1,"Expected":2,"Actual":2,"Passed":true}],"Conditions":[{"Case":{"choice_id":"comparison","input":1,"expected":true},"Observation":{"Reached":true,"Value":true},"Passed":true}]}`), &x))
	return x
}
func TestObservedFlagsAndScoreOrder(t *testing.T) {
	if !observed(fixtureObservation()) {
		t.Fatal("complete observation")
	}
	panics(t, func() { x := fixtureObservation(); x.Outputs[0].Actual = 3; observed(x) })
	panics(t, func() { x := fixtureObservation(); x.Conditions[0].Observation.Reached = false; observed(x) })
	if got := order([2][2]float32{}); got != [4]uint16{0, 1, 2, 3} {
		t.Fatal(got)
	}
	if got := order([2][2]float32{{0, 4}, {0, 2}}); got != [4]uint16{3, 1, 2, 0} {
		t.Fatal(got)
	}
	if got := distribution([]int64{8, 2, 6, 4}); got.MedianNS != 5 || got.P95NS != 8 || got.Observations != 4 {
		t.Fatal(got)
	}
}
func TestLosslessCompressedReader(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "rows.jsonl.gz"))
	must(err)
	z := gzip.NewWriter(f)
	_, err = z.Write([]byte("{\"x\":7}\n"))
	must(err)
	must(z.Close())
	must(f.Close())
	if got := lines[map[string]int](dir, "rows.jsonl"); len(got) != 1 || got[0]["x"] != 7 {
		t.Fatal(got)
	}
}
func TestRankEventPrecedesEveryCandidate(t *testing.T) {
	dir := t.TempDir()
	zero := uint16(0)
	obs := fixtureObservation()
	candidate := r.Candidate{Mask: 0, Declared: obs, Acceptable: true}
	row := r.Search{ID: "unit", Mode: "fixed", Order: [4]uint16{0, 1, 2, 3}, Attempts: []r.Candidate{candidate}, Selected: &zero, Probe: obs, ResultState: "COMPLETE"}
	records := map[string]r.Search{"unit/fixed": row}
	events := []event{{Kind: "ranked", ID: "unit", Mode: "fixed", Order: row.Order}, {Kind: "attempt", ID: "unit", Mode: "fixed", Candidate: candidate}, {Kind: "probe", ID: "unit", Mode: "fixed", Observations: obs, ResultState: "COMPLETE"}}
	write := func(rows []event) {
		f, err := os.Create(filepath.Join(dir, "events.jsonl"))
		must(err)
		for _, e := range rows {
			must(json.NewEncoder(f).Encode(e))
		}
		must(f.Close())
	}
	write(events)
	if checkEvents(lines[event](dir, "events.jsonl"), records) != 3 {
		t.Fatal("events lost")
	}
	write([]event{events[1], events[0], events[2]})
	panics(t, func() { checkEvents(lines[event](dir, "events.jsonl"), records) })
	write(events[:2])
	panics(t, func() { checkEvents(lines[event](dir, "events.jsonl"), records) })
}

func TestZeroDurationStillAllowsRecordedRuleWork(t *testing.T) {
	auditPolicyTiming(r.Search{Mode: "rule", RuleBucketChecks: 6, RuleNS: 0, PreparationNS: 100})
	panics(t, func() { auditPolicyTiming(r.Search{Mode: "rule", RuleNS: -1}) })
	panics(t, func() { auditPolicyTiming(r.Search{Mode: "baseline", RuleNS: 1}) })
	panics(t, func() { auditPolicyTiming(r.Search{Mode: "fixed", PreparationNS: 1}) })
}
