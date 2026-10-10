package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func TestGoalPairCommandRecordsTrainingAndRunsExactCases(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	a, b := fixture(), fixture()
	b.TestCases = []pathplan.TestCase{{Input: -9007199254740995, Expected: -9007199254740995}, {Input: 9007199254740993, Expected: 10}}
	aPath := saveDocument(t, dir, "maximum.json", a)
	bPath := saveDocument(t, dir, "minimum.json", b)
	modelPath := filepath.Join(dir, "pairs.json")
	var out bytes.Buffer
	if err := run(ctx, []string{"fit", "-goal-pairs", "-out", modelPath, "-epochs", "2", aPath, bPath}, &out); err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Schema string       `json:"schema"`
		Pairs  *goalPairFit `json:"goal_pairs"`
	}
	if err := json.Unmarshal(out.Bytes(), &receipt); err != nil || receipt.Schema != "gooo/contract-local-fit/v2" || receipt.Pairs == nil || len(receipt.Pairs.Pairs) != 1 || receipt.Pairs.Pairs[0].First != 0 || receipt.Pairs.Pairs[0].Second != 1 || receipt.Pairs.Options.Weight != .5 || receipt.Pairs.Options.Margin != 2 {
		t.Fatal("missing training contract", out.String(), err)
	}
	// Two epochs establish CLI integration, not trained first-choice accuracy.
	for _, doc := range []string{aPath, bPath} {
		out.Reset()
		if err := run(ctx, []string{"search", "-model", modelPath, doc}, &out); err != nil {
			t.Fatal(err)
		}
		var search struct {
			Ranking pathplan.ContractRanking  `json:"ranking"`
			Result  pathplan.ContractProgress `json:"result"`
		}
		if err := json.Unmarshal(out.Bytes(), &search); err != nil || search.Ranking.Calls != 1 || search.Result.SelectedPassed != 2 || search.Result.Status != "TRAINING_COMPLETE" {
			t.Fatal("paired model search", out.String(), err)
		}
	}
	for _, paths := range [][]string{{aPath}, {aPath, aPath}} {
		unused := filepath.Join(dir, "rejected.json")
		args := append([]string{"fit", "-goal-pairs", "-out", unused, "-epochs", "1"}, paths...)
		if err := run(ctx, args, &out); err == nil {
			t.Fatal("invalid pair accepted", paths)
		}
		if _, err := os.Stat(unused); !os.IsNotExist(err) {
			t.Fatal("rejected fit wrote model", err)
		}
	}
}
