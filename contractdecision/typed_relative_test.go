package contractdecision

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func typedRelativeFixture() InteractionRequirementSample {
	s := v2SyntheticSample()
	conditions := make(conditionRows, 3)
	for i, value := range []int64{-503, 0, 607} {
		if err := decision.DeclaredConditionFeaturesInto(decision.DeclaredConditionFeatureInput{
			Input: value, Expected: i == 0, TargetChoice: 0, ChoiceCount: 2, Index: i, Count: 3,
		}, &conditions[i]); err != nil {
			panic(err)
		}
	}
	s.Conditions = conditions
	for i := range s.Inputs {
		clear(s.Inputs[i][:5])
		s.Inputs[i][2+i] = .25
		s.Inputs[i][192], s.Inputs[i][200], s.Inputs[i][202], s.Inputs[i][204], s.Inputs[i][206] = 1, 1.0/3, 1.0/3, 1.0/3, 3.0/128
		s.Inputs[i][208], s.Inputs[i][210] = 1, 1
		for branch := range 2 {
			s.Inputs[i][256+branch*48], s.Inputs[i][258+branch*48] = 1, 1
		}
	}
	return s
}

func TestTypedSourceRelativeRoutesByKindAndKeepsHeadsIndependent(t *testing.T) {
	if TypedSourceRelativeHeadParameterCount != 9182 || TypedSourceRelativeParameterCount != 18364 || TypedSourceRelativeParameterCount >= InteractionRequirementParameterCount {
		t.Fatal("typed tensor layout changed")
	}
	s := typedRelativeFixture()
	var weights [2][TypedSourceRelativeHeadParameterCount]float32
	weights[0][typedScoreBias+1], weights[1][typedScoreBias] = 1, 2
	m, err := NewTypedSourceRelative(weights)
	if err != nil {
		t.Fatal(err)
	}
	var w TypedSourceRelativeWorkspace
	var out ChoicePrediction
	if err := m.PredictChoicesInto(s.Inputs, s.Cases, s.Conditions, &w, &out); err != nil {
		t.Fatal(err)
	}
	if out.Logits[0] != [2]float32{0, 1} || out.Logits[1] != [2]float32{2, 0} {
		t.Fatal(out)
	}
	weights[1][typedScoreBias] = -3
	changed, _ := NewTypedSourceRelative(weights)
	var other ChoicePrediction
	if err := changed.PredictChoicesInto(s.Inputs, s.Cases, s.Conditions, &w, &other); err != nil {
		t.Fatal(err)
	}
	if other.Logits[0] != out.Logits[0] || other.Logits[1] == out.Logits[1] {
		t.Fatal("head parameters leaked")
	}
	s.Inputs[0], s.Inputs[1] = s.Inputs[1], s.Inputs[0]
	var swapped ChoicePrediction
	if err := m.PredictChoicesInto(s.Inputs, s.Cases, s.Conditions, &w, &swapped); err != nil {
		t.Fatal(err)
	}
	if swapped.Logits[0] != out.Logits[1] || swapped.Logits[1] != out.Logits[0] {
		t.Fatal("routing followed ordinal")
	}
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			var local TypedSourceRelativeWorkspace
			var prediction Prediction
			if err := m.PredictInto(s.Inputs, s.Cases, s.Conditions, []uint16{3, 2, 1, 0}, &local, &prediction); err != nil || prediction.Selected != 2 {
				t.Error("concurrent mask ranking", err, prediction)
			}
		})
	}
	wg.Wait()
}

