package contractdecision

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
)

func TestOrderedRequirementFullStreamsErrorsAndConcurrentPredictions(t *testing.T) {
	var weights [OrderedRequirementParameterCount]float32
	weights[orderedRequirementConditionStart+2] = 8
	weights[orderedRequirementHiddenStart+OrderedFeatureDim+PoolDim] = 1
	weights[orderedRequirementScoreStart+HiddenDim] = 1
	m, _ := NewOrderedRequirementConditioned(weights, ExtremePooling)
	inputs := make([][OrderedFeatureDim]float32, 16)
	outputs, conditions := &countedCases{n: 128, fail: -1}, &countedConditions{n: 128, fail: -1}
	var w OrderedRequirementWorkspace
	var p Prediction
	if err := m.PredictInto(inputs, outputs, conditions, []uint16{0, 65535}, &w, &p); err != nil ||
		outputs.seen != 128 || conditions.seen != 128 || p.Selected != 65535 {
		t.Fatal("full stream consumption", outputs.seen, conditions.seen, p, err)
	}
	beforeW, beforeP := w, p
	for _, reader := range []ConditionSource{nil, &countedConditions{n: 129}, &countedConditions{n: 1, version: "wrong"},
		&countedConditions{n: 128, fail: 127}, &countedConditions{n: 1, fail: -1, nonfinite: true}} {
		if m.PredictInto(inputs, rows{{}}, reader, []uint16{0, 1}, &w, &p) == nil || w != beforeW || p != beforeP {
			t.Fatal("condition error changed output")
		}
	}
	inputs[15][OrderedFeatureDim-1] = float32(math.Inf(1))
	if m.PredictInto(inputs, rows{{}}, conditionRows{}, []uint16{0, 1}, &w, &p) == nil || w != beforeW || p != beforeP {
		t.Fatal("nonfinite final source cell accepted")
	}
	inputs[15][OrderedFeatureDim-1] = 0
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			var local OrderedRequirementWorkspace
			var prediction Prediction
			if err := m.PredictInto(inputs, &countedCases{n: 128, fail: -1}, &countedConditions{n: 128, fail: -1}, []uint16{0, 65535}, &local, &prediction); err != nil || prediction != beforeP {
				t.Error("concurrent model read", err)
			}
		})
	}
	group.Wait()
}

func TestOrderedRequirementAuditConsumesSuffixAndFitChecksContext(t *testing.T) {
	samples := []OrderedRequirementSample{
		{Inputs: make([][OrderedFeatureDim]float32, 1), Cases: rows{{}}, Conditions: conditionRows{}, Masks: []uint16{0, 1}, Acceptable: 1},
		{Inputs: make([][OrderedFeatureDim]float32, 1), Cases: rows{{}}, Conditions: conditionRows{}, Masks: []uint16{0, 1}, Acceptable: 2},
	}
	first, err := AuditOrderedRequirements(context.Background(), samples)
	if err != nil || first.BestFirstPasses != 1 || first.UnavoidableMisses != 1 || first.Schema != OrderedRequirementInputAuditSchema {
		t.Fatal(first, err)
	}
	samples[1].Inputs[0][OrderedFeatureDim-1] = 1.0 / 8
	second, err := AuditOrderedRequirements(context.Background(), samples)
	if err != nil || second.BestFirstPasses != 2 || second.UnavoidableMisses != 0 || len(second.Groups) != 2 {
		t.Fatal("final ordered source cell ignored", second, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	options := FitOptions{Epochs: 1, LearningRate: .3, Seed: 17}
	if model, _, err := FitOrderedRequirementConditioned(ctx, samples, options, MeanPooling); !errors.Is(err, context.Canceled) || model != nil {
		t.Fatal("cancelled ordered fit", err)
	}
	if _, err := AuditOrderedRequirements(ctx, samples); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled audit", err)
	}
	samples[1].Conditions = nil
	if model, _, err := FitOrderedRequirementConditioned(context.Background(), samples, options, MeanPooling); err == nil || model != nil {
		t.Fatal("unbound condition input accepted")
	}
}
