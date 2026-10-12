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

func TestSourceRelativeV3CLIInspectSearchAndDeterministicOmission(t *testing.T) {
	doc := orderedDocument(false)
	doc.Plan.Base.Expressions[3].Operation = "divide"
	doc.Plan.Base.Expressions[4].Operation = "add"
	doc.TestCases = []pathplan.TestCase{{Input: -5, Expected: 0}, {Input: 14, Expected: 27}, {Input: 23, Expected: 36}}
	dir := t.TempDir()
	name := saveDocument(t, dir, "output-relations.json", doc)
	var out bytes.Buffer
	if err := run(context.Background(), []string{"inspect", "-source-relative-v3-source", name}, &out); err != nil {
		t.Fatal(err)
	}
	var inspection relativeV2Inspection
	if err := json.Unmarshal(out.Bytes(), &inspection); err != nil || inspection.Schema != "gooo/contract-input-inspection/v7" ||
		inspection.SourceVersion != contractdecision.SourceRelativeV3FeatureVersion || inspection.ModelCalls != 0 ||
		inspection.CandidateExecutions != 0 || inspection.TrainingUpdates != 0 || len(inspection.Outputs) != 3 {
		t.Fatal("zero-call V3 inspection", err)
	}
	for _, choice := range inspection.Choices {
		if choice.Outputs == nil || !choice.Outputs.Available || choice.Outputs.CaseCount != 3 || choice.Features == nil ||
			choice.Features[208]+choice.Features[224]+choice.Features[240] < .99999 {
			t.Fatal("complete declared-output relationships missing")
		}
	}
	model, err := contractdecision.NewSourceRelativeV3([contractdecision.InteractionRequirementParameterCount]float32{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := model.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	modelPath := filepath.Join(dir, "zero-v3.json")
	if err := os.WriteFile(modelPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, args := range [][]string{{"search", "-model", modelPath, name}, {"search", name}} {
		out.Reset()
		if err := run(ctx, args, &out); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Ranking pathplan.ContractRanking
			Result  pathplan.ContractProgress
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Result.Status != "TRAINING_COMPLETE" ||
			result.Result.SelectedPassed != 3 || result.Result.PredictionsThisAdvance != 0 {
			t.Fatal("finite V3/deterministic construction", err)
		}
		calls := 0
		if len(args) > 2 {
			calls = 1
		}
		if result.Ranking.Calls != calls {
			t.Fatal("unexpected prediction count")
		}
	}
	out.Reset()
	if err := run(ctx, []string{"inspect", "-source-relative-v3-source", "-source-relative-v2-source", name}, &out); err == nil || out.Len() != 0 {
		t.Fatal("mixed ABI inspection produced output")
	}
}
