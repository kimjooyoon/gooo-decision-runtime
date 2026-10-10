package pathplan

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
)

func wiredFlowModel(t *testing.T, feedback bool) *flowdecision.Model {
	t.Helper()
	var weights [flowdecision.ParameterCount]float32
	if feedback {
		weights[256], weights[311] = 8, 128
		weights[flowdecision.FeatureDim*flowdecision.HiddenDim] = -1
	} else {
		weights[321] = 8 // source then-return carries the input
	}
	weights[flowdecision.FeatureDim*flowdecision.HiddenDim+2*flowdecision.HiddenDim] = 2
	m, err := flowdecision.New(weights)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func flowDigest(features [384]float32) string {
	var raw [384 * 4]byte
	for i, x := range features {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(x))
	}
	return hash(raw[:])
}

func TestFlowSessionUsesAssignmentFactsBeforeFirstBody(t *testing.T) {
	ctx := sessionContext(t)
	cases := []TestCase{{-9007199254740995, 10}, {9007199254740993, 9007199254740993}, {18014398509481990, 18014398509481990}}
	m := wiredFlowModel(t, false)
	for _, reversed := range []bool{false, true} {
		p, _ := Prepare(assignmentFlowPlan(reversed))
		s, err := p.NewFlowSession(ctx, m, cases, "")
		if err != nil {
			t.Fatal(err)
		}
		before, _ := s.Observe()
		want := uint16(1)
		if reversed {
			want = 0
		}
		if before.Search.Attempted != 0 || before.Ranking.Proposed != want || before.Ranking.Calls != 1 || before.Ranking.FeatureVersion != decision.ExecutionFlowFeatureVersion {
			t.Fatal("flow did not reach initial ranking", before)
		}
		input, err := s.ExecutionInput()
		var features [384]float32
		if err != nil || input.ExecutionFlowFeaturesInto("branches", &features) != nil || flowDigest(features) != before.Ranking.FeatureSHA[0] {
			t.Fatal("receipt does not bind actual 384-cell input", err)
		}
		after, body, err := s.Advance(ctx, 1)
		if err != nil || body == nil || after.Search.Attempted != 1 || after.Search.Status != "TRAINING_COMPLETE" || after.Search.NewAttempts[0].Mask != want {
			t.Fatal("selected path was not immediately constructed and checked", after, err)
		}
	}
}

func TestFlowSessionFeedsCommittedOutputAndPreservesStaticFacts(t *testing.T) {
	ctx := sessionContext(t)
	p, _ := Prepare(executionMaxPlan())
	cases := []TestCase{{-9007199254740995, 10}, {9007199254740993, 9007199254740993}}
	m := wiredFlowModel(t, true)
	s, err := p.NewFlowSession(ctx, m, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	input, _ := s.ExecutionInput()
	var before, after [384]float32
	if err := input.ExecutionFlowFeaturesInto("branches", &before); err != nil {
		t.Fatal(err)
	}
	p1, _, err := s.Advance(ctx, 1)
	if err != nil || p1.Search.NewAttempts[0].Mask != 0 || !ConditionsPassed(p1.Search.NewAttempts[0].Conditions) {
		t.Fatal(p1, err)
	}
	input, _ = s.ExecutionInput()
	if err := input.ExecutionFlowFeaturesInto("branches", &after); err != nil || !reflect.DeepEqual(before[320:], after[320:]) || before == after {
		t.Fatal("failure changed static facts or disappeared", err)
	}
	r, err := s.ReconsiderFlow(ctx, m)
	if err != nil || r.OutputFailure == nil || r.OutputFailure.Result.Actual != -9007199254740995 || r.Calls != 1 || r.Proposed != 2 || !r.Applied || r.FeatureSHA[1] != flowDigest(after) {
		t.Fatal(r, err)
	}
	p2, body, err := s.Advance(ctx, 1)
	if err != nil || body == nil || p2.Search.Status != "TRAINING_COMPLETE" || p2.Search.NewAttempts[0].Mask != 2 {
		t.Fatal(p2, err)
	}
	result, _, _, rankings, err := p.SearchFlowBatches(ctx, m, cases, 4, 1, "", 2)
	if err != nil || len(result.Attempts) != 2 || result.Selection.ModelCalls != 2 || len(rankings) != 1 {
		t.Fatal(result, err)
	}
}

func TestFlowSessionDeterministicContinuationAndBusyCancellation(t *testing.T) {
	ctx := sessionContext(t)
	p, _ := Prepare(executionMaxPlan())
	cases := []TestCase{{-1, 10}, {20, 20}}
	want, body, _, err := p.SearchBatches(ctx, nil, cases, 4, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	got, other, _, rankings, err := p.SearchFlowBatches(ctx, nil, cases, 4, 1, "", 0)
	if err != nil || !reflect.DeepEqual(want, got) || body.GoSource() != other.GoSource() || len(rankings) != 0 {
		t.Fatal("nil flow model changed deterministic search", err)
	}
	m := wiredFlowModel(t, true)
	s, err := p.NewFlowSession(ctx, m, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	s.Advance(ctx, 1)
	if _, err := s.ReconsiderExecution(ctx, wiredExecutionModel(t)); err == nil {
		t.Fatal("different artifact ABI accepted")
	}
	s.lock.Lock()
	if _, err := s.ReconsiderFlow(ctx, m); !errors.Is(err, ErrSessionBusy) {
		t.Fatal("busy ranking waited", err)
	}
	s.lock.Unlock()
	before, _ := s.Observe()
	queue, scheduled := slices.Clone(s.core.queue), slices.Clone(s.core.scheduled)
	weights := s.core.logWeights
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.ReconsiderFlow(cancelled, m); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	after, _ := s.Observe()
	if before.Search.Attempted != after.Search.Attempted || before.Ranking.CumulativeCalls != after.Ranking.CumulativeCalls ||
		!slices.Equal(queue, s.core.queue) || !slices.Equal(scheduled, s.core.scheduled) || weights != s.core.logWeights {
		t.Fatal("cancellation advanced frontier or called model")
	}
}
