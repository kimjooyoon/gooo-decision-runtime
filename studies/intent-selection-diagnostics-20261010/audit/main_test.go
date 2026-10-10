package main

import (
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
	"testing"
)

func TestOriginalTraceRecount(t *testing.T) { audit("../result", "../..") }

func TestInactiveTraceRejectsInventedOptionScore(t *testing.T) {
	m, err := flowdecision.Decode(read("../../relational-flow-learning-20261010/result", "model-v6.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range lines[record](read("../result", "records.jsonl.gz")) {
		if r.Model != "v6" || r.ActiveHidden[1] != 0 {
			continue
		}
		old := previous{ID: r.ID, Model: r.Model, SourceSHA: r.SourceSHA, InputSHA: r.InputSHA, Acceptable: r.Acceptable, Prediction: r.Prediction}
		r.Explanation.OptionScores[1][0] += 1
		defer func() {
			if recover() == nil {
				t.Fatal("invented inactive score accepted")
			}
		}()
		trace(r, old, m)
		return
	}
	t.Fatal("inactive trace missing")
}
