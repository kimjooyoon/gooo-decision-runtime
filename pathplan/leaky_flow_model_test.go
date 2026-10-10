package pathplan

import (
	"errors"
	"reflect"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
)

func TestLeakyFlowSessionUsesSerializedComputationAndRejectsSubstitution(t *testing.T) {
	var weights [flowdecision.ParameterCount]float32
	hiddenBias := flowdecision.FeatureDim * flowdecision.HiddenDim
	outputWeights := hiddenBias + flowdecision.HiddenDim
	weights[hiddenBias] = -1
	weights[outputWeights], weights[outputWeights+flowdecision.HiddenDim] = .5, -.5
	leaky, err := flowdecision.NewForActivation(weights, decision.RelationalFlowFeatureVersion, flowdecision.LeakyReLUActivation)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := leaky.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	leaky, err = flowdecision.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	old, err := flowdecision.NewForFeatures(weights, decision.RelationalFlowFeatureVersion)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Prepare(executionMaxPlan())
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{-9007199254740995, 10}, {9007199254740993, 9007199254740993}}
	ctx := sessionContext(t)
	s, err := p.NewFlowSession(ctx, leaky, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	observed, err := s.Observe()
	if err != nil {
		t.Fatal(err)
	}
	if observed.Ranking.Proposed != 3 || observed.Ranking.ModelFingerprint != leaky.Fingerprint() || s.modelSchema != flowdecision.ActivationSchema {
		t.Fatal("leaky computation not routed", observed)
	}
	original, err := p.NewFlowSession(ctx, old, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	orig, err := original.Observe()
	if err != nil || orig.Ranking.Proposed != 0 || original.modelSchema != flowdecision.Schema {
		t.Fatal("ReLU default changed", err)
	}
	if _, _, err = s.Advance(ctx, 1); !errors.Is(err, ErrNoConditionCandidate) {
		t.Fatal(err)
	}
	before, err := s.Observe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReconsiderFlow(ctx, old); err == nil {
		t.Fatal("same weights with wrong activation accepted")
	}
	after, err := s.Observe()
	if err != nil || after.Search.Sequence != before.Search.Sequence+1 || after.Search.PreviousSHA != before.Search.SHA {
		t.Fatal("observation chain changed", err)
	}
	// Observe appends a receipt even when no candidate or ranking changes.
	after.SHA, after.Search.SHA = before.SHA, before.Search.SHA
	after.Search.Sequence, after.Search.PreviousSHA = before.Search.Sequence, before.Search.PreviousSHA
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed substitution changed session", err)
	}
	result, _, _, _, err := p.SearchFlowBatches(ctx, leaky, cases, 4, 1, "", 4)
	if err != nil || result.Status != "TRAINING_COMPLETE" {
		t.Fatal(result, err)
	}
}
