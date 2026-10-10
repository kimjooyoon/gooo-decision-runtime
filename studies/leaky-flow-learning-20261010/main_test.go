package main

import "testing"

func TestOnlyDirectTrainingSamplesAreCopied(t *testing.T) {
	var v6 [384]float32
	v6[255] = 3
	rows := []prepared{
		{Split: "future_train", Acceptable: 4, V6: [][384]float32{v6}},
		{Split: "representation_transfer"},
		{Split: "constant_transfer"},
		{Split: "control"},
	}
	a := training(rows)
	if len(a) != 1 || a[0].Inputs[0] != v6 || a[0].Acceptable != 4 {
		t.Fatal("training boundary")
	}
	a[0].Inputs[0][0], a[0].Masks[0] = 99, 3
	if rows[0].V6[0] != v6 {
		t.Fatal("training aliases source")
	}
}

func TestFrozenInputsMatchRuntimeWithoutModelCalls(t *testing.T) {
	rows, _ := load("../semantic-flow-normalization-20261010/result")
	baseline("../relational-flow-learning-20261010/result")
	if len(training(rows)) != 16 {
		t.Fatal("fixed split")
	}
	direct := map[string][][384]float32{}
	for _, row := range rows {
		if row.Form == "direct" {
			direct[row.Group] = row.V6
		}
	}
	pairs := 0
	for _, row := range rows {
		input, err := row.plan.InitialExecutionInput(row.Document.TestCases)
		if err != nil {
			t.Fatal(err)
		}
		for i, c := range row.Document.Plan.Decisions {
			var actual [384]float32
			if err := input.ExecutionRelationalFlowFeaturesInto(c.ID, &actual); err != nil || actual != row.V6[i] {
				t.Fatal("frozen input/runtime mismatch", row.ID, err)
			}
			for j, v := range actual {
				if (j < 236 || j >= 256) && v != row.V5[i][j] {
					t.Fatal("preserved channels", row.ID, j)
				}
				if ((j >= 192 && j < 236) || (j >= 256 && j < 320)) && v != 0 {
					t.Fatal("future observation", row.ID, j)
				}
			}
			if row.Split == "control" && actual != row.V5[i] {
				t.Fatal("control changed")
			}
			if row.Split != "control" && actual[255] != 3 {
				t.Fatal("normalized marker")
			}
			if row.Form != "direct" && row.Split != "control" && actual != direct[row.Group][i] {
				t.Fatal("equivalent inputs differ")
			}
		}
		if row.Form != "direct" && row.Split != "control" {
			pairs++
		}
	}
	if pairs != 128 {
		t.Fatal("paired representations", pairs)
	}
}
