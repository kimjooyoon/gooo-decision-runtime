package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func fixture() pathplan.Document {
	return pathplan.Document{Schema: pathplan.DocumentSchema, MaxAttempts: 2,
		Plan: pathplan.Plan{Schema: pathplan.Schema,
			Base: bodyplan.Plan{Schema: bodyplan.Schema, ID: "declared-contract", Name: "DeclaredContract", ResultType: decision.TypeInt,
				Expressions: []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 10}, {Kind: "binary", Operation: "less_than", Left: 0, Right: 1}},
				Statements:  []bodyplan.Stmt{{Kind: "return", Expr: 0}, {Kind: "return", Expr: 1}, {Kind: "if", Expr: 2, Then: []int{0}, Else: []int{1}}}, Root: []int{2}},
			Decisions: []pathplan.Choice{{ID: "branches", Kind: pathplan.BranchLayout, Target: 2, Intent: "return required value / 요구한 값 반환", Fallback: "layout_forward", Options: []pathplan.Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}}}},
		TestCases: []pathplan.TestCase{{Input: -9007199254740995, Expected: 10}, {Input: 9007199254740993, Expected: 9007199254740993}}}
}

func saveDocument(t *testing.T, dir, name string, d pathplan.Document) string {
	t.Helper()
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLocalFitAndBothSearchModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	a := fixture()
	b := fixture()
	b.TestCases = []pathplan.TestCase{{Input: -9007199254740995, Expected: -9007199254740995}, {Input: 9007199254740993, Expected: 10}}
	aPath := saveDocument(t, dir, "maximum.json", a)
	bPath := saveDocument(t, dir, "minimum.json", b)
	modelPath := filepath.Join(dir, "model.json")
	var out bytes.Buffer
	if err := run(ctx, []string{"fit", "-out", modelPath, "-epochs", "2", aPath, bPath}, &out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := contractdecision.Decode(raw); err != nil {
		t.Fatal(err)
	}
	// Two epochs exercise the command path; no trained accuracy claim is made.
	for _, args := range [][]string{{"search", aPath}, {"search", "-model", modelPath, aPath}} {
		out.Reset()
		if err := run(ctx, args, &out); err != nil {
			t.Fatal(err)
		}
		var got struct {
			Ranking pathplan.ContractRanking  `json:"ranking"`
			Result  pathplan.ContractProgress `json:"result"`
		}
		if err := json.Unmarshal(out.Bytes(), &got); err != nil || got.Result.Status != "TRAINING_COMPLETE" || got.Result.SelectedPassed != 2 {
			t.Fatal(got, err)
		}
		wantCalls := 0
		if len(args) > 2 {
			wantCalls = 1
		}
		if got.Ranking.Calls != wantCalls {
			t.Fatal("call count", got.Ranking)
		}
	}
	if err := run(ctx, []string{"fit", "-out", modelPath, aPath}, &out); err == nil {
		t.Fatal("model overwritten")
	}
	a.MaxAttempts = 1
	aPath = saveDocument(t, dir, "short.json", a)
	out.Reset()
	if err := run(ctx, []string{"search", aPath}, &out); err == nil || !bytes.Contains(out.Bytes(), []byte(`"status":"PARTIAL"`)) {
		t.Fatal("partial success hidden", err, out.String())
	}
}

func TestTrainingRejectsUnsatisfiableContractsAndCommandBounds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	d := fixture()
	d.TestCases[0].Expected = 999
	p, err := d.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sample(ctx, d, p); err == nil {
		t.Fatal("empty acceptable set")
	}
	for _, args := range [][]string{nil, {"other"}, {"fit"}, {"search"}, {"fit", "-out", "unused", "-epochs", "0", "unknown"}} {
		if err := run(ctx, args, &bytes.Buffer{}); err == nil {
			t.Fatal("invalid command accepted", args)
		}
	}
	d = fixture()
	d.Seed = "seed"
	path := saveDocument(t, t.TempDir(), "seed.json", d)
	if err := run(ctx, []string{"search", path}, &bytes.Buffer{}); err == nil {
		t.Fatal("seed silently ignored")
	}
}
