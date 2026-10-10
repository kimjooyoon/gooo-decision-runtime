package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestInspectDistinguishesConditionGoalsWithoutTrainingOrExecution(t *testing.T) {
	dir := t.TempDir()
	var reports [2]contractInputInspection
	for i, expected := range []bool{true, false} {
		doc := conditionOnlyGoal(expected)
		name := []string{"true.json", "false.json"}[i]
		path := saveDocument(t, dir, name, doc)
		var out bytes.Buffer
		if err := run(context.Background(), []string{"inspect", path}, &out); err != nil {
			t.Fatal(err)
		}
		report := &reports[i]
		if err := json.Unmarshal(out.Bytes(), report); err != nil || report.Schema != "gooo/contract-input-inspection/v1" ||
			report.ConditionFeatureFormat != decision.DeclaredConditionFeatureVersion || report.ModelCalls != 0 ||
			report.CandidateExecutions != 0 || report.TrainingUpdates != 0 || len(report.Conditions) != 1 {
			t.Fatal("inspection scope changed", out.String(), err)
		}
		if report.Conditions[0].Case != doc.Plan.ConditionCases[0] || len(report.Outputs) != len(doc.TestCases) ||
			report.Outputs[0].Case != doc.TestCases[0] || report.Conditions[0].Features[16] != 1.0/8 {
			t.Fatal("exact source values changed", report)
		}
	}
	a, b := reports[0], reports[1]
	if !reflect.DeepEqual(a.Choices, b.Choices) || !reflect.DeepEqual(a.Outputs, b.Outputs) ||
		a.OutputCasesSHA256 != b.OutputCasesSHA256 || a.PlanSHA256 == b.PlanSHA256 ||
		a.Conditions[0].Features == b.Conditions[0].Features {
		t.Fatal("condition-only distinction was lost or rewritten into old inputs")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatal("inspection created a model or output file", entries, err)
	}
}

func TestInspectInputBoundsCancellationAndEmptyConditions(t *testing.T) {
	dir := t.TempDir()
	path := saveDocument(t, dir, "plain.json", fixture())
	for _, args := range [][]string{{"inspect"}, {"inspect", path, path}, {"inspect", "-model", path}} {
		var out bytes.Buffer
		if err := run(context.Background(), args, &out); err == nil || out.Len() != 0 {
			t.Fatal("invalid inspection accepted", args, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	if err := run(ctx, []string{"inspect", path}, &out); !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatal("cancelled inspection", err)
	}
	if err := run(nil, []string{"inspect", path}, &out); err == nil || out.Len() != 0 {
		t.Fatal("nil context accepted")
	}
	if err := run(context.Background(), []string{"inspect", path}, &out); err != nil {
		t.Fatal(err)
	}
	var report contractInputInspection
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Conditions == nil || len(report.Conditions) != 0 {
		t.Fatal("empty conditions acquired a fabricated row", err, out.String())
	}
}
