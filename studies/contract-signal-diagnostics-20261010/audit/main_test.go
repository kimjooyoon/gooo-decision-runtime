package main

import (
	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	d "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-signal-diagnostics-20261010/record"
	"testing"
)

func TestAllRecordedGoalSignals(t *testing.T) {
	result, err := audit("../result", "../..")
	if err != nil {
		t.Fatal(err)
	}
	for mode, counts := range map[string][3]int{"mean": {61, 28, 8819}, "extreme": {62, 675, 6750}} {
		g := result.Groups[mode+"/all"]
		if g.Pairs != 97 || g.PoolEqual != 0 || g.SourceEqual != 97 || g.JointEqual != 0 || g.HiddenEqual != 0 || g.OptionsEqual != 0 || g.SameProposal != counts[0] || g.CrossedUnits != counts[1] || g.SourceDominated != counts[2] || g.TotalPairedUnits != 4656 || g.Units != 9312 {
			t.Fatal(mode, g)
		}
	}
}

func TestAlteredIntermediateAndGoalBindingsRejected(t *testing.T) {
	rows := d.Lines[d.Row](d.Read("../result", "traces.jsonl.gz", ""))
	source := d.Sources("../..")[0]
	raw := d.Read("../../contract-goals-20261010/result", "model.json", d.Models[0].SHA)
	m, err := contractdecision.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"prefix", "joint", "pool", "winner", "hidden", "score", "probability", "goal", "large_integer"} {
		t.Run(field, func(t *testing.T) {
			row := rows[0]
			s := source
			s.CaseRows = append([][32]float32(nil), source.CaseRows...)
			switch field {
			case "prefix":
				row.Trace.SourcePrefix[0][0]++
			case "joint":
				row.Trace.Joint[0][0]++
			case "pool":
				row.Trace.Pool[0]++
			case "winner":
				row.Trace.Winner[0] = 0
			case "hidden":
				row.Trace.Hidden[0][0]++
			case "score":
				row.Trace.OptionScores[0][0]++
			case "probability":
				row.Prediction.Probabilities[0] = 2
			case "goal":
				row.CaseSHA = "changed"
			case "large_integer":
				original := source.Document.TestCases[0]
				if original.Input >= -9007199254740992 && original.Input <= 9007199254740992 {
					t.Fatal("large integer fixture required")
				}
				if err := decision.DeclaredCaseFeaturesInto(original.Input+1, original.Expected, &s.CaseRows[0]); err != nil {
					t.Fatal(err)
				}
			}
			defer func() {
				if recover() == nil {
					t.Error("altered trace accepted")
				}
			}()
			checkTrace(row, s, m.Weights())
		})
	}
}
