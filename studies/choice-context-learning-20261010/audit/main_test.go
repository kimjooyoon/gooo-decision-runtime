package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/choice-context-learning-20261010/record"
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
	c := s.Comparisons["global->choice"]
	if c.Improved != 87 || c.Regressed != 63 || c.Unchanged != 238 || len(c.Regressions) != 63 {
		t.Fatal(c)
	}
	for mode, want := range map[string]int{"deterministic": 193, "global": 197, "choice": 221} {
		g := s.Groups[mode+"/all"]
		if g.FirstValid != want || g.Complete != 386 || g.FirstConditionPassed != 1164 {
			t.Fatal(mode, g)
		}
	}
	if s.Pairs["choice/all"].BothValid != 29 || s.Pairs["choice/all"].BothInvalid != 1 || s.Groups["choice/unseen_family"].FirstValid != 31 {
		t.Fatal("lost limitations", s.Pairs)
	}
	x := int64(18014398509481990)
	v, condition := actual(r.Spec{Family: "scale", K: 21}, 2, x)
	if v != 378302368699121790 || condition {
		t.Fatal("exact multiplication above 2^53", v, condition)
	}
}

func TestTamperedCasesLabelsAndReceiptsAreRejected(t *testing.T) {
	sources := lines[r.Source](load("../result", "sources.jsonl.gz"))
	x := sources[0]
	for _, change := range []func(*r.Source){
		func(x *r.Source) { x.Candidates[0].Outputs[0].Actual++ },
		func(x *r.Source) { x.Candidates[0].Outputs[0].Actual++; x.Candidates[0].Outputs[0].Expected++ },
		func(x *r.Source) { x.CaseRows[0][12] += 1.0 / 2048 },
		func(x *r.Source) { x.Inputs[0][0]++ },
		func(x *r.Source) { x.Acceptable ^= 1 },
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
	for _, i := range []int{384, 386} {
		v := decode[r.Source](r.Encode(sources[i]))
		v.Candidates[0].Outputs[127].Actual++
		rejected(t, func() { checkSource(v, sources[i].Spec) })
	}
}

func TestOriginalModelIdentityIsRequired(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"model-global.json", "model-choice.json", "history-global.json", "history-choice.json"} {
		if err := os.WriteFile(filepath.Join(root, name), load("../result", name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	report := decode[r.Report](load("../result", "report.json"))
	m := report.Models["choice"]
	m.Parameters--
	report.Models["choice"] = m
	rejected(t, func() { checkModels(root, report) })
	report = decode[r.Report](load("../result", "report.json"))
	raw := append(load(root, "model-choice.json"), ' ')
	if err := os.WriteFile(filepath.Join(root, "model-choice.json"), raw, 0600); err != nil {
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
	if count != 3 {
		t.Fatal("example count", count)
	}
}
