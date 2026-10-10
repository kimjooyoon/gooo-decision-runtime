package contractdecision

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

type countedConditions struct {
	n, seen, fail int
	version       string
	nonfinite     bool
}

func (r *countedConditions) ConditionCount() int { return r.n }
func (r *countedConditions) ConditionFeatureVersion() string {
	if r.version != "" {
		return r.version
	}
	return decision.DeclaredConditionFeatureVersion
}
func (r *countedConditions) ConditionFeatures(i int) ([ConditionDim]float32, error) {
	r.seen++
	var row [ConditionDim]float32
	if i == r.fail {
		return row, errors.New("condition reader failed")
	}
	row[0], row[16] = 1.0/8, 1.0/8
	if i == r.n-1 {
		row[2] = 1.0 / 8
	}
	if r.nonfinite {
		row[0] = float32(math.NaN())
	}
	return row, nil
}

func TestRequirementConditionedReadsBothFullStreamsAndPreservesErrors(t *testing.T) {
	var weights [RequirementParameterCount]float32
	weights[requirementConditionStart+2] = 8
	weights[requirementHiddenStart+FeatureDim+PoolDim] = 1
	weights[requirementScoreStart+HiddenDim] = 1
	m, err := NewRequirementConditioned(weights, ExtremePooling)
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([][FeatureDim]float32, 16)
	outputs := &countedCases{n: 128, fail: -1}
	conditions := &countedConditions{n: 128, fail: -1}
	var w RequirementWorkspace
	var p Prediction
	if err := m.PredictInto(inputs, outputs, conditions, []uint16{0, 65535}, &w, &p); err != nil ||
		outputs.seen != 128 || conditions.seen != 128 || p.Selected != 65535 {
		t.Fatal("last condition or output rows lost", outputs.seen, conditions.seen, p, err)
	}
	beforeW, beforeP := w, p
	for _, reader := range []ConditionSource{nil, &countedConditions{n: -1}, &countedConditions{n: 129},
		&countedConditions{n: 1, version: "different"}, &countedConditions{n: 128, fail: 127},
		&countedConditions{n: 1, fail: -1, nonfinite: true}} {
		if err := m.PredictInto(inputs, rows{{}}, reader, []uint16{0, 1}, &w, &p); err == nil || w != beforeW || p != beforeP {
			t.Fatal("condition error changed destinations", err)
		}
	}
	if err := m.PredictInto(inputs, rows{{}}, conditionRows{}, []uint16{0, 0}, &w, &p); err == nil || w != beforeW || p != beforeP {
		t.Fatal("duplicate candidates changed destinations")
	}
	if err := (*RequirementModel)(nil).PredictInto(inputs, rows{{}}, conditionRows{}, []uint16{0}, &w, &p); err == nil {
		t.Fatal("nil model accepted")
	}
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			var local RequirementWorkspace
			var prediction Prediction
			if err := m.PredictInto(inputs, &countedCases{n: 128, fail: -1}, &countedConditions{n: 128, fail: -1},
				[]uint16{0, 65535}, &local, &prediction); err != nil || prediction != beforeP {
				t.Error("shared read-only model changed prediction", prediction, err)
			}
		})
	}
	group.Wait()
}

func TestRequirementConditionedLearnsOppositeBooleanGoals(t *testing.T) {
	inputs := make([][FeatureDim]float32, 1)
	inputs[0][0] = 1.0 / 8
	var output [CaseDim]float32
	if err := decision.DeclaredCaseFeaturesInto(-9007199254740995, 0, &output); err != nil {
		t.Fatal(err)
	}
	samples := make([]RequirementSample, 2)
	for i, expected := range []bool{true, false} {
		conditions := make(conditionRows, 1)
		if err := decision.DeclaredConditionFeaturesInto(decision.DeclaredConditionFeatureInput{
			Input: -9007199254740995, Expected: expected, ChoiceCount: 1, Count: 1}, &conditions[0]); err != nil {
			t.Fatal(err)
		}
		samples[i] = RequirementSample{Sample: Sample{Inputs: inputs, Cases: rows{output}, Masks: []uint16{0, 1}, Acceptable: 1 << i}, Conditions: conditions}
	}
	options := FitOptions{Epochs: 2000, LearningRate: .3, Seed: 17}
	m, history, err := FitRequirementConditioned(context.Background(), samples, options, MeanPooling)
	if err != nil || len(history) != options.Epochs || history[len(history)-1].Loss >= history[0].Loss {
		t.Fatal(history, err)
	}
	for i, sample := range samples {
		var w RequirementWorkspace
		var p Prediction
		if err := m.PredictInto(sample.Inputs, sample.Cases, sample.Conditions, sample.Masks, &w, &p); err != nil || p.Selected != uint16(i) {
			t.Fatal("condition-only distinction was not learned", i, p, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if model, _, err := FitRequirementConditioned(ctx, samples, options, MeanPooling); !errors.Is(err, context.Canceled) || model != nil {
		t.Fatal("cancelled fit returned a model", err)
	}
	broken := append([]RequirementSample(nil), samples...)
	broken[1].Conditions = nil
	if model, _, err := FitRequirementConditioned(context.Background(), broken, options, MeanPooling); err == nil || model != nil {
		t.Fatal("missing condition contract accepted")
	}
}
