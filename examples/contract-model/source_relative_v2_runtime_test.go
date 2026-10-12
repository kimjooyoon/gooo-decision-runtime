package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

// A fresh zero-weight model and authored synthetic document, no quality refit.
func TestSourceRelativeV2CLIInspectImmediateSearchAndDeterministicUse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	doc := orderedDocument(false)
	doc.Plan.Base.Expressions[3].Operation = "divide"
	doc.Plan.Base.Expressions[4].Operation = "add"
	doc.TestCases = []pathplan.TestCase{{Input: -4, Expected: 0}, {Input: 13, Expected: 26}, {Input: 20, Expected: 33}}
	demo, err := os.ReadFile("testdata/source-relative-v2-division.json")
	if err != nil {
		t.Fatal(err)
	}
	declared, err := pathplan.DecodeDocument(demo)
	if err != nil || !reflect.DeepEqual(declared, doc) {
		t.Fatal("demo document differs from its authored SDK fixture", err)
	}
	dir := t.TempDir()
	name := saveDocument(t, dir, "division.json", doc)
	var out bytes.Buffer
	if err := run(context.Background(), []string{"inspect", "-source-relative-v2-source", name}, &out); err != nil {
		t.Fatal(err)
	}
	var r relativeV2Inspection
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Schema != "gooo/contract-input-inspection/v6" || r.SourceVersion != contractdecision.SourceRelativeV2FeatureVersion || r.ModelCalls != 0 || r.CandidateExecutions != 0 || r.TrainingUpdates != 0 || len(r.Outputs) != 3 {
		t.Fatal("V2 inspection scope differs")
	}
	for _, choice := range r.Choices {
		if choice.Features == nil || !choice.Source.Available || !choice.Relations.Available {
			t.Fatal("V2 division input declined")
		}
	}
	m, err := contractdecision.NewSourceRelativeV2([contractdecision.InteractionRequirementParameterCount]float32{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(dir, "zero-v2.json")
	if err = os.WriteFile(model, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"search", "-model", model, name}, {"search", name}} {
		out.Reset()
		if err = run(ctx, args, &out); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Ranking pathplan.ContractRanking
			Result  pathplan.ContractProgress
		}
		if err = json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		calls := 0
		if len(args) > 2 {
			calls = 1
		}
		if result.Ranking.Calls != calls || result.Result.Status != "TRAINING_COMPLETE" || result.Result.SelectedPassed != 3 || result.Result.PredictionsThisAdvance != 0 {
			t.Fatal("V2 or deterministic search differs", out.String())
		}
	}
	out.Reset()
	if err = run(context.Background(), []string{"inspect", "-source-relative-v2-source", "-source-relative-source", name}, &out); err == nil {
		t.Fatal("mixed V1/V2 inspection accepted")
	}
	bad := bytes.Replace(raw, []byte(contractdecision.SourceRelativeV2FeatureVersion), []byte(contractdecision.SourceRelativeOrderedSourceFeatureVersion), 1)
	if err = os.WriteFile(model, bad, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err = run(ctx, []string{"search", "-model", model, name}, &out); err == nil || strings.Contains(out.String(), "TRAINING_COMPLETE") {
		t.Fatal("incompatible V2 input silently used deterministic search")
	}
}
