package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func TestProseNeutralV3CLIInspectionRepackagingAndSearch(t *testing.T) {
	doc := orderedDocument(false)
	doc.Plan.Base.Expressions[3].Operation = "divide"
	doc.Plan.Base.Expressions[4].Operation = "add"
	doc.TestCases = []pathplan.TestCase{{Input: -5, Expected: 0}, {Input: 14, Expected: 27}, {Input: 23, Expected: 36}}
	dir := t.TempDir()
	name := saveDocument(t, dir, "new-neutral-document.json", doc)
	var out bytes.Buffer
	if err := run(context.Background(), []string{"inspect", "-source-relative-v3-prose-neutral-source", name}, &out); err != nil {
		t.Fatal(err)
	}
	var inspection relativeV2Inspection
	if err := json.Unmarshal(out.Bytes(), &inspection); err != nil || inspection.Schema != "gooo/contract-input-inspection/v8" || inspection.SourceVersion != contractdecision.SourceRelativeV3ProseNeutralFeatureVersion || inspection.IntentPolicy != contractdecision.SourceRelativeV3ProseNeutralIntentPolicy || inspection.ClearedIntentCells == nil || *inspection.ClearedIntentCells != ([2]int{64, 192}) || inspection.ModelCalls != 0 || inspection.CandidateExecutions != 0 || inspection.TrainingUpdates != 0 || len(inspection.Outputs) != len(doc.TestCases) || len(inspection.Conditions) != len(doc.Plan.ConditionCases) {
		t.Fatal("explicit zero-call inspection", err)
	}
	for _, choice := range inspection.Choices {
		if choice.Features == nil || choice.Outputs == nil || choice.Outputs.CaseCount != 3 {
			t.Fatal("declared output view lost")
		}
		for _, value := range choice.Features[64:192] {
			if value != 0 {
				t.Fatal("intent channel populated")
			}
		}
	}
	for i, row := range inspection.Outputs {
		if row.Case != doc.TestCases[i] {
			t.Fatal("numeric requirement changed", i)
		}
	}
	for i, row := range inspection.Conditions {
		if row.Case != doc.Plan.ConditionCases[i] {
			t.Fatal("Boolean requirement changed", i)
		}
	}
	for _, flag := range []string{"-ordered-source", "-canonical-source", "-condition-relations", "-source-relative-source", "-source-relative-v2-source", "-source-relative-v3-source"} {
		out.Reset()
		if err := run(context.Background(), []string{"inspect", "-source-relative-v3-prose-neutral-source", flag, name}, &out); err == nil || out.Len() != 0 {
			t.Fatal("mixed inspection ABI emitted output", flag)
		}
	}
	m, err := contractdecision.NewSourceRelativeV3([contractdecision.InteractionRequirementParameterCount]float32{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	input, packed := filepath.Join(dir, "zero-v3.json"), filepath.Join(dir, "explicit-neutral.json")
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	args := []string{"repackage-prose-neutral", "-input", input, "-out", packed}
	if err := run(context.Background(), args, &out); err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		ModelSHA            string `json:"model_sha256"`
		SourceV3SHA         string `json:"source_v3_fingerprint"`
		InputSHA            string `json:"input_sha256"`
		ModelCalls          int    `json:"model_calls"`
		CandidateExecutions int    `json:"candidate_executions"`
		TrainingUpdates     int    `json:"training_updates"`
	}
	if err := json.Unmarshal(out.Bytes(), &receipt); err != nil || receipt.SourceV3SHA != m.Fingerprint() || receipt.InputSHA != fmt.Sprintf("%x", sha256.Sum256(raw)) || receipt.ModelCalls != 0 || receipt.CandidateExecutions != 0 || receipt.TrainingUpdates != 0 {
		t.Fatal("zero-call repackaging receipt", err)
	}
	converted, err := os.ReadFile(packed)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ModelSHA != fmt.Sprintf("%x", sha256.Sum256(converted)) {
		t.Fatal("converted bytes differ from receipt")
	}
	out.Reset()
	if err := run(context.Background(), args, &out); err == nil || out.Len() != 0 {
		t.Fatal("existing model overwritten")
	}
	still, err := os.ReadFile(packed)
	if err != nil || !bytes.Equal(still, converted) {
		t.Fatal("existing artifact mutated")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, searchArgs := range [][]string{{"search", "-model", packed, name}, {"search", name}} {
		out.Reset()
		if err := run(ctx, searchArgs, &out); err != nil {
			t.Fatal(err)
		}
		var report struct {
			Ranking pathplan.ContractRanking
			Result  pathplan.ContractProgress
		}
		if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Result.Status != "TRAINING_COMPLETE" || report.Result.SelectedPassed != 3 || report.Result.PredictionsThisAdvance != 0 {
			t.Fatal("finite construction", err)
		}
		calls := 0
		if len(searchArgs) > 2 {
			calls = 1
		}
		if report.Ranking.Calls != calls {
			t.Fatal("unexpected model calls")
		}
		if calls == 1 && report.Ranking.SourceFeatures != contractdecision.SourceRelativeV3ProseNeutralFeatureVersion {
			t.Fatal("model route ignored neutral policy")
		}
	}
}

func TestProseNeutralV3CLIInvalidContextsWriteNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	doc := saveDocument(t, dir, "cancel-document.json", orderedDocument(false))
	for _, invalid := range []context.Context{nil, ctx} {
		for _, args := range [][]string{{"inspect", "-source-relative-v3-prose-neutral-source", doc}, {"repackage-prose-neutral", "-input", doc, "-out", filepath.Join(dir, "must-not-exist.json")}} {
			var out bytes.Buffer
			if err := run(invalid, args, &out); err == nil || out.Len() != 0 {
				t.Fatal("invalid context emitted output")
			}
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "must-not-exist.json")); !os.IsNotExist(err) {
		t.Fatal("invalid context created artifact", err)
	}
}
