package main

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestSourceDatasetPreparationKeepsFiniteLabelsAndSplits(t *testing.T) {
	raw, err := os.ReadFile("../../studies/condition-candidate-20261010/dataset.json")
	if err != nil {
		t.Fatal(err)
	}
	var records []sourceRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		t.Fatal(err)
	}
	rows, err := prepare(context.Background(), records)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, record := range records {
		if seen[record.ID] {
			t.Fatal("duplicate program", record.ID)
		}
		seen[record.ID] = true
		counts[record.Split]++
	}
	if !reflect.DeepEqual(counts, map[string]int{"train": 8, "wording": 4, "structure": 8, "both": 4}) || len(rows) != 120 {
		t.Fatal(counts, len(rows))
	}
	for _, r := range rows {
		if len(r.Sample.Inputs) != 2 || len(r.Sample.Masks) != 4 || r.Sample.Acceptable == 0 {
			t.Fatal("invalid labeled program", r.Record.ID)
		}
		for i, outcome := range r.Outcomes {
			want := outcome.OutputPassed == outcome.OutputTotal && outcome.ConditionPassed == outcome.ConditionTotal
			if (r.Sample.Acceptable>>i&1 != 0) != want {
				t.Fatal("label disagrees with complete source cases", r.Record.ID, i)
			}
			if r.Context == "initial" {
				got, source, sha, err := replay(context.Background(), r, uint16(i))
				if err != nil || got != outcome || source == "" || len(sha) != 64 {
					t.Fatal("selected program replay differs", r.Record.ID, i, err)
				}
			}
		}
	}
	// Opposite source condition contracts preserve final-output candidates while
	// changing the acceptable complete mask set. No label is manually assigned.
	for family := range 4 {
		for wording := range 3 {
			first, second := rows[(family*6+wording)*5], rows[(family*6+3+wording)*5]
			if first.Sample.Acceptable == second.Sample.Acceptable {
				t.Fatal("paired conditions did not distinguish masks")
			}
			for i := range first.Outcomes {
				if first.Outcomes[i].OutputPassed != second.Outcomes[i].OutputPassed {
					t.Fatal("paired output contract changed")
				}
			}
		}
	}
	modified := append([]sourceRecord(nil), records...)
	modified[0].SHA = "changed"
	if _, err := prepare(context.Background(), modified); err == nil {
		t.Fatal("changed source accepted")
	}
}
