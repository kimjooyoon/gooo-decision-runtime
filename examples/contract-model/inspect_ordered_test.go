package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestInspectOrderedSourceOptInKeepsLegacyFeatures(t *testing.T) {
	path := saveDocument(t, t.TempDir(), "source.json", conditionOnlyGoal(true))
	var reports [2]contractInputInspection
	for i, args := range [][]string{{"inspect", path}, {"inspect", "-ordered-source", path}} {
		var out bytes.Buffer
		if err := run(context.Background(), args, &out); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(out.Bytes(), &reports[i]); err != nil {
			t.Fatal(err)
		}
	}
	a, b := reports[0], reports[1]
	if a.Schema != "gooo/contract-input-inspection/v1" || a.Choices[0].OrderedContext != nil ||
		b.Schema != "gooo/contract-input-inspection/v2" || b.OrderedFeatureFormat != decision.OrderedExpressionFeatureVersion ||
		b.Choices[0].OrderedContext == nil || !b.Choices[0].OrderedContext.Available || b.Choices[0].OrderedFeatures == nil ||
		a.Choices[0].Features != b.Choices[0].Features || b.ModelCalls != 0 || b.CandidateExecutions != 0 || b.TrainingUpdates != 0 {
		t.Fatal("inspection lost scope or changed legacy features", b)
	}
}
