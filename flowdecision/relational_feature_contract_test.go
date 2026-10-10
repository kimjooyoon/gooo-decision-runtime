package flowdecision

import (
	"context"
	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"testing"
)

func TestRelationalFlowFreshFitAndArtifactIdentity(t *testing.T) {
	var inputs [1][FeatureDim]float32
	inputs[0][255], inputs[0][239] = 3, 1
	m, history, err := FitForFeatures(context.Background(), []Sample{{Inputs: inputs[:], Masks: []uint16{0, 1}, Acceptable: 2}}, FitOptions{Epochs: 2, LearningRate: .1, Seed: 17}, decision.RelationalFlowFeatureVersion)
	if err != nil || len(history) != 2 || m.FeatureVersion() != decision.RelationalFlowFeatureVersion {
		t.Fatal(err)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(raw)
	if err != nil || loaded.Fingerprint() != m.Fingerprint() || loaded.Weights() != m.Weights() {
		t.Fatal("artifact identity", err)
	}
	old, err := NewForFeatures(m.Weights(), decision.SemanticFlowFeatureVersion)
	if err != nil || old.Fingerprint() == m.Fingerprint() {
		t.Fatal("v5 identity relabelled", err)
	}
}
