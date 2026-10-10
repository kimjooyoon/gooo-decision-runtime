package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func conditionOnlyGoal(expected bool) pathplan.Document {
	d := fixture()
	d.Plan.Base.Expressions[1].Int = 0
	d.Plan.Base.Statements[0].Expr = 1
	d.Plan.Decisions[0] = pathplan.Choice{ID: "comparison", Kind: pathplan.OperandOrder, Target: 2,
		Intent: "조건을 만족한다 / satisfy the condition", Fallback: "layout_forward",
		Options: []pathplan.Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}}
	d.Plan.ConditionCases = []pathplan.ConditionCase{{ChoiceID: "comparison", Input: -9007199254740995, Expected: expected}}
	d.TestCases = []pathplan.TestCase{{Input: -9007199254740995, Expected: 0}, {Input: 9007199254740993, Expected: 0}}
	return d
}

func TestInputAuditExposesConditionOnlyGoalCollision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	samples := make([]contractdecision.Sample, 2)
	for i, expected := range []bool{true, false} {
		d := conditionOnlyGoal(expected)
		p, err := d.Prepare()
		if err != nil {
			t.Fatal(err)
		}
		samples[i], err = sample(ctx, d, p)
		if err != nil {
			t.Fatal(err)
		}
	}
	if samples[0].Acceptable != 1 || samples[1].Acceptable != 2 {
		t.Fatal("Gooo condition checks did not produce opposite complete labels", samples)
	}
	audit, err := contractdecision.AuditInputs(ctx, samples)
	if err != nil || len(audit.Groups) != 1 || audit.BestFirstPasses != 1 ||
		audit.UnavoidableMisses != 1 || audit.ConflictingGroups != 1 {
		t.Fatal("condition-only requirements were not diagnosed", audit, err)
	}
}

func TestLocalFitReportsInputConflictsAndStillFits(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	a := saveDocument(t, dir, "true.json", conditionOnlyGoal(true))
	b := saveDocument(t, dir, "false.json", conditionOnlyGoal(false))
	var out bytes.Buffer
	if err := run(ctx, []string{"fit", "-choice-context", "-epochs", "1",
		"-out", filepath.Join(dir, "model.json"), a, b}, &out); err != nil {
		t.Fatal("diagnostic became a training gate", err)
	}
	var receipt struct {
		Audit *contractdecision.InputAudit `json:"input_audit"`
	}
	if err := json.Unmarshal(out.Bytes(), &receipt); err != nil || receipt.Audit == nil ||
		receipt.Audit.Samples != 2 || receipt.Audit.BestFirstPasses != 1 || receipt.Audit.UnavoidableMisses != 1 {
		t.Fatal("fit omitted the representation limit", out.String(), err)
	}
}
