package conditiondecision

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestFeatureVersionBindsArtifactAndFingerprint(t *testing.T) {
	raw, err := os.ReadFile("../studies/condition-candidate-20261010/result/model.json")
	if err != nil {
		t.Fatal(err)
	}
	old, err := Decode(raw)
	if err != nil || old.FeatureVersion() != decision.ConditionChannelFeatureVersion || old.Fingerprint() != "456a3528de6abbcfc9ef63feb66264d59e31d7807b54e38f3a6ce08f0daf278a" {
		t.Fatal("published v1 identity changed", err)
	}
	legacy, err := New(old.Weights())
	if err != nil {
		t.Fatal(err)
	}
	legacyBytes, _ := legacy.Marshal()
	oldBytes, _ := old.Marshal()
	if !bytes.Equal(legacyBytes, oldBytes) {
		t.Fatal("default constructor artifact changed")
	}
	// This synthetic clone checks ABI binding; it is not a newly trained model.
	current, err := NewForFeatures(old.Weights(), decision.ConditionBranchFeatureVersion)
	if err != nil || current.Fingerprint() == old.Fingerprint() {
		t.Fatal("ABI absent from identity", err)
	}
	for _, m := range []*Model{old, current, {}} {
		encoded, err := m.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := Decode(encoded)
		if err != nil || loaded.FeatureVersion() != m.FeatureVersion() || loaded.Fingerprint() != m.Fingerprint() || loaded.Weights() != m.Weights() {
			t.Fatal("ABI round trip", err)
		}
		again, _ := loaded.Marshal()
		if !bytes.Equal(encoded, again) {
			t.Fatal("artifact changed")
		}
		unknown := bytes.ReplaceAll(encoded, []byte(m.FeatureVersion()), []byte("unknown-features"))
		if _, err := Decode(unknown); err == nil {
			t.Fatal("unknown features accepted")
		}
	}
	var absent *Model
	if absent.FeatureVersion() != "" {
		t.Fatal("nil version invented")
	}
	if _, err := NewForFeatures(old.Weights(), ""); err == nil {
		t.Fatal("empty version accepted")
	}
}

func TestFitForFeaturesPreservesExplicitVersionAndCancellation(t *testing.T) {
	opts := FitOptions{Epochs: 2, LearningRate: 0.1, Seed: 17}
	samples := []Sample{fixture()}
	m, history, err := FitForFeatures(context.Background(), samples, opts, decision.ConditionBranchFeatureVersion)
	if err != nil || len(history) != 2 || m.FeatureVersion() != decision.ConditionBranchFeatureVersion {
		t.Fatal("training version lost", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if m, _, err := FitForFeatures(ctx, samples, opts, decision.ConditionBranchFeatureVersion); m != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled training returned model", err)
	}
	if m, _, err := FitForFeatures(context.Background(), samples, opts, "future"); m != nil || err == nil {
		t.Fatal("unknown version trained")
	}
	legacy, _, err := Fit(context.Background(), samples, opts)
	if err != nil || legacy.FeatureVersion() != decision.ConditionChannelFeatureVersion || legacy.Weights() != m.Weights() {
		t.Fatal("version changed identical-array training", err)
	}
}
