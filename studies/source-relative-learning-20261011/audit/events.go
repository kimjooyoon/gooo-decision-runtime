package main

import (
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/record"
	"reflect"
)

type event struct {
	Kind, ID, Mode   string
	InputSHA         string `json:"input_sha"`
	Predictions      []r.Prediction
	Logits           [2][2]float32
	Order            [4]uint16
	Candidate        r.Candidate
	Mask             uint16
	ResultState      string `json:"result_state"`
	Observations     r.Observations
	PreparationNS    int64 `json:"preparation_ns"`
	RuleNS           int64 `json:"rule_ns"`
	RuleBucketChecks int   `json:"rule_bucket_checks"`
	RelationRows     int   `json:"relation_rows"`
}

func checkEvents(all []event, records map[string]r.Search) int {
	active := ""
	attempts, rankings := 0, 0
	closed := true
	seen := map[string]bool{}
	for _, e := range all {
		key := e.ID + "/" + e.Mode
		row, ok := records[key]
		require(ok, "event has no search")
		switch e.Kind {
		case "ranked":
			require(closed && !seen[key], "unfinished or duplicate ranking")
			seen[key] = true
			active = key
			attempts = 0
			closed = false
			rankings++
			require(reflect.DeepEqual(e.Predictions, row.Predictions) && e.Logits == row.Logits && e.Order == row.Order && e.InputSHA == row.InputSHA, "ranking event differs")
			require(e.PreparationNS == row.PreparationNS && e.RuleNS == row.RuleNS && e.RuleBucketChecks == row.RuleBucketChecks && e.RelationRows == row.RelationRows, "ranking cost event differs")
		case "attempt":
			require(active == key && !closed && attempts < len(row.Attempts), "candidate event order differs")
			require(reflect.DeepEqual(e.Candidate, row.Attempts[attempts]), "attempt event differs")
			attempts++
		case "probe":
			require(active == key && !closed && attempts == len(row.Attempts), "probe event order differs")
			require(e.Mask == row.ResultMask && e.ResultState == row.ResultState && reflect.DeepEqual(e.Observations, row.Probe), "result probe event differs")
			closed = true
		default:
			panic("unknown event kind")
		}
	}
	require(closed && rankings == len(records), "event coverage incomplete")
	return len(all)
}
