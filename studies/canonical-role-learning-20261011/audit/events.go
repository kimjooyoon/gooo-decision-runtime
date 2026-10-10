package main

import (
	"reflect"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-role-learning-20261011/record"
)

type event struct {
	Kind, ID, Mode string
	Predictions    []r.Prediction
	Logits         [2][2]float32
	Order          [4]uint16
	Candidate      r.Candidate
	Mask           uint16
	Observations   r.Observations
}

func auditEvents(root string, records map[string]r.Search) int {
	all := lines[event](root, "events.jsonl")
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
			if active != "" && !closed {
				previous := records[active]
				require(previous.Selected == nil && attempts == len(previous.Attempts), "unfinished event sequence")
			}
			require(!seen[key], "duplicate ranking event")
			seen[key] = true
			active = key
			attempts = 0
			closed = false
			rankings++
			require(reflect.DeepEqual(e.Predictions, row.Predictions) && e.Logits == row.Logits && e.Order == row.Order, "initial ranking event differs")
		case "attempt":
			require(active == key && !closed && attempts < len(row.Attempts), "candidate event order differs")
			require(reflect.DeepEqual(e.Candidate, row.Attempts[attempts]), "retained attempt event differs")
			attempts++
		case "probe":
			require(active == key && !closed && attempts == len(row.Attempts) && row.Selected != nil, "probe event order differs")
			require(e.Mask == *row.Selected && reflect.DeepEqual(e.Observations, row.Probe), "probe event differs")
			closed = true
		default:
			panic("unknown event kind")
		}
	}
	require(rankings == len(records), "ranking event coverage differs")
	if !closed {
		last := records[active]
		require(last.Selected == nil && attempts == len(last.Attempts), "last event incomplete")
	}
	return len(all)
}
