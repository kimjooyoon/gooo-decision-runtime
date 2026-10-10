package executiondecision

import (
	"context"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
)

func TestExecutionShapeAndDecoderDoNotReuseConditionWeights(t *testing.T) {
	if FeatureDim != 320 || ParameterCount != 7754 {
		t.Fatal("execution shape changed")
	}
	m, err := New([ParameterCount]float32{})
	if err != nil {
		t.Fatal(err)
	}
	if m.FeatureVersion() != decision.ExecutionFeatureVersion {
		t.Fatal("wrong input ABI")
	}
	old, _ := conditiondecision.New([conditiondecision.ParameterCount]float32{})
	oldRaw, _ := old.Marshal()
	if _, err := Decode(oldRaw); err == nil {
		t.Fatal("condition artifact accepted")
	}
	raw, _ := m.Marshal()
	if _, err := conditiondecision.Decode(raw); err == nil {
		t.Fatal("execution artifact accepted by legacy decoder")
	}
	if m.Fingerprint() == old.Fingerprint() {
		t.Fatal("model identities collided")
	}
	if _, err := NewForFeatures(m.Weights(), decision.ConditionBranchFeatureVersion); err == nil {
		t.Fatal("old representation accepted")
	}
}

func TestFittingUsesAppendedOutputChannel(t *testing.T) {
	a, b := Sample{Inputs: make([][FeatureDim]float32, 1), Masks: []uint16{0, 1}, Acceptable: 1}, Sample{Inputs: make([][FeatureDim]float32, 1), Masks: []uint16{0, 1}, Acceptable: 2}
	// Inputs differ only above the legacy boundary. This is a synthetic unit
	// test of gradient/inference connectivity, not a source-generalization study.
	a.Inputs[0][270], b.Inputs[0][286] = 1, 1
	m, _, err := Fit(context.Background(), []Sample{a, b}, FitOptions{Epochs: 160, LearningRate: 0.3, Seed: 17})
	if err != nil {
		t.Fatal(err)
	}
	for i, sample := range []Sample{a, b} {
		var w Workspace
		var prediction Prediction
		if err := m.PredictInto(sample.Inputs, sample.Masks, &w, &prediction); err != nil || prediction.Selected != uint16(i) {
			t.Fatal("new channel did not affect learned choice", prediction, err)
		}
	}
}
