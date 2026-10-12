package main

import (
	"context"
	"encoding/json"
	"github.com/kimjooyoon/meta-ontology-go/internal/bodycodegen"
	"os"
	"path/filepath"
	"testing"
)

// Parse/encode only: no model, candidate, fitting or native execution.
func TestOwnedStudySourceAndInputShapes(t *testing.T) {
	raw, err := os.ReadFile("../prefit/specifications.json")
	if err != nil {
		t.Fatal(err)
	}
	var specs []specification
	if err = json.Unmarshal(raw, &specs); err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1536 {
		t.Fatal(len(specs))
	}
	for _, s := range specs {
		if s.Family != 0 || s.Goal != 0 || s.Language != "ko" || s.Form != 1 || s.Parity != 0 {
			continue
		}
		source, err := os.ReadFile(filepath.Join("../prefit", s.Source))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := bodycodegen.DecodeSourcePathDocument(context.Background(), s.ID+".gooo", source, "Choose", nil)
		if err != nil {
			t.Fatal(s.ID, err)
		}
		p, err := doc.Prepare()
		if err != nil {
			t.Fatal(err)
		}
		input, err := p.InitialContractInput(doc.TestCases)
		if err != nil {
			t.Fatal(err)
		}
		if input.CaseCount() != 8 || input.ConditionCount() != 3 {
			t.Fatal(s.ID, "case count")
		}
		for _, choice := range doc.Plan.Decisions {
			var before, after [528]float32
			if err = input.SourceRelativeV3ProseNeutralFeaturesInto(choice.ID, &before); err != nil {
				t.Fatal(s.ID, err)
			}
			if err = input.SourceRelativeV4FeaturesInto(choice.ID, &after); err != nil {
				t.Fatal(s.ID, err)
			}
			for i := range after {
				if i < 256 || i >= 352 {
					if before[i] != after[i] {
						t.Fatal(s.ID, "unrelated cell", i)
					}
				}
			}
		}
	}
}

func TestOwnedStudyGroupsDoNotTreatPartialSiblingsAsComplete(t *testing.T) {
	rows := make([]observation, 16)
	for i := range rows {
		rows[i] = observation{Split: "evaluation", Contract: "one", First: measurement{Complete: true}, Checked: measurement{Complete: true}, Attempts: 1}
	}
	_, contracts := groupedOutcomes(rows)
	if contracts["evaluation"].CompleteAcrossSiblings != 1 {
		t.Fatal(contracts)
	}
	rows[7].First.Complete = false
	groups, contracts := groupedOutcomes(rows)
	if contracts["evaluation"].CompleteAcrossSiblings != 0 || groups["family/0"].FirstComplete != 15 {
		t.Fatal(groups, contracts)
	}
	_, contracts = groupedOutcomes(rows[:15])
	if contracts["evaluation"].CompleteAcrossSiblings != 0 {
		t.Fatal(contracts)
	}
}
