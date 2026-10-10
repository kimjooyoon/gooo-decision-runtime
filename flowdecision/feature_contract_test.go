package flowdecision

import (
	"context"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/executiondecision"
)

func TestValueFlowShapeAndSeparateArtifact(t *testing.T) {
	if FeatureDim != 384 || ParameterCount != 9290 {
		t.Fatal("value flow shape changed")
	}
	m, err := New([ParameterCount]float32{})
	if err != nil || m.FeatureVersion() != decision.ExecutionFlowFeatureVersion {
		t.Fatal("wrong input ABI", err)
	}
	old, _ := executiondecision.New([executiondecision.ParameterCount]float32{})
	oldRaw, _ := old.Marshal()
	if _, err := Decode(oldRaw); err == nil {
		t.Fatal("v3 artifact accepted")
	}
	raw, _ := m.Marshal()
	if _, err := executiondecision.Decode(raw); err == nil {
		t.Fatal("v4 artifact accepted by v3 decoder")
	}
	if _, err := conditiondecision.Decode(raw); err == nil {
		t.Fatal("v4 artifact accepted by condition decoder")
	}
	if m.Fingerprint() == old.Fingerprint() {
		t.Fatal("model identities collided")
	}
	for _, version := range []string{decision.ExecutionFeatureVersion, decision.ConditionBranchFeatureVersion} {
		if _, err := NewForFeatures(m.Weights(), version); err == nil {
			t.Fatal("old representation accepted")
		}
	}
}

func TestFittingUsesStaticReturnChannel(t *testing.T) {
	a, b := Sample{Inputs: make([][FeatureDim]float32, 1), Masks: []uint16{0, 1}, Acceptable: 1}, Sample{Inputs: make([][FeatureDim]float32, 1), Masks: []uint16{0, 1}, Acceptable: 2}
	// Only the new cells differ. This tests numerical connectivity; source
	// generalization is measured by separate, fixed learning studies.
	a.Inputs[0][321], b.Inputs[0][337] = 1, 1
	m, _, err := Fit(context.Background(), []Sample{a, b}, FitOptions{Epochs: 160, LearningRate: 0.3, Seed: 17})
	if err != nil {
		t.Fatal(err)
	}
	for i, sample := range []Sample{a, b} {
		var w Workspace
		var prediction Prediction
		if err := m.PredictInto(sample.Inputs, sample.Masks, &w, &prediction); err != nil || prediction.Selected != uint16(i) {
			t.Fatal("static channel did not affect learned choice", prediction, err)
		}
	}
}
