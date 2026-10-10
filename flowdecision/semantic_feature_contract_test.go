package flowdecision

import (
	"context"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestSemanticFlowExplicitTrainingAndArtifactVersion(t *testing.T) {
	var inputs [1][FeatureDim]float32
	inputs[0][255], inputs[0][321] = 2, 0.125
	m, history, err := FitForFeatures(context.Background(), []Sample{{Inputs: inputs[:], Masks: []uint16{0, 1}, Acceptable: 2}}, FitOptions{Epochs: 2, LearningRate: 0.1, Seed: 17}, decision.SemanticFlowFeatureVersion)
	if err != nil || len(history) != 2 || m.FeatureVersion() != decision.SemanticFlowFeatureVersion {
		t.Fatal("explicit fit", err)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(raw)
	if err != nil || loaded.Fingerprint() != m.Fingerprint() || loaded.Weights() != m.Weights() {
		t.Fatal("artifact", err)
	}
	old, err := New(m.Weights())
	if err != nil || old.FeatureVersion() != decision.ExecutionFlowFeatureVersion || old.Fingerprint() == m.Fingerprint() {
		t.Fatal("v4 identity silently upgraded", err)
	}
}
