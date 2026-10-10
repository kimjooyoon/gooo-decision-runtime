package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func pair(t *testing.T) dataset {
	t.Helper()
	f, err := os.Open("../../studies/branch-feature-discrimination-20261010/result/report.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var saved struct {
		Rows []struct {
			Source   string            `json:"gooo_source"`
			SHA      string            `json:"source_sha256"`
			Document pathplan.Document `json:"document"`
		}
	}
	if err := json.NewDecoder(z).Decode(&saved); err != nil || len(saved.Rows) != 2 {
		t.Fatal(err)
	}
	var d dataset
	for i, r := range saved.Rows {
		split, id := "train", "forward"
		if i == 1 {
			split, id = "wording", "reverse"
		}
		d.Records = append(d.Records, sourceRecord{id, split, r.Source, r.SHA, r.Document})
	}
	return d
}

func TestPairedPreparationKeepsTrainingSeparatedAndExactCandidates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dir := t.TempDir()
	f, err := rowFile(dir, "candidates.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	inputs, err := rowFile(dir, "contexts.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer inputs.Close()
	p, rows, err := prepare(ctx, pair(t), f, inputs)
	if err != nil || len(p) != 2 || len(rows) != 6 {
		t.Fatal(err, len(rows))
	}
	for v := range 2 {
		if len(samples(rows, v)) != 3 {
			t.Fatal("evaluation rows entered fit")
		}
	}
	if rows[0].Acceptable != 1 || rows[3].Acceptable != 2 || conflicts(rows, 0) != 1 || conflicts(rows, 1) != 0 {
		t.Fatal("candidate or feature labels differ")
	}
	if rows[0].FeatureSHA[0] != rows[3].FeatureSHA[0] || rows[0].FeatureSHA[1] == rows[3].FeatureSHA[1] {
		t.Fatal("paired input representation")
	}
	if p[1].candidates[1].Cases[5].Input != 9007199254740993 || !p[1].candidates[1].Accepted {
		t.Fatal("exact integer lost")
	}
	var models [2]*conditiondecision.Model
	for v, version := range versions {
		models[v], err = conditiondecision.NewForFeatures([conditiondecision.ParameterCount]float32{}, version)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := report{Groups: map[string]group{}, SearchGroups: map[string]searchGroup{}}
	j, err := rowFile(dir, "judgments.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if _, err := rank(ctx, models[0], 0, p, rows, j, &r); err != nil || r.Judgments != 6 {
		t.Fatal("immediate candidate replay", err)
	}
	searches, err := rowFile(dir, "searches.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer searches.Close()
	if err := search(ctx, models, p, searches, &r); err != nil || r.Searches != 10 {
		t.Fatal(err, r.Searches)
	}
	for _, group := range r.SearchGroups {
		if group.Complete != group.Programs {
			t.Fatal("finite search did not complete")
		}
	}
	if err := save(dir, "completed.json", r); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "completed.json"))
	if err := save(dir, "completed.json", struct{}{}); err == nil {
		t.Fatal("existing observation overwritten")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "completed.json"))
	if string(before) != string(after) {
		t.Fatal("original observation changed")
	}
}

func TestDatasetIdentityRejectsChangedProtocol(t *testing.T) {
	for _, raw := range [][]byte{[]byte(`{}`), []byte(`{"schema":"a","schema":"b"}`), make([]byte, (8<<20)+1)} {
		if _, err := validateDataset(raw, "expected"); err == nil {
			t.Fatal("invalid dataset accepted")
		}
	}
}
