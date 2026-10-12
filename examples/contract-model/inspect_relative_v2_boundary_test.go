package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestSourceRelativeV2CLIInspectEmptyConditionsAndCancellation(t *testing.T) {
	doc := orderedDocument(false)
	doc.Plan.ConditionCases = nil
	name := saveDocument(t, t.TempDir(), "empty-conditions.json", doc)
	args := []string{"inspect", "-source-relative-v2-source", name}
	var out bytes.Buffer
	if err := run(context.Background(), args, &out); err != nil {
		t.Fatal(err)
	}
	var report relativeV2Inspection
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Conditions == nil || len(report.Conditions) != 0 || len(report.Outputs) != len(doc.TestCases) ||
		report.ModelCalls != 0 || report.CandidateExecutions != 0 || report.TrainingUpdates != 0 {
		t.Fatal("empty requirement list or inspection counters changed", out.String())
	}
	for i, row := range report.Outputs {
		if row.Case != doc.TestCases[i] {
			t.Fatal("inspection changed declared output", i)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, test := range []struct {
		name string
		ctx  context.Context
	}{{"cancelled", ctx}, {"nil-context", nil}} {
		t.Run(test.name, func(t *testing.T) {
			out.Reset()
			err := run(test.ctx, args, &out)
			if err == nil || out.Len() != 0 || (test.ctx != nil && !errors.Is(err, context.Canceled)) {
				t.Fatal("invalid context emitted an inspection", err, out.String())
			}
		})
	}
}

func TestSourceRelativeV2CLIInspectUnsupportedContextKeepsReasons(t *testing.T) {
	doc := orderedDocument(false)
	doc.Plan.Base.Expressions[4].Left = 3 // Nested arithmetic lies outside the model input.
	name := saveDocument(t, t.TempDir(), "nested.json", doc)
	var out bytes.Buffer
	if err := run(context.Background(), []string{"inspect", "-source-relative-v2-source", name}, &out); err != nil {
		t.Fatal(err)
	}
	var report relativeV2Inspection
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Choices) != len(doc.Plan.Decisions) || len(report.Outputs) != len(doc.TestCases) ||
		len(report.Conditions) != len(doc.Plan.ConditionCases) || report.ModelCalls != 0 ||
		report.CandidateExecutions != 0 || report.TrainingUpdates != 0 {
		t.Fatal("unsupported source lost requirements or executed work", out.String())
	}
	for _, choice := range report.Choices {
		if choice.Source.Available || choice.Source.Reason == "" || choice.Relations.Available ||
			choice.Relations.Reason == "" || choice.Features != nil {
			t.Fatal("unsupported model input concealed its reason", choice)
		}
	}
	for i, row := range report.Conditions {
		if row.Case != doc.Plan.ConditionCases[i] {
			t.Fatal("inspection changed declared condition", i)
		}
	}
	for i, row := range report.Outputs {
		if row.Case != doc.TestCases[i] {
			t.Fatal("unsupported context changed declared output", i)
		}
	}
}
