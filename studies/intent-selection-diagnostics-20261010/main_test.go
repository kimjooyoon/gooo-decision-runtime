package main

import "testing"

func TestFrozenDirectPairsDifferOnlyInIntent(t *testing.T) {
	rows := sources("..")
	for _, spec := range specs {
		old := oldPredictions("..", spec)
		byID := map[string]record{}
		for _, s := range rows {
			byID[s.ID] = record{ID: s.ID, Model: spec.Name, Acceptable: s.Acceptable, Inputs: input(s, spec.Name)}
		}
		for _, s := range rows {
			if len(s.ID) < 4 || s.ID[:4] != "max-" {
				continue
			}
			p := compare(byID[s.ID], byID["min-"+s.ID[4:]])
			if !p.DifferentInputs || !p.OnlyBranchIntentDiffers || !p.DisjointAcceptableSets || old[s.ID].Count != 4 {
				t.Fatal(p)
			}
		}
	}
}
