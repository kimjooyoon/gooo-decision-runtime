package main

import (
	"context"
	"testing"

	"github.com/kimjooyoon/meta-ontology-go/internal/bodycodegen"
)

func TestDeclaredScopeAndGoooTemplates(t *testing.T) {
	all := specifications()
	train := 0
	for _, s := range all {
		if s.Split == "future_train" {
			train++
		}
	}
	if len(all) != 162 || train != 16 {
		t.Fatal("scope differs", len(all), train)
	}
	for _, s := range append(all[:5:5], all[160:]...) {
		doc, err := bodycodegen.DecodeSourcePathDocument(context.Background(), s.ID+".gooo", []byte(source(s)), "Choose", nil)
		if err != nil {
			t.Fatal(s.ID, err)
		}
		p, err := doc.Prepare()
		if err != nil {
			t.Fatal(s.ID, err)
		}
		for _, choice := range doc.Plan.Decisions {
			view, err := p.SemanticBranchContext(choice.ID)
			if err != nil || view.Normalized != (s.Split != "control") {
				t.Fatal(s.ID, view, err)
			}
		}
	}
}
