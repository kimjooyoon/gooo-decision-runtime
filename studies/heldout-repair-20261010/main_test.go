package main

import "testing"

func TestNextUsesOnlyScoresAndKnownFailedMask(t *testing.T) {
	var scores [64]float64
	for rejected := range uint16(4) {
		want := uint16(0)
		if rejected == 0 {
			want = 1
		}
		if got := next(scores, rejected); got != want {
			t.Fatal("ascending ties", got, want)
		}
	}
	scores[1], scores[3] = 10, 9
	if next(scores, 1) != 3 || next(scores, 3) != 1 {
		t.Fatal("remaining score order")
	}
}

func TestAllHeldoutSourcesAndSavedInputsWithoutModelCalls(t *testing.T) {
	rows, _ := loadSources("../semantic-flow-normalization-20261010/result")
	old := loadModel("../leaky-flow-learning-20261010/result", "initial_trained", "model-leaky_v6.json", "d5f9c4ffb204852f4e1b3b1b1c491c7d1fb0753d0df2c1e0f0ffead80d9ac47e", "4ad76a785a122b478128b88edce4a056b90e9f867c7e7d631d5a3cf05de61acf")
	newModel := loadModel("../feedback-flow-learning-20261010/result", "observation_trained", "model-feedback_v6.json", "b81317ce44943549c58237eb9b87e797eca56c181a24086780f7f08a13d4ad59", "4aba2c54b83efd939fb3b2054ab03b87ed7bc6a44a640c743fbc2242a66242c8")
	starts := 0
	for _, s := range rows {
		if s.Split == "future_train" {
			t.Fatal("training source included")
		}
		p, err := s.Document.Prepare()
		if err != nil {
			t.Fatal(err)
		}
		input, err := p.InitialExecutionInput(s.Document.TestCases)
		if err != nil {
			t.Fatal(err)
		}
		features := make([][384]float32, 2)
		for i, c := range s.Document.Plan.Decisions {
			if err := input.ExecutionRelationalFlowFeaturesInto(c.ID, &features[i]); err != nil {
				t.Fatal(err)
			}
		}
		sha := featureSHA(features, 384)
		if old.Initial[s.ID].InputSHA != sha || newModel.Initial[s.ID].InputSHA != sha {
			t.Fatal("saved input mismatch", s.ID)
		}
		for mask := range uint16(4) {
			if s.Acceptable>>mask&1 == 0 {
				starts++
			}
		}
	}
	if starts != 438 {
		t.Fatal("fixed observation count", starts)
	}
}
