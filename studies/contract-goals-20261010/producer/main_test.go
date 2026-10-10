package main

import (
	"context"
	"reflect"
	"testing"
	"time"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
	"github.com/kimjooyoon/meta-ontology-go/internal/bodycodegen"
)

func TestFixedScopeAndRepresentativeSourceLowering(t *testing.T) {
	specs := r.Specs()
	counts := map[string]int{}
	cases := 0
	for _, s := range specs {
		counts[s.Split]++
		cases += len(r.Cases(s))
	}
	want := map[string]int{"train": 24, "representation": 48, "constant": 72, "wording": 24, "unseen_family": 24, "rare_tail": 2, "contradiction": 2}
	if len(specs) != 196 || cases != 2048 || !reflect.DeepEqual(counts, want) {
		t.Fatal("fixed scope", counts, cases)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	seen := map[string]bool{}
	for i, s := range specs {
		key := s.Family + "/" + s.Form
		if seen[key] {
			continue
		}
		seen[key] = true
		d, err := bodycodegen.DecodeSourcePathDocument(ctx, s.ID+".gooo", []byte(r.Gooo(s)), "Choose", nil)
		if err != nil {
			t.Fatal(s.ID, err)
		}
		if !reflect.DeepEqual(d.TestCases, r.Cases(s)) {
			t.Fatal("declared cases changed")
		}
		p, err := d.Prepare()
		if err != nil {
			t.Fatal(s.ID, err)
		}
		view, err := p.InitialContractInput(d.TestCases)
		if err != nil {
			t.Fatal(err)
		}
		other := specs[i+1]
		d2, err := bodycodegen.DecodeSourcePathDocument(ctx, other.ID+".gooo", []byte(r.Gooo(other)), "Choose", nil)
		if err != nil {
			t.Fatal(err)
		}
		p2, err := d2.Prepare()
		if err != nil {
			t.Fatal(err)
		}
		view2, err := p2.InitialContractInput(d2.TestCases)
		if err != nil {
			t.Fatal(err)
		}
		for _, choice := range d.Plan.Decisions {
			var a, b [384]float32
			if view.RelationalSourceFeaturesInto(choice.ID, &a) != nil || view2.RelationalSourceFeaturesInto(choice.ID, &b) != nil || a != b {
				t.Fatal("goals changed initial source", s.ID)
			}
		}
	}
}
