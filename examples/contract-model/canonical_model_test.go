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

func canonicalAssignmentDocument() pathplan.Document {
	d := orderedDocument(false)
	d.Plan.Base.Expressions = append(d.Plan.Base.Expressions, bodyplan.Expr{Kind: "local", Name: "result"})
	d.Plan.Base.Statements[0] = bodyplan.Stmt{Kind: "assign", Name: "result", Expr: 3}
	d.Plan.Base.Statements[1] = bodyplan.Stmt{Kind: "assign", Name: "result", Expr: 4}
	d.Plan.Base.Statements = append(d.Plan.Base.Statements,
		bodyplan.Stmt{Kind: "let", Name: "result", Expr: 0}, bodyplan.Stmt{Kind: "return", Expr: 5})
	d.Plan.Base.Root = []int{3, 2, 4}
	return d
}

func TestCanonicalCLIInspectionFitAndSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	paths := []string{saveDocument(t, dir, "direct.json", orderedDocument(false)), saveDocument(t, dir, "assignment.json", canonicalAssignmentDocument())}
	var inspections [2]contractInputInspection
	for i, path := range paths {
		var out bytes.Buffer
		if err := run(ctx, []string{"inspect", "-ordered-source", "-canonical-source", path}, &out); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(out.Bytes(), &inspections[i]); err != nil {
			t.Fatal(err)
		}
		r := inspections[i]
		if r.Schema != "gooo/contract-input-inspection/v3" || r.CanonicalFeatureFormat != contractdecision.CanonicalOrderedSourceFeatureVersion || r.ModelCalls != 0 || r.TrainingUpdates != 0 || r.CandidateExecutions != 0 {
			t.Fatal("inspection scope", r)
		}
		c := r.Choices[0]
		if c.CanonicalFeatures == nil || c.CanonicalContext == nil || !c.CanonicalContext.Available || c.OrderedFeatures == nil {
			t.Fatal("canonical input absent")
		}
	}
	if *inspections[0].Choices[0].CanonicalFeatures != *inspections[1].Choices[0].CanonicalFeatures || inspections[0].Choices[0].Features == inspections[1].Choices[0].Features {
		t.Fatal("new representation or legacy representation changed")
	}
	modelPath := filepath.Join(dir, "canonical.json")
	var out bytes.Buffer
	// Two epochs exercise fitting and file routing, not model quality.
	if err := run(ctx, append([]string{"fit", "-canonical-interactions", "-epochs", "2", "-out", modelPath}, paths...), &out); err != nil {
		t.Fatal(err)
	}
	var fit struct {
		Schema    string                       `json:"schema"`
		Source    string                       `json:"source_feature_version"`
		Documents int                          `json:"training_documents"`
		Audit     *contractdecision.InputAudit `json:"input_audit"`
	}
	if err := json.Unmarshal(out.Bytes(), &fit); err != nil || fit.Schema != "gooo/canonical-interaction-requirement-local-fit/v1" || fit.Source != contractdecision.CanonicalOrderedSourceFeatureVersion || fit.Documents != 2 || fit.Audit == nil || fit.Audit.Schema != contractdecision.CanonicalRequirementInputAuditSchema || fit.Audit.RepeatedGroups != 1 || fit.Audit.ConflictingGroups != 0 {
		t.Fatal("fit metadata", err, out.String())
	}
	raw, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := contractdecision.DecodeCanonicalInteractionRequirementConditioned(raw); err != nil {
		t.Fatal(err)
	}
	var rankings [2]pathplan.ContractRanking
	for i, path := range paths {
		out.Reset()
		if err := run(ctx, []string{"search", "-model", modelPath, path}, &out); err != nil {
			t.Fatal(err)
		}
		var got struct {
			Ranking pathplan.ContractRanking  `json:"ranking"`
			Initial pathplan.ContractProgress `json:"initial"`
			Result  pathplan.ContractProgress `json:"result"`
		}
		if err := json.Unmarshal(out.Bytes(), &got); err != nil || got.Ranking.Calls != 1 || got.Ranking.SourceFeatures != fit.Source || got.Initial.Attempted != 0 || got.Result.Status != "TRAINING_COMPLETE" || got.Result.SelectedPassed != 3 || got.Result.Attempted > 2 || got.Result.PredictionsThisAdvance != 0 {
			t.Fatal("immediate search routing", err, out.String())
		}
		rankings[i] = got.Ranking
	}
	if rankings[0].Logits != rankings[1].Logits || rankings[0].FeatureSHA != rankings[1].FeatureSHA {
		t.Fatal("CLI consumed different input across variable forms")
	}
	for _, flags := range [][]string{{"-interactions"}, {"-ordered-requirements"}, {"-requirements"}, {"-choice-context"}, {"-goal-pairs"}, {"-case-features", "source_literal_integer_case_v1"}} {
		args := append([]string{"fit", "-canonical-interactions", "-out", filepath.Join(dir, "unused.json")}, flags...)
		out.Reset()
		if err := run(ctx, append(args, paths...), &out); err == nil || out.Len() != 0 {
			t.Fatal("mixed model flags accepted", flags, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "unused.json")); !os.IsNotExist(err) {
		t.Fatal("rejected options wrote a model", err)
	}
}

func TestCanonicalCLIUnsupportedSourceIsExplicit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	d := orderedDocument(false)
	d.Plan.Base.Expressions[4].Left = 3
	path := saveDocument(t, dir, "nested.json", d)
	var out bytes.Buffer
	if err := run(ctx, []string{"inspect", "-canonical-source", path}, &out); err != nil {
		t.Fatal(err)
	}
	var report contractInputInspection
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Choices[0].CanonicalContext == nil || report.Choices[0].CanonicalContext.Available || report.Choices[0].CanonicalFeatures != nil {
		t.Fatal("unsupported input concealed", err)
	}
	modelPath := filepath.Join(dir, "absent.json")
	out.Reset()
	if err := run(ctx, []string{"fit", "-canonical-interactions", "-out", modelPath, path}, &out); err == nil || out.Len() != 0 {
		t.Fatal("unsupported fit accepted", err)
	}
	if _, err := os.Stat(modelPath); !os.IsNotExist(err) {
		t.Fatal("unsupported fit wrote model", err)
	}
}
