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

func TestSourceRelativeCLIInspectAndSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	model, err := contractdecision.NewSourceRelativeRequirementConditioned([contractdecision.InteractionRequirementParameterCount]float32{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := model.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	modelPath := filepath.Join(dir, "relative.json")
	if err := os.WriteFile(modelPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var inspections [2]contractInputInspection
	var rankings [2]pathplan.ContractRanking
	for i, d := range []pathplan.Document{orderedDocument(false), canonicalAssignmentDocument()} {
		name := []string{"direct.json", "assignment.json"}[i]
		path := saveDocument(t, dir, name, d)
		var out bytes.Buffer
		if err := run(ctx, []string{"inspect", "-source-relative-source", path}, &out); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(out.Bytes(), &inspections[i]); err != nil {
			t.Fatal(err)
		}
		got := inspections[i]
		if got.Schema != "gooo/contract-input-inspection/v5" || got.RelativeFeatureFormat != contractdecision.SourceRelativeOrderedSourceFeatureVersion || got.ModelCalls != 0 || got.CandidateExecutions != 0 || got.TrainingUpdates != 0 {
			t.Fatal("inspection scope", got)
		}
		c := got.Choices[0]
		if c.RelativeFeatures == nil || c.ConditionRelations == nil || !c.ConditionRelations.Available {
			t.Fatal("relative input absent")
		}
		for j, v := range c.ConditionRelations.Features {
			if c.RelativeFeatures[192+j] != v {
				t.Fatal("feature layout differs")
			}
		}
		out.Reset()
		if err := run(ctx, []string{"search", "-model", modelPath, path}, &out); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Ranking         pathplan.ContractRanking `json:"ranking"`
			Initial, Result pathplan.ContractProgress
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Ranking.Calls != 1 || result.Initial.Attempted != 0 || result.Result.Status != "TRAINING_COMPLETE" || result.Result.PredictionsThisAdvance != 0 || result.Result.Selection.ModelVariant != "source_relative_requirement_contract_fp32" {
			t.Fatal("runtime route", out.String())
		}
		rankings[i] = result.Ranking
	}
	if *inspections[0].Choices[0].RelativeFeatures != *inspections[1].Choices[0].RelativeFeatures || rankings[0].FeatureSHA != rankings[1].FeatureSHA || rankings[0].Logits != rankings[1].Logits {
		t.Fatal("equivalent source forms changed runtime input")
	}
}

func TestSourceRelativeCLIReportsUnsupportedInputAndRejectsBadWeights(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	d := orderedDocument(false)
	d.Plan.Base.Expressions[4].Left = 3
	path := saveDocument(t, dir, "nested.json", d)
	var out bytes.Buffer
	if err := run(ctx, []string{"inspect", "-source-relative-source", path}, &out); err != nil {
		t.Fatal(err)
	}
	var input contractInputInspection
	if err := json.Unmarshal(out.Bytes(), &input); err != nil {
		t.Fatal(err)
	}
	if input.Choices[0].ConditionRelations == nil || input.Choices[0].ConditionRelations.Available || input.Choices[0].RelativeFeatures != nil {
		t.Fatal("unsupported context concealed")
	}
	model, err := contractdecision.NewSourceRelativeRequirementConditioned([contractdecision.InteractionRequirementParameterCount]float32{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := model.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	modelPath := filepath.Join(dir, "relative.json")
	if err := os.WriteFile(modelPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run(ctx, []string{"search", "-model", modelPath, path}, &out); err == nil || err.Error() != "finite search budget ended before every declared requirement passed" {
		t.Fatal("unsupported fixture must retain its incomplete deterministic result", err)
	}
	var result struct {
		Ranking pathplan.ContractRanking  `json:"ranking"`
		Result  pathplan.ContractProgress `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Ranking.Declined || result.Ranking.Calls != 0 || result.Ranking.PredictNS != 0 || result.Result.Status != "PARTIAL" {
		t.Fatal("unsupported source predicted", result)
	}
	bad := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(bad, []byte(`{"schema":"gooo/source-relative-condition-research-model/v1","weights":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run(ctx, []string{"search", "-model", bad, path}, &out); err == nil || out.Len() != 0 {
		t.Fatal("malformed model accepted", err)
	}
}
