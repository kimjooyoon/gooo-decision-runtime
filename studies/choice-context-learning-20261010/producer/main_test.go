package main

import (
	"context"
	"reflect"
	"testing"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/choice-context-learning-20261010/record"
	"github.com/kimjooyoon/meta-ontology-go/internal/bodycodegen"
)

// Preflight lowers every frozen source without fitting, predicting or executing
// a candidate. Corpus errors must not become post-fit exclusions.
func TestFrozenCorpusSourceLowering(t *testing.T) {
	rows := frozenSources("../corpus.jsonl.gz")
	specs := r.Specs()
	counts := map[string]int{}
	seen := map[string]bool{}
	cases := 0
	var previous [][384]float32
	for i, row := range rows {
		s := row.Spec
		if s != specs[i] || row.Gooo != r.Gooo(s) || seen[s.ID] {
			t.Fatal("frozen identity", i)
		}
		seen[s.ID] = true
		counts[s.Split]++
		cases += len(r.Cases(s))
		d, err := bodycodegen.DecodeSourcePathDocument(context.Background(), s.ID+".gooo", []byte(row.Gooo), "Choose", nil)
		if err != nil {
			t.Fatal(s.ID, err)
		}
		if !reflect.DeepEqual(d.TestCases, r.Cases(s)) {
			t.Fatal("exact declared int64 cases", s.ID)
		}
		p, err := d.Prepare()
		if err != nil {
			t.Fatal(s.ID, err)
		}
		input, err := p.InitialContractInput(d.TestCases)
		if err != nil {
			t.Fatal(s.ID, err)
		}
		var features [][384]float32
		for _, choice := range d.Plan.Decisions {
			var cells [384]float32
			if err := input.RelationalSourceFeaturesInto(choice.ID, &cells); err != nil {
				t.Fatal(err)
			}
			features = append(features, cells)
		}
		if len(features) != 2 {
			t.Fatal("two choices", s.ID)
		}
		if i%2 == 1 && !reflect.DeepEqual(previous, features) {
			t.Fatal("paired source features differ", s.ID)
		}
		previous = features
	}
	want := map[string]int{"train": 32, "representation": 96, "constant": 128, "wording": 64, "unseen_family": 64, "rare_tail": 2, "contradiction": 2}
	if cases != 3584 || !reflect.DeepEqual(counts, want) {
		t.Fatal("fixed scope", counts, cases)
	}
}
