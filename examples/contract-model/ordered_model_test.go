package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func orderedDocument(reverse bool) pathplan.Document {
	d := fixture()
	d.Plan.Base.Expressions[1].Int = 13
	d.Plan.Base.Expressions = append(d.Plan.Base.Expressions,
		bodyplan.Expr{Kind: "binary", Operation: "subtract", Left: 0, Right: 1},
		bodyplan.Expr{Kind: "binary", Operation: "subtract", Left: 1, Right: 0})
	d.Plan.Base.Statements[0].Expr, d.Plan.Base.Statements[1].Expr = 3, 4
	if reverse {
		d.Plan.Base.Statements[0].Expr, d.Plan.Base.Statements[1].Expr = 4, 3
	}
	d.TestCases = []pathplan.TestCase{{Input: -4, Expected: 17}, {Input: 13, Expected: 0}, {Input: 20, Expected: 7}}
	d.Plan.ConditionCases = []pathplan.ConditionCase{{ChoiceID: "branches", Input: -4, Expected: true}}
	return d
}

func TestOrderedRequirementCLIArtifactAuditAndImmediateSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	a := saveDocument(t, dir, "forward.json", orderedDocument(false))
	b := saveDocument(t, dir, "reverse.json", orderedDocument(true))
	model := filepath.Join(dir, "ordered.json")
	var out bytes.Buffer
	if err := run(ctx, []string{"fit", "-ordered-requirements", "-epochs", "2000", "-out", model, a, b}, &out); err != nil {
		t.Fatal(err)
	}
	var fit struct {
		Schema    string                       `json:"schema"`
		Source    string                       `json:"source_feature_version"`
		Documents int                          `json:"training_documents"`
		Audit     *contractdecision.InputAudit `json:"input_audit"`
	}
	if err := json.Unmarshal(out.Bytes(), &fit); err != nil || fit.Schema != "gooo/ordered-requirement-local-fit/v1" ||
		fit.Source != contractdecision.OrderedSourceFeatureVersion || fit.Documents != 2 || fit.Audit == nil || fit.Audit.BestFirstPasses != 2 {
		t.Fatal(out.String(), err)
	}
	raw, err := os.ReadFile(model)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := contractdecision.DecodeOrderedRequirementConditioned(raw); err != nil {
		t.Fatal(err)
	}
	for i, path := range []string{a, b} {
		out.Reset()
		if err := run(ctx, []string{"search", "-model", model, path}, &out); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Ranking pathplan.ContractRanking  `json:"ranking"`
			Initial pathplan.ContractProgress `json:"initial"`
			Result  pathplan.ContractProgress `json:"result"`
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Ranking.Calls != 1 || result.Ranking.Proposed != uint16(1-i) ||
			result.Initial.Attempted != 0 || result.Result.Attempted != 1 || result.Result.Status != "TRAINING_COMPLETE" || result.Result.SelectedPassed != 3 {
			t.Fatal("CLI did not consume ordered source before construction", out.String(), err)
		}
	}
	for _, flags := range [][]string{{"-requirements"}, {"-choice-context"}, {"-goal-pairs"}, {"-case-features", "source_literal_integer_case_v1"}} {
		args := append([]string{"fit", "-ordered-requirements", "-out", filepath.Join(dir, "unused.json")}, flags...)
		args = append(args, a, b)
		out.Reset()
		if err := run(ctx, args, &out); err == nil || out.Len() != 0 {
			t.Fatal("incompatible model options accepted", flags, err)
		}
	}
}
