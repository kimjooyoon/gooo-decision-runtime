package contractdecision

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
)

func TestInteractionRequirementRejectsIntermediateOverflowAndCancelledFit(t *testing.T) {
	for _, index := range []int{0, interactionRequirementContextStart} {
		var weights [InteractionRequirementParameterCount]float32
		weights[index] = math.MaxFloat32
		m, err := NewInteractionRequirementConditioned(weights, MeanPooling)
		if err != nil {
			t.Fatal(err)
		}
		inputs := make([][OrderedFeatureDim]float32, 1)
		inputs[0][0] = 1
		outputs := rows{{1}}
		var workspace InteractionRequirementWorkspace
		prediction := Prediction{Selected: 17}
		before := workspace
		if m.PredictInto(inputs, outputs, conditionRows{}, []uint16{0, 1}, &workspace, &prediction) == nil || workspace != before || prediction.Selected != 17 {
			t.Fatal("tanh hid nonfinite intermediate arithmetic or changed destinations", index)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sample := InteractionRequirementSample{Inputs: make([][OrderedFeatureDim]float32, 1), Cases: rows{{}},
		Conditions: conditionRows{}, Masks: []uint16{0, 1}, Acceptable: 1}
	model, _, err := FitInteractionRequirementConditioned(ctx, []InteractionRequirementSample{sample}, FitOptions{Epochs: 1, LearningRate: .3}, MeanPooling)
	if !errors.Is(err, context.Canceled) || model != nil {
		t.Fatal("cancelled fit proceeded", err)
	}
}

func TestInteractionRequirementFullStreamsErrorsAndConcurrentPredictions(t *testing.T) {
	var weights [InteractionRequirementParameterCount]float32
	weights[interactionRequirementConditionStart+2] = 8
	weights[interactionRequirementHiddenStart+OrderedFeatureDim+2*PoolDim] = 1
	weights[interactionRequirementScoreStart+HiddenDim] = 1
	m, _ := NewInteractionRequirementConditioned(weights, ExtremePooling)
	inputs := make([][OrderedFeatureDim]float32, 16)
	outputs, conditions := &countedCases{n: 128, fail: -1}, &countedConditions{n: 128, fail: -1}
	var w InteractionRequirementWorkspace
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
			var local InteractionRequirementWorkspace
			var prediction Prediction
			if err := m.PredictInto(inputs, &countedCases{n: 128, fail: -1}, &countedConditions{n: 128, fail: -1}, []uint16{0, 65535}, &local, &prediction); err != nil || prediction != beforeP {
				t.Error("concurrent model read", err)
			}
		})
	}
	group.Wait()
}
