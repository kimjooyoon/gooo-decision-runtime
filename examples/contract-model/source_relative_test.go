package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func TestInspectSourceRelativeConditionsIsOptionalAndSourceBound(t *testing.T) {
	dir := t.TempDir()
	d := orderedDocument(false)
	d.Plan.ConditionCases = []pathplan.ConditionCase{{ChoiceID: d.Plan.Decisions[0].ID, Input: 0, Expected: true}}
	path := saveDocument(t, dir, "relations.json", d)
	for _, enabled := range []bool{false, true} {
		args := []string{"inspect", "-canonical-source"}
		if enabled {
			args = append(args, "-condition-relations")
		}
		var out bytes.Buffer
		if err := run(context.Background(), append(args, path), &out); err != nil {
			t.Fatal(err)
		}
		var report contractInputInspection
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if report.ModelCalls != 0 || report.TrainingUpdates != 0 || report.CandidateExecutions != 0 {
			t.Fatal("inspection ran a model or candidate")
		}
		for _, choice := range report.Choices {
			view := choice.ConditionRelations
			if !enabled {
				if view != nil || report.Schema != "gooo/contract-input-inspection/v3" {
					t.Fatal("existing inspection changed")
				}
				continue
			}
			if report.Schema != "gooo/contract-input-inspection/v4" || view == nil || !view.Available ||
				view.FeatureVersion != pathplan.SourceRelativeConditionFeatureVersion || view.PlanSHA256 != report.PlanSHA256 ||
				view.ChoiceID != choice.ID || view.Count != 1 || view.Buckets != ([3][2]uint16{{0, 1}}) {
				t.Fatal("missing source-bound relations", report)
			}
		}
	}
}
