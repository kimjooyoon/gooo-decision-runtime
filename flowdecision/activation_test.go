package flowdecision

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestLeakyActivationKeepsNegativePathsAndBindsArtifact(t *testing.T) {
	var weights [ParameterCount]float32
	weights[0], weights[FeatureDim*HiddenDim] = 1, -2
	weights[w2Start], weights[w2Start+HiddenDim] = .5, -.5
	m, err := NewForActivation(weights, decision.RelationalFlowFeatureVersion, LeakyReLUActivation)
	if err != nil {
		t.Fatal(err)
	}
	old, err := NewForFeatures(weights, decision.RelationalFlowFeatureVersion)
	if err != nil {
		t.Fatal(err)
	}
	var input [1][FeatureDim]float32
	input[0][0] = 1
	var w Workspace
	var p Prediction
	var e Explanation
	if err := m.ExplainInto(input[:], []uint16{0, 1}, &w, &p, &e); err != nil || e.Hidden[0][0] != -negativeSlope || p.Selected != 1 {
		t.Fatal(e, p, err)
	}
	if old.Activation() != ReLUActivation || m.Activation() != LeakyReLUActivation || old.Fingerprint() == m.Fingerprint() {
		t.Fatal("activation identity")
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(raw)
	if err != nil || loaded.Activation() != LeakyReLUActivation || loaded.Fingerprint() != m.Fingerprint() || loaded.Weights() != m.Weights() {
		t.Fatal("artifact round trip", err)
	}
	for _, bad := range []string{
		strings.Replace(string(raw), ActivationSchema, Schema, 1),
		strings.Replace(string(raw), LeakyReLUActivation, ReLUActivation, 1),
		strings.Replace(string(raw), LeakyReLUActivation, "leaky_relu_0.1_v1", 1),
		strings.Replace(string(raw), `"activation":"`+LeakyReLUActivation+`"`, `"activation":null`, 1),
		strings.Replace(string(raw), `,"activation":"`+LeakyReLUActivation+`"`, "", 1),
	} {
		if _, err := Decode([]byte(bad)); err == nil {
			t.Fatal("mismatched activation accepted")
		}
	}
	legacy, _ := old.Marshal()
	for _, value := range []string{`null`, `""`, `"relu_v1"`} {
		bad := strings.Replace(string(legacy), `"schema":`, `"activation":`+value+`,"schema":`, 1)
		if _, err := Decode([]byte(bad)); err == nil {
			t.Fatal("legacy activation field accepted")
		}
	}
}

func TestPublishedReLUBytesAndFingerprintsStayExact(t *testing.T) {
	for _, s := range []struct{ Path, SHA, Fingerprint string }{
		{"../studies/semantic-flow-learning-20261010/result/model-v5.json", "8127de6776d13b9c06710d1da762bb254401c1d8ff7e26ee6bb3c2d50d5cb8db", "eb53c0fdef003f151aed582c56ce285947f70361b25199dba2eb882f0d14a16a"},
		{"../studies/relational-flow-learning-20261010/result/model-v6.json", "3ba2c75cd7a3dd398e445bdd31abaaa3b3337e6261beac9e88faba4ea12c9c3b", "652bd858d4395c04c73100ed9726843a2b79fe84aaac770d4f8908fe752f8159"},
	} {
		raw, err := os.ReadFile(s.Path)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != s.SHA {
			t.Fatal("immutable artifact")
		}
		m, err := Decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		out, err := m.Marshal()
		if err != nil || !bytes.Equal(raw, out) || m.Fingerprint() != s.Fingerprint || m.Activation() != ReLUActivation {
			t.Fatal("old model changed", err)
		}
	}
}

func TestLeakyBackwardRetainsNegativeGradient(t *testing.T) {
	var weights [ParameterCount]float32
	weights[0], weights[FeatureDim*HiddenDim] = 1, -2
	weights[w2Start], weights[w2Start+HiddenDim] = .5, -.5
	m, _ := NewForActivation(weights, decision.RelationalFlowFeatureVersion, LeakyReLUActivation)
	var input [1][FeatureDim]float32
	input[0][0] = 1
	var w Workspace
	if err := m.forward(input[:], []uint16{0, 1}, &w); err != nil {
		t.Fatal(err)
	}
	var residual [MaxChoices][2]float64
	residual[0] = [2]float64{.25, -.25}
	var gradient [ParameterCount]float64
	m.backward(input[:], &w, &residual, &gradient)
	if gradient[0] != .25*float64(negativeSlope) || gradient[FeatureDim*HiddenDim] != gradient[0] || gradient[w2Start] != -.25*float64(negativeSlope) {
		t.Fatal("negative gradient disappeared", gradient[0])
	}
	// Check the actual two-candidate loss on both sides of a negative activation.
	loss := func() float64 {
		if err := m.forward(input[:], []uint16{0, 1}, &w); err != nil {
			t.Fatal(err)
		}
		_, a := distribution(w.scores[:2], 3)
		_, b := distribution(w.scores[:2], 2)
		return a - b
	}
	if err := m.forward(input[:], []uint16{0, 1}, &w); err != nil {
		t.Fatal(err)
	}
	all, _ := distribution(w.scores[:2], 3)
	good, _ := distribution(w.scores[:2], 2)
	residual[0] = [2]float64{all[0] - good[0], all[1] - good[1]}
	clear(gradient[:])
	m.backward(input[:], &w, &residual, &gradient)
	for _, i := range []int{0, FeatureDim * HiddenDim, w2Start, w2Start + HiddenDim, b2Start} {
		old := m.weights[i]
		m.weights[i] = old + .001
		up := loss()
		m.weights[i] = old - .001
		down := loss()
		m.weights[i] = old
		if math.Abs((up-down)/.002-gradient[i]) > 1e-4 {
			t.Fatal("negative finite difference", i, (up-down)/.002, gradient[i])
		}
	}
}

func TestLeakyFreshFitAndInvalidActivation(t *testing.T) {
	m, h, err := FitForActivation(context.Background(), []Sample{fixture()}, FitOptions{Epochs: 3, LearningRate: .1, Seed: 17}, decision.RelationalFlowFeatureVersion, LeakyReLUActivation)
	if err != nil || len(h) != 3 || m.Activation() != LeakyReLUActivation {
		t.Fatal(err)
	}
	for _, name := range []string{"", "unknown"} {
		if _, err := NewForActivation([ParameterCount]float32{}, decision.RelationalFlowFeatureVersion, name); err == nil {
			t.Fatal("invalid activation")
		}
		if _, _, err := FitForActivation(context.Background(), []Sample{fixture()}, FitOptions{Epochs: 1, LearningRate: .1}, decision.RelationalFlowFeatureVersion, name); err == nil {
			t.Fatal("invalid fit activation")
		}
	}
}
