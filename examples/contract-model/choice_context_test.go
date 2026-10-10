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

func TestChoiceContextLocalFitAndImmediateSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	document := saveDocument(t, dir, "training.json", fixture())
	modelPath := filepath.Join(dir, "model.json")
	var out bytes.Buffer
	if err := run(ctx, []string{"fit", "-choice-context", "-epochs", "2", "-out", modelPath, document}, &out); err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Schema  string `json:"schema"`
		Version string `json:"case_feature_version"`
	}
	if err := json.Unmarshal(out.Bytes(), &receipt); err != nil || receipt.Schema != "gooo/choice-conditioned-local-fit/v1" || receipt.Version != decision.DeclaredCaseFeatureVersion {
		t.Fatal("fit receipt", out.String(), err)
	}
	raw, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	model, err := contractdecision.DecodeChoiceConditioned(raw)
	if err != nil {
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
	if err := json.Unmarshal(out.Bytes(), &search); err != nil || search.Ranking.Calls != 1 || search.Ranking.ModelFingerprint != model.Fingerprint() || search.Result.Status != "TRAINING_COMPLETE" || search.Result.Selection.ModelVariant != "choice_conditioned_contract_fp32" {
		t.Fatal("automatic artifact routing", out.String(), err)
	}
	for _, args := range [][]string{
		{"fit", "-choice-context", "-goal-pairs", "-out", filepath.Join(dir, "bad.json"), document, document},
		{"fit", "-choice-context", "-case-features", decision.SourceLiteralCaseFeatureVersion, "-out", filepath.Join(dir, "bad.json"), document},
	} {
		if err := run(ctx, args, &out); err == nil {
			t.Fatal("unsupported fit composition accepted")
		}
	}
	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx, []string{"search", "-model", empty, document}, &out); err == nil {
		t.Fatal("an explicit empty model silently became deterministic fallback")
	}
}
