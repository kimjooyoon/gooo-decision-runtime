package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/ordered-requirement-learning-20261010/record"
)

func rejected(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("altered observation accepted")
		}
	}()
	fn()
}

func TestOriginalResultsAndIndependentArithmetic(t *testing.T) {
	s, err := audit("../result")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, decode[Summary](load("../result", "audit.json"))) {
		t.Fatal("published audit differs from the full independent recount")
	}
	c := s.Comparisons["requirements->ordered"]
	if c.Improved != 13 || c.Regressed != 12 || c.Unchanged != 23 {
		t.Fatal(c)
	}
	for mode, want := range map[string]int{"deterministic": 12, "requirements": 12, "ordered": 13} {
		g := s.Groups[mode+"/all"]
		if g.FirstValid != want || g.Complete != 48 || g.FirstConditionTotal != 1144 {
			t.Fatal(mode, g)
		}
	}
	if s.Groups["ordered/train"].FirstValid != 2 || s.Groups["ordered/heldout_satisfiable"].FirstValid != 11 || s.Groups["ordered/unsupported"].ModelCalls != 0 {
		t.Fatal("lost learning limitations")
	}
	if s.InputCollisions["ordered"].ConflictingGroups != 0 || s.InputCollisions["requirements"].ConflictingGroups != 24 {
		t.Fatal("input distinction")
	}
	if s.DecisionDiagnostics["ordered/unsupported"].Declined != 8 || len(s.Interactions) != 48 {
		t.Fatal("input coverage")
	}
	x := int64(18014398509481990)
	v, condition := actual(r.Spec{Family: "distance", K: 53}, 2, x)
	if v != 18014398509481937 || condition {
		t.Fatal("exact int64 difference", v, condition)
	}

}

func TestTamperedCasesLabelsAndReceiptsAreRejected(t *testing.T) {
	sources := lines[r.Source](load("../result", "sources.jsonl.gz"))
	x := sources[0]
	for _, change := range []func(*r.Source){
		func(x *r.Source) { x.Candidates[0].Outputs[0].Actual++ },
		func(x *r.Source) { x.Candidates[0].Outputs[0].Actual++; x.Candidates[0].Outputs[0].Expected++ },
		func(x *r.Source) { x.CaseRows[0][12] += 1.0 / 2048 },
		func(x *r.Source) { x.ConditionRows[0][12] += 1.0 / 2048 },
		func(x *r.Source) { x.Inputs[0][0]++ },
		func(x *r.Source) { x.Acceptable ^= 1 },
		func(x *r.Source) { x.OrderedInputs[0][527] += 1.0 / 2048 },
		func(x *r.Source) {
			x.OrderedInputs[0][400] += 1.0 / 2048
			x.OrderedSHA[0] = r.OrderedFeatureSHA(x.OrderedInputs[0])
		},
		func(x *r.Source) { x.Document.Plan.ConditionCases[0].Expected = false },
		func(x *r.Source) { x.Spec.Split = "train-corrupted" },
	} {
		v := decode[r.Source](r.Encode(x))
		change(&v)
		rejected(t, func() { checkSource(v, x.Spec) })
	}
	searches := lines[r.Search](load("../result", "searches.jsonl.gz"))
	base := searches[2]
	for _, change := range []func(*r.Search){
		func(x *r.Search) { x.Ranking.Calls++ },
		func(x *r.Search) { x.Progress[1].NewAttempts[0].Results[0].Actual++ },
		func(x *r.Search) { x.Ranking.Proposed ^= 2 },
		func(x *r.Search) { x.Progress[1].PreviousSHA = "changed" },
	} {
		v := decode[r.Search](r.Encode(base))
		change(&v)
		rejected(t, func() { checkSearch(v, x, base.Ranking.ModelFingerprint) })
	}
	for _, i := range []int{40, 47} {
		v := decode[r.Source](r.Encode(sources[i]))
		v.Candidates[0].Conditions[127].Observation.Value = !v.Candidates[0].Conditions[127].Observation.Value
		rejected(t, func() { checkSource(v, sources[i].Spec) })
	}
}

func TestOriginalModelIdentityIsRequired(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"model-ordered.json", "model-requirements.json", "history-ordered.json", "history-requirements.json"} {
		if err := os.WriteFile(filepath.Join(root, name), load("../result", name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	report := decode[r.Report](load("../result", "report.json"))
	m := report.Models["ordered"]
	m.Parameters--
	report.Models["ordered"] = m
	rejected(t, func() { checkModels(root, report) })
	report = decode[r.Report](load("../result", "report.json"))
	raw := append(load(root, "model-ordered.json"), ' ')
	if err := os.WriteFile(filepath.Join(root, "model-ordered.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	rejected(t, func() { checkModels(root, report) })
}

func TestReadableExamplesAreOriginalRecords(t *testing.T) {
	sources := lines[r.Source](load("../result", "sources.jsonl.gz"))
	count := 0
	for _, x := range sources {
		path := filepath.Join("../examples", x.Spec.ID)
		if _, err := os.Stat(path + ".gooo"); os.IsNotExist(err) {
			continue
		}
		raw, err := os.ReadFile(path + ".gooo")
		if err != nil || string(raw) != x.Gooo {
			t.Fatal("source example", x.Spec.ID, err)
		}
		got, err := os.ReadFile(path + ".json")
		if err != nil || !reflect.DeepEqual(x.Document, decode[pathplan.Document](got)) {
			t.Fatal("document example", x.Spec.ID, err)
		}
		count++
	}
	if count != 4 {
		t.Fatal("example count", count)
	}
}
