package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func TestTypedSourceRelativeCLISearchAndOmission(t *testing.T) {
	doc := orderedDocument(false)
	doc.Plan.Base.Expressions[1].Int = 257
	doc.Plan.Base.Expressions[3].Operation = "multiply"
	doc.TestCases = []pathplan.TestCase{{Input: -3, Expected: -771}, {Input: 257, Expected: 0}, {Input: 701, Expected: -444}}
	doc.Plan.ConditionCases = []pathplan.ConditionCase{{ChoiceID: doc.Plan.Decisions[0].ID, Input: 256, Expected: true}, {ChoiceID: doc.Plan.Decisions[0].ID, Input: 257, Expected: false}}
	dir := t.TempDir()
	name := saveDocument(t, dir, "typed-heads.json", doc)
	var weights [2][contractdecision.TypedSourceRelativeHeadParameterCount]float32
	weights[0][contractdecision.TypedSourceRelativeHeadParameterCount-1] = 2
	m, err := contractdecision.NewTypedSourceRelative(weights)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	modelPath := filepath.Join(dir, "synthetic-typed.json")
	if err := os.WriteFile(modelPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, args := range [][]string{{"search", "-model", modelPath, name}, {"search", name}} {
		var out bytes.Buffer
		if err := run(ctx, args, &out); err != nil {
			t.Fatal(err)
		}
		var report struct {
			Ranking pathplan.ContractRanking
			Result  pathplan.ContractProgress
		}
		if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Result.Status != "TRAINING_COMPLETE" || report.Result.SelectedPassed != 3 || report.Result.PredictionsThisAdvance != 0 {
			t.Fatal("typed finite search", err)
		}
		wantCalls := 0
		if len(args) > 2 {
			wantCalls = 1
			if report.Ranking.Schema != "gooo/typed-source-relative-path-ranking/v1" || report.Ranking.ModelFingerprint != m.Fingerprint() {
				t.Fatal("typed artifact dispatch")
			}
		}
		if report.Ranking.Calls != wantCalls {
			t.Fatal("prediction count")
		}
	}
}
