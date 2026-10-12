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

func TestSourceRelativeV4CLIInspectSearchAndDeterministicOmission(t *testing.T) {
	doc := orderedDocument(false)
	doc.Plan.Base.Expressions[3].Operation = "divide"
	doc.Plan.Base.Expressions[4].Operation = "add"
	doc.TestCases = []pathplan.TestCase{{Input: -5, Expected: 0}, {Input: 14, Expected: 27}, {Input: 23, Expected: 36}}
	dir := t.TempDir()
	name := saveDocument(t, dir, "branch-goals.json", doc)
	var out bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := run(ctx, []string{"inspect", "-source-relative-v4-source", name}, &out); err != nil {
		t.Fatal(err)
	}
	var inspection relativeV2Inspection
	if err := json.Unmarshal(out.Bytes(), &inspection); err != nil || inspection.Schema != "gooo/contract-input-inspection/v9" ||
		inspection.SourceVersion != contractdecision.SourceRelativeV4FeatureVersion || inspection.ModelCalls != 0 ||
		inspection.CandidateExecutions != 0 || inspection.TrainingUpdates != 0 || len(inspection.Outputs) != 3 {
		t.Fatal("zero-call V4 inspection", err)
	}
	for _, choice := range inspection.Choices {
		if choice.Branches == nil || !choice.Branches.Available || choice.Branches.CaseCount != 3 || choice.Features == nil {
			t.Fatal("missing branch relationships")
		}
	}
	m, _ := contractdecision.NewSourceRelativeV4([contractdecision.InteractionRequirementParameterCount]float32{})
	raw, _ := m.Marshal()
	modelPath := filepath.Join(dir, "zero-v4.json")
	if err := os.WriteFile(modelPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"search", "-model", modelPath, name}, {"search", name}} {
		out.Reset()
		if err := run(ctx, args, &out); err != nil {
			t.Fatal(err)
		}
		var report struct {
			Ranking pathplan.ContractRanking
			Result  pathplan.ContractProgress
		}
		if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Result.Status != "TRAINING_COMPLETE" ||
			report.Result.SelectedPassed != 3 || report.Result.PredictionsThisAdvance != 0 {
			t.Fatal("finite V4/deterministic search", err)
		}
		wantCalls := 0
		if len(args) > 2 {
			wantCalls = 1
			if report.Ranking.SourceFeatures != contractdecision.SourceRelativeV4FeatureVersion {
				t.Fatal("V4 model was dispatched to another encoder")
			}
		}
		if report.Ranking.Calls != wantCalls {
			t.Fatal("prediction count", report.Ranking.Calls)
		}
	}
	for _, other := range []string{"-ordered-source", "-canonical-source", "-condition-relations", "-source-relative-source", "-source-relative-v2-source", "-source-relative-v3-source", "-source-relative-v3-prose-neutral-source"} {
		out.Reset()
		if err := run(ctx, []string{"inspect", "-source-relative-v4-source", other, name}, &out); err == nil || out.Len() != 0 {
			t.Fatal("mixed V4 ABI inspection produced output", other)
		}
	}
}
