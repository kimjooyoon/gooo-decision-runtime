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
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func TestSourceLiteralFitAndImmediateSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	document := saveDocument(t, dir, "training.json", fixture())
	modelPath := filepath.Join(dir, "model.json")
	var out bytes.Buffer
	if err := run(ctx, []string{"fit", "-case-features", decision.SourceLiteralCaseFeatureVersion, "-epochs", "2", "-out", modelPath, document}, &out); err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Schema  string `json:"schema"`
		Version string `json:"case_feature_version"`
	}
	if err := json.Unmarshal(out.Bytes(), &receipt); err != nil || receipt.Schema != "gooo/contract-local-fit/v3" || receipt.Version != decision.SourceLiteralCaseFeatureVersion {
		t.Fatal(out.String(), err)
	}
	raw, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	model, err := contractdecision.Decode(raw)
	if err != nil || model.ArtifactSchema() != contractdecision.SourceLiteralSchema {
		t.Fatal(err)
	}
	out.Reset()
	if err := run(ctx, []string{"search", "-model", modelPath, document}, &out); err != nil {
		t.Fatal(err)
	}
	var search struct {
		Ranking pathplan.ContractRanking  `json:"ranking"`
		Result  pathplan.ContractProgress `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &search); err != nil || search.Ranking.CaseFeatures != decision.SourceLiteralCaseFeatureVersion || search.Ranking.Calls != 1 || search.Result.Status != "TRAINING_COMPLETE" || search.Result.SelectedPassed != 2 {
		t.Fatal(out.String(), err)
	}
	for _, args := range [][]string{
		{"fit", "-out", filepath.Join(dir, "bad.json"), "-case-features", "other", document},
		{"fit", "-out", filepath.Join(dir, "bad.json"), "-case-features", decision.SourceLiteralCaseFeatureVersion, "-goal-pairs", document, document},
	} {
		if err := run(ctx, args, &out); err == nil {
			t.Fatal("unsupported feature combination accepted")
		}
	}
}