func TestTypedSourceRelativeArtifactHasExactShapeAndIdentity(t *testing.T) {
	m := &TypedSourceRelativeModel{weights: initialTypedRelative(2026101261)}
	if err := m.bindFingerprint(); err != nil {
		t.Fatal(err)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeTypedSourceRelative(raw)
	if err != nil || decoded.weights != m.weights || decoded.Fingerprint() != fmt.Sprintf("%x", sha256.Sum256(raw)) {
		t.Fatal("typed artifact", err)
	}
	again, _ := decoded.Marshal()
	if !bytes.Equal(raw, again) {
		t.Fatal("artifact roundtrip")
	}
	if _, err := DecodeSourceRelativeV4(raw); err == nil {
		t.Fatal("V4 accepted typed head artifact")
	}
	for _, bad := range [][]byte{nil, append(bytes.Clone(raw), []byte(" {}")...), append([]byte(`{"head_routing":"duplicate",`), raw[1:]...), bytes.Repeat([]byte(" "), (512<<10)+1)} {
		if _, err := DecodeTypedSourceRelative(bad); err == nil {
			t.Fatal("invalid artifact accepted")
		}
	}
	for _, mutation := range []struct{ key, value string }{
		{"schema", `"wrong"`}, {"head_routing", `"wrong"`}, {"source_features", `"wrong"`},
		{"intent_policy", `null`}, {"pool_width_per_head", `8`}, {"hidden_width_per_head", `24`},
		{"weights", `[]`}, {"weights", `[[],[]]`}, {"weights", `[null,null]`}, {"extra", `true`},
	} {
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(raw, &fields)
		fields[mutation.key] = json.RawMessage(mutation.value)
		bad, _ := json.Marshal(fields)
		if _, err := DecodeTypedSourceRelative(bad); err == nil {
			t.Fatal("bad field", mutation.key)
		}
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	var heads [][]json.RawMessage
	_ = json.Unmarshal(fields["weights"], &heads)
	heads[1][91] = json.RawMessage("null")
	fields["weights"], _ = json.Marshal(heads)
	bad, _ := json.Marshal(fields)
	if _, err := DecodeTypedSourceRelative(bad); err == nil {
		t.Fatal("null tensor value")
	}
	weights := m.Weights()
	weights[1][7] = float32(math.Inf(1))
	if _, err := NewTypedSourceRelative(weights); err == nil {
		t.Fatal("nonfinite tensor")
	}
}

func TestTypedSourceRelativeErrorsPreserveDestinations(t *testing.T) {
	m, _ := NewTypedSourceRelative([2][TypedSourceRelativeHeadParameterCount]float32{})
	for _, cell := range []int{0, 3, 64, 191, 352, 527} {
		s := typedRelativeFixture()
		s.Inputs[0][cell] = float32(math.NaN())
		var w TypedSourceRelativeWorkspace
		w.scores[0] = 71
		before := w
		out := Prediction{Count: 31, Selected: 29}
		if err := m.PredictInto(s.Inputs, s.Cases, s.Conditions, s.Masks, &w, &out); err == nil || w != before || out.Count != 31 || out.Selected != 29 {
			t.Fatal("error changed destination", cell)
		}
	}
	s := typedRelativeFixture()
	for _, kinds := range [][5]float32{{}, {0, 0, .2, .2, 0}, {1, 0, 0, 0, 0}, {0, 0, -1, 0, 0}} {
		copy(s.Inputs[0][:5], kinds[:])
		var w TypedSourceRelativeWorkspace
		var out ChoicePrediction
		if err := m.PredictChoicesInto(s.Inputs, s.Cases, s.Conditions, &w, &out); err == nil {
			t.Fatal("ambiguous kind")
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if model, history, err := FitTypedSourceRelative(cancelled, []InteractionRequirementSample{typedRelativeFixture()}, FitOptions{Epochs: 1, LearningRate: .01}); !errors.Is(err, context.Canceled) || model != nil || history != nil {
		t.Fatal("cancelled fit", err)
	}
}

func TestTypedSourceRelativeFitUpdatesOnlyTheRoutedHead(t *testing.T) {
	s := typedRelativeFixture()
	s.Inputs[1][2], s.Inputs[1][3] = .25, 0
	s.Inputs[1][196], s.Inputs[1][197] = 1, 0
	options := FitOptions{Seed: 2026101263, Epochs: 1, LearningRate: .02, L2: 0}
	initial := initialTypedRelative(options.Seed)
	m, history, err := FitTypedSourceRelative(context.Background(), []InteractionRequirementSample{s}, options)
	if err != nil || len(history) != 1 {
		t.Fatal("synthetic one-step fit", err)
	}
	if m.weights[0] == initial[0] || m.weights[1] != initial[1] {
		t.Fatal("gradient crossed routed head")
	}
	raw, err := m.Marshal()
	if err != nil || m.Fingerprint() != fmt.Sprintf("%x", sha256.Sum256(raw)) {
		t.Fatal("fitted identity", err)
	}
}

func TestTypedSourceRelativeHeadGradient(t *testing.T) {
	s := typedRelativeFixture()
	m := &TypedSourceRelativeModel{weights: initialTypedRelative(2026101267)}
	var w TypedSourceRelativeWorkspace
	if err := w.capture(s.Cases, s.Conditions); err != nil {
		t.Fatal(err)
	}
	if err := m.forward(s.Inputs, s.Masks, &w); err != nil {
		t.Fatal(err)
	}
	all, _ := distribution(w.scores[:len(s.Masks)], allCandidates(len(s.Masks)))
	good, _ := distribution(w.scores[:len(s.Masks)], s.Acceptable)
	var residual [2][2]float64
	for i, mask := range s.Masks {
		for c := range s.Inputs {
			residual[c][mask>>c&1] += all[i] - good[i]
		}
	}
	var gradient [2][TypedSourceRelativeHeadParameterCount]float64
	for choice, input := range s.Inputs {
		m.weights[choice].backward(choice, input, &w.cases, &w.choices[choice], residual[choice], &gradient[choice])
	}
	loss := func() float64 {
		var local TypedSourceRelativeWorkspace
		if err := local.capture(s.Cases, s.Conditions); err != nil {
			t.Fatal(err)
		}
		if err := m.forward(s.Inputs, s.Masks, &local); err != nil {
			t.Fatal(err)
		}
		_, a := distribution(local.scores[:len(s.Masks)], allCandidates(len(s.Masks)))
		_, b := distribution(local.scores[:len(s.Masks)], s.Acceptable)
		return a - b
	}
	indices := []int{0, 42, typedOutputBias, typedConditionStart + 1, typedConditionStart + 43, typedConditionBias, typedContextStart + 193, typedContextBias, typedHiddenBias, typedScoreStart, typedScoreBias, typedScoreBias + 1}
	for h := range typedHiddenDim {
		for term := range interactionTermCount {
			indices = append(indices, typedHiddenStart+h*typedJointDim+OrderedFeatureDim+term*typedPoolDim)
		}
		indices = append(indices, typedHiddenStart+h*typedJointDim+2, typedHiddenStart+h*typedJointDim+256, typedScoreStart+h)
	}
	for head := range 2 {
		for _, i := range indices {
			old, epsilon := m.weights[head][i], float32(.001)
			m.weights[head][i] = old + epsilon
			plus := loss()
			m.weights[head][i] = old - epsilon
			minus := loss()
			m.weights[head][i] = old
			numerical := (plus - minus) / float64(2*epsilon)
			if math.Abs(numerical-gradient[head][i]) > .001 {
				t.Fatalf("head%d parameter%d: analytic%g numerical%g", head, i, gradient[head][i], numerical)
			}
		}
	}
}
