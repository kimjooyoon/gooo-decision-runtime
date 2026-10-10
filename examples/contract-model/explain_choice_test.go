package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func saveChoiceExplanationModel(t *testing.T, root string) (string, *contractdecision.ChoiceModel) {
	t.Helper()
	var weights [contractdecision.ParameterCount]float32
	var contextWeights [contractdecision.ChoiceContextParameterCount]float32
	for i := range weights {
		weights[i] = float32(i%17-8) / 1000
	}
	for i := range contextWeights {
		contextWeights[i] = float32(i%11-5) / 1000
	}
	m, err := contractdecision.NewChoiceConditioned(weights, contextWeights, contractdecision.ExtremePooling)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(root, "choice-model.json")
	if err := os.WriteFile(name, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return name, m
}

func TestExplainCommandReadsExactGoalsWithoutCandidateExecution(t *testing.T) {
	root := t.TempDir()
	name, model := saveChoiceExplanationModel(t, root)
	doc := fixture()
	// These goals contradict each other; explanation still reads them without
	// running a search, accepting a body or replacing the author's requirements.
	doc.TestCases = append(doc.TestCases, doc.TestCases[1])
	doc.TestCases[2].Expected++
	path := saveDocument(t, root, "goals.json", doc)
	var out bytes.Buffer
	if err := run(context.Background(), []string{"explain", "-model", name, path}, &out); err != nil {
		t.Fatal(err)
	}
	var report choiceExplanationReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Schema != "gooo/choice-contract-explanation/v1" || report.ModelCalls != 1 || report.CandidateExecutions != 0 || report.TrainingUpdates != 0 || report.EnumeratedMasks != 0 {
		t.Fatal("explanation scope", err, out.String())
	}
	if !reflect.DeepEqual(report.DeclaredCases, doc.TestCases) || report.ModelFingerprint != model.Fingerprint() || len(report.Choices) != 1 || report.PredictNS <= 0 {
		t.Fatal("exact goals or model identity changed")
	}
	prepared, err := doc.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	input, err := prepared.InitialContractInput(doc.TestCases)
	if err != nil {
		t.Fatal(err)
	}
	var w contractdecision.ChoiceWorkspace
	var prediction contractdecision.ChoicePrediction
	if err := model.PredictChoicesInto([][contractdecision.FeatureDim]float32{report.Choices[0].Features}, input, &w, &prediction); err != nil || prediction.Logits[0] != report.Choices[0].Computation.OptionScores {
		t.Fatal("command changed model computation", err)
	}
	if report.Choices[0].ID != doc.Plan.Decisions[0].ID || report.Choices[0].Options[1] != doc.Plan.Decisions[0].Options[1].Label || report.CaseSHA256 != input.CaseSHA256() || report.PlanSHA256 != input.PlanSHA256() {
		t.Fatal("source choice binding")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatal("explanation created files", err)
	}
}

func TestExplainCommandBoundsAndCancellation(t *testing.T) {
	root := t.TempDir()
	name, _ := saveChoiceExplanationModel(t, root)
	path := saveDocument(t, root, "goals.json", fixture())
	for _, args := range [][]string{{"explain"}, {"explain", path}, {"explain", "-model", name}, {"explain", "-model", name, path, path}, {"explain", "-model", path, path}} {
		var out bytes.Buffer
		if err := run(context.Background(), args, &out); err == nil || out.Len() != 0 {
			t.Fatal("bad explanation request accepted", args, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	if err := run(ctx, []string{"explain", "-model", name, path}, &out); !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatal("canceled explanation", err)
	}
	if err := run(nil, []string{"explain", "-model", name, path}, &out); err == nil || out.Len() != 0 {
		t.Fatal("nil context accepted")
	}
}
