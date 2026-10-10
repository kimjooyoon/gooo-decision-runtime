package main

import (
	"context"
	"reflect"
	"testing"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/ordered-requirement-learning-20261010/record"
	"github.com/kimjooyoon/meta-ontology-go/internal/bodycodegen"
)

// Source lowering and input inspection happen before fitting. This preflight
// does not execute candidate bodies, make predictions or train a model.
func TestFrozenSourceInputContracts(t *testing.T) {
	rows, specs := frozenSources("../corpus.jsonl.gz"), r.Specs()
	counts := map[string]int{}
	seen := map[string]bool{}
	outputs, conditions := 0, 0
	var previous [][384]float32
	var priorConditions [][32]float32
	for i, row := range rows {
		s := row.Spec
		if s != specs[i] || row.Gooo != r.Gooo(s) || seen[s.ID] {
			t.Fatal("frozen identity", i)
		}
		seen[s.ID] = true
		counts[s.Split]++
		d, err := bodycodegen.DecodeSourcePathDocument(context.Background(), s.ID+".gooo", []byte(row.Gooo), "Choose", nil)
		if err != nil {
			t.Fatal(s.ID, err)
		}
		if !reflect.DeepEqual(d.TestCases, r.Cases(s)) || !reflect.DeepEqual(d.Plan.ConditionCases, r.Conditions(s)) {
			t.Fatal("authored exact goals", s.ID)
		}
		outputs += len(d.TestCases)
		conditions += len(d.Plan.ConditionCases)
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
			var ordered [528]float32
			err := input.OrderedSourceFeaturesInto(choice.ID, &ordered)
			if (err != nil) != (s.Form == "rare") {
				t.Fatal("ordered availability", s.ID, err)
			}
			if err == nil && !reflect.DeepEqual(ordered[:384], cells[:]) {
				t.Fatal("legacy prefix", s.ID)
			}
		}
		var conditionRows [][32]float32
		for j := range input.ConditionCount() {
			cells, err := input.ConditionFeatures(j)
			if err != nil {
				t.Fatal(err)
			}
			conditionRows = append(conditionRows, cells)
		}
		if len(features) != 2 {
			t.Fatal("two choices", s.ID)
		}
		comparison := d.Plan.Decisions[s.Order]
		if comparison.ID != "comparison" || comparison.Options[0].Label != "layout_forward" || comparison.Options[1].Label != "layout_reverse" {
			t.Fatal("choice order", s.ID)
		}
		if i%2 == 1 {
			if !reflect.DeepEqual(previous, features) || !reflect.DeepEqual(r.Cases(specs[i-1]), r.Cases(s)) {
				t.Fatal("condition-only pair changed original channels", s.ID)
			}
			if reflect.DeepEqual(priorConditions, conditionRows) != (s.Form == "empty") {
				t.Fatal("condition channel distinction", s.ID)
			}
		}
		previous, priorConditions = features, conditionRows
	}
	want := map[string]int{"train": 8, "heldout_direct": 8, "heldout_alias": 8, "heldout_assignment": 8, "heldout_rebinding": 8, "unsupported": 8}
	if outputs != 384 || conditions != 1144 || !reflect.DeepEqual(counts, want) {
		t.Fatal("fixed scope", counts, outputs, conditions)
	}
}
