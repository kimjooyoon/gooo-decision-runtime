package main

import (
	"context"
	"os"
	"testing"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/executiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func syntheticPair() dataset {
	var d dataset
	for i, split := range []string{"train", "wording"} {
		p := pathplan.Plan{Schema: pathplan.Schema, Base: bodyplan.Plan{Schema: bodyplan.Schema, ID: "unit-max", Name: "UnitMax", ResultType: decision.TypeInt,
			Expressions: []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 7}, {Kind: "binary", Operation: "less_than", Left: 0, Right: 1}},
			Statements:  []bodyplan.Stmt{{Kind: "return", Expr: 0}, {Kind: "return", Expr: 1}, {Kind: "if", Expr: 2, Then: []int{0}, Else: []int{1}}}, Root: []int{2}},
			Decisions:      []pathplan.Choice{{ID: "comparison", Kind: pathplan.OperandOrder, Target: 2, Intent: "input below bound", Fallback: "layout_forward", Options: []pathplan.Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}}, {ID: "branches", Kind: pathplan.BranchLayout, Target: 2, Intent: "큰 값 반환", Fallback: "layout_forward", Options: []pathplan.Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}}},
			ConditionCases: []pathplan.ConditionCase{{ChoiceID: "comparison", Input: -9007199254740995, Expected: true}}}
		if i == 1 {
			p.Base.Statements[2].Then, p.Base.Statements[2].Else = []int{1}, []int{0}
		}
		doc := pathplan.Document{Schema: pathplan.DocumentSchema, Plan: p, TestCases: []pathplan.TestCase{{Input: -9007199254740995, Expected: 7}, {Input: 9007199254740993, Expected: 9007199254740993}}, MaxAttempts: 4}
		d.Records = append(d.Records, sourceRecord{ID: split, Split: split, Source: "unit fixture", SHA: hash([]byte("unit fixture")), Document: doc})
	}
	return d
}

func TestPreparationSeparatesTrainingAndOutputAblation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dir := t.TempDir()
	f, err := rowFile(dir, "candidates.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := rowFile(dir, "contexts.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	programs, rows, err := prepare(ctx, syntheticPair(), f, g)
	if err != nil || len(rows) != 10 || len(trainingRows(rows)) != 5 {
		t.Fatal(err, len(rows))
	}
	if len(modelSamples(rows, 0).([]conditiondecision.Sample)) != 5 || len(modelSamples(rows, 2).([]executiondecision.Sample)) != 5 {
		t.Fatal("held rows entered fit")
	}
	if rows[0].Acceptable != 4 || rows[5].Acceptable != 1 {
		t.Fatal("source orientation labels")
	}
	for _, row := range rows {
		for choice := range row.Features[0] {
			for cell := range 256 {
				if row.Features[0][choice][cell] != row.Features[1][choice][cell] || row.Features[0][choice][cell] != row.Features[2][choice][cell] {
					t.Fatal("ablation altered prefix")
				}
			}
			for cell := 256; cell < 320; cell++ {
				if row.Features[0][choice][cell] != 0 || row.Features[1][choice][cell] != 0 {
					t.Fatal("ablation tail nonzero")
				}
			}
		}
	}
	if rows[1].Features[2][0][256] != 0.125 {
		t.Fatal("output-only failure not supplied")
	}
	if programs[0].candidates[2].Cases[1].Actual != 9007199254740993 {
		t.Fatal("exact integer changed")
	}
	var models [3]fittedModel
	models[0].v2, _ = conditiondecision.NewForFeatures([conditiondecision.ParameterCount]float32{}, decision.ConditionBranchFeatureVersion)
	for i := 1; i < 3; i++ {
		models[i].v3, _ = executiondecision.New([executiondecision.ParameterCount]float32{})
	}
	report := report{Groups: map[string]group{}, SearchGroups: map[string]searchGroup{}}
	j, _ := rowFile(dir, "judgments.jsonl")
	defer j.Close()
	for i, m := range models {
		if _, err := rank(ctx, m, i, programs, rows, j, &report); err != nil {
			t.Fatal(err)
		}
	}
	s, _ := rowFile(dir, "searches.jsonl")
	defer s.Close()
	if err := search(ctx, models, programs, s, &report); err != nil || report.Judgments != 30 || report.Searches != 14 {
		t.Fatal(err, report.Judgments, report.Searches)
	}
	for _, g := range report.SearchGroups {
		if g.Complete != g.Programs {
			t.Fatal("finite unit search incomplete")
		}
	}
	if err := save(dir, "saved.json", report); err != nil {
		t.Fatal(err)
	}
	if err := save(dir, "saved.json", struct{}{}); err == nil {
		t.Fatal("saved output overwritten")
	}
	if _, err := os.Stat(dir + "/saved.json"); err != nil {
		t.Fatal(err)
	}
}

func TestFixedOutputOffWeightsIgnoreRuntimeTail(t *testing.T) {
	row := inputRecord{Split: "train", Masks: []uint16{0, 1}, Acceptable: 1}
	for i := range 3 {
		row.Features[i] = make([][320]float32, 1)
		row.Features[i][0][0] = 1
	}
	m, _, err := fitModel(context.Background(), []inputRecord{row}, 1, conditiondecision.FitOptions{Epochs: 2, LearningRate: 0.25, Seed: 17})
	if err != nil {
		t.Fatal(err)
	}
	weights := m.v3.Weights()
	for h := range 24 {
		for _, w := range weights[h*320+256 : (h+1)*320] {
			if w != 0 {
				t.Fatal("output connection retained")
			}
		}
	}
	var before, after conditiondecision.Prediction
	if err := m.predict(row.Features[1], row.Masks, &before); err != nil {
		t.Fatal(err)
	}
	for j := 256; j < 320; j++ {
		row.Features[1][0][j] = 0.125
	}
	if err := m.predict(row.Features[1], row.Masks, &after); err != nil || before != after {
		t.Fatal("output-off prediction changed", err)
	}
}
