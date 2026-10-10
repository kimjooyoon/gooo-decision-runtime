package pathplan

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
)

func zeroConditionModel(t *testing.T) *conditiondecision.Model {
	t.Helper()
	m, err := conditiondecision.New([conditiondecision.ParameterCount]float32{})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestConditionSessionRejectsWrongConditionsThenCompletes(t *testing.T) {
	ctx := sessionContext(t)
	p, err := Prepare(conditionContractPlan())
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{-9007199254740995, -9007199254740995}, {9007199254740993, 9007199254740993}, {18014398509481990, 18014398509481990}}
	m := zeroConditionModel(t)
	s, err := p.NewConditionSession(ctx, m, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	initial, err := s.Observe()
	if err != nil {
		t.Fatal(err)
	}
	if initial.Ranking.Calls != 1 || !initial.Ranking.Applied || initial.Search.Selection.ModelCalls != 1 || initial.Search.Selection.WeightsSHA256 != "" {
		t.Fatal("ranking identity or actual call count")
	}
	first, body, err := s.Advance(ctx, 1)
	if !errors.Is(err, ErrNoConditionCandidate) || body != nil || first.Search.NewAttempts[0].Passed != 3 || first.Search.ConditionRejected != 1 {
		t.Fatal("output-only success accepted", err)
	}
	r, err := s.Reconsider(ctx, m)
	if err != nil || r.Calls != 1 || !r.Applied || r.CumulativeCalls != 2 || !r.HasFailure || r.Failure.Result.Case.Input != -9007199254740995 || r.Failure.Mask != 0 {
		t.Fatal("condition observation lost", r, err)
	}
	input, _ := p.ObserveConditionInput(ctx, first.Search.NewAttempts[0].Choices)
	if !reflect.DeepEqual(input, &s.input) {
		t.Fatal("committed input differs from direct candidate observation")
	}
	// Returned evidence cannot mutate the retained observation or ranking.
	r.Failure.Result.Case.Input = 7
	r.Logits[0][0] = 99
	first.Search.NewAttempts[0].Conditions[0].Case.Input = 7
	if s.latest.Failure.Result.Case.Input != -9007199254740995 || s.latest.Logits[0][0] != 0 {
		t.Fatal("borrowed receipt")
	}
	if _, err = s.Reconsider(ctx, m); err == nil {
		t.Fatal("repeated feedback without new attempt")
	}
	next, body, err := s.Advance(ctx, 1)
	if err != nil || body == nil || next.Search.Status != "TRAINING_COMPLETE" || next.Search.NewAttempts[0].Mask != 1 || next.Search.Attempted != 2 {
		t.Fatal("did not advance to valid distinct path", err)
	}
	if s.input.present {
		t.Fatal("stale condition failure retained after passing condition")
	}
	result, selected, records, feedback, err := p.SearchConditionBatches(ctx, m, cases, 4, 1, "", 2)
	if err != nil || result.Status != "TRAINING_COMPLETE" || len(result.Attempts) != 2 || selected.GoSource() != body.GoSource() || len(feedback) != 1 || len(records) != 4 {
		t.Fatal("batch cannot continue past rejected condition", err)
	}
	for _, record := range records {
		saved := record.SHA
		record.SHA = ""
		raw, _ := json.Marshal(record)
		if hash(raw) != saved {
			t.Fatal("combined progress digest")
		}
	}
}

func TestConditionSessionDeterministicParityReuseAndBounds(t *testing.T) {
	ctx := sessionContext(t)
	p, _ := Prepare(chainedDecisionPlan(7))
	cases := []TestCase{{3, 999}}
	want, wantBody, wantProgress, err := p.SearchBatches(ctx, nil, cases, 64, 8, "")
	if err != nil {
		t.Fatal(err)
	}
	got, body, progress, feedback, err := p.SearchConditionBatches(ctx, nil, cases, 64, 8, "", 0)
	if err != nil || !reflect.DeepEqual(want, got) || body.GoSource() != wantBody.GoSource() || feedback != nil || len(progress) != len(wantProgress) {
		t.Fatal("deterministic mode changed", err)
	}
	for i := range progress {
		if !reflect.DeepEqual(progress[i].Search, wantProgress[i]) {
			t.Fatal("deterministic progress changed", i)
		}
	}
	m := zeroConditionModel(t)
	s, err := p.NewConditionSession(ctx, m, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[uint16]bool{}
	for i := range 17 {
		step, _, err := s.Advance(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		mask := step.Search.NewAttempts[0].Mask
		if seen[mask] {
			t.Fatal("committed path repeated")
		}
		seen[mask] = true
		r, err := s.Reconsider(ctx, m)
		if i == 16 {
			if err == nil {
				t.Fatal("round limit ignored")
			}
			break
		}
		if err != nil || r.Calls != 0 || !r.Reused || r.CumulativeCalls != 1 || r.Applied {
			t.Fatal("unchanged features reranked", r, err)
		}
	}
	high, _ := Prepare(chainedDecisionPlan(16))
	s, err = high.NewConditionSession(ctx, m, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	row, _, err := s.Advance(ctx, 1)
	if err != nil || row.Search.Declared != 65536 || row.Search.ScheduledBytes != 8192 || row.Ranking.ChoiceCount != 16 || row.Ranking.Calls != 1 || row.Search.FrontierNodes > 16 {
		t.Fatal("wide choice bounds", err)
	}
	for _, args := range [][3]int{{65, 1, 0}, {4, 5, 0}, {4, 0, 0}, {4, 1, 17}, {4, 1, -1}} {
		if _, _, _, _, err := p.SearchConditionBatches(ctx, m, cases, args[0], args[1], "", args[2]); err == nil {
			t.Fatal("invalid bounds", args)
		}
	}
	if _, _, _, _, err := p.SearchConditionBatches(ctx, nil, cases, 4, 1, "", 1); err == nil {
		t.Fatal("feedback without model")
	}
}

type conditionCancelContext struct {
	context.Context
	calls, limit int
}

func (c *conditionCancelContext) Err() error {
	c.calls++
	if c.calls >= c.limit {
		return context.Canceled
	}
	return nil
}

func TestConditionSessionFeedbackCancellationAndIdentityAreAtomic(t *testing.T) {
	ctx := sessionContext(t)
	p, _ := Prepare(conditionContractPlan())
	m := zeroConditionModel(t)
	s, err := p.NewConditionSession(ctx, m, []TestCase{{1, 999}}, "")
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = s.Advance(ctx, 1)
	beforeQueue := append(searchHeap(nil), s.core.queue...)
	beforeScheduled := append([]uint64(nil), s.core.scheduled...)
	beforeWeights := s.core.logWeights
	weights := m.Weights()
	weights[0] = 1
	other, _ := conditiondecision.New(weights)
	if _, err = s.Reconsider(ctx, other); err == nil || s.core.feedbackRounds != 0 {
		t.Fatal("changed model accepted")
	}
	r, err := s.Reconsider(&conditionCancelContext{Context: ctx, limit: 3}, m)
	if !errors.Is(err, context.Canceled) || r.Calls != 1 || r.Applied || r.AddedMask || r.CumulativeCalls != 2 {
		t.Fatal("canceled prediction not accounted", r, err)
	}
	if !reflect.DeepEqual(beforeQueue, s.core.queue) || !reflect.DeepEqual(beforeScheduled, s.core.scheduled) || beforeWeights != s.core.logWeights || s.core.attempted != 1 {
		t.Fatal("canceled ranking changed search")
	}
	for _, lock := range []*sync.Mutex{&s.lock, &s.core.lock} {
		lock.Lock()
		if _, _, err := s.Advance(ctx, 1); !errors.Is(err, ErrSessionBusy) {
			t.Fatal("busy advance waited")
		}
		if _, err := s.Observe(); !errors.Is(err, ErrSessionBusy) {
			t.Fatal("busy observation waited")
		}
		lock.Unlock()
	}
	// Concurrent readers have owned values or a prompt busy result.
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 10 {
				r, err := s.Observe()
				if err != nil && !errors.Is(err, ErrSessionBusy) {
					t.Error(err)
				}
				r.Ranking.Logits[0][0] = 100
			}
		})
	}
	wg.Wait()
	if s.latest.Logits[0][0] != 0 {
		t.Fatal("concurrent observation mutated ranking")
	}
}

func TestConditionSessionUnnecessaryLastMaskAndRepresentationDecline(t *testing.T) {
	ctx := sessionContext(t)
	p, _ := Prepare(chainedDecisionPlan(1))
	m := zeroConditionModel(t)
	s, err := p.NewConditionSession(ctx, m, []TestCase{{1, 999}}, "")
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = s.Advance(ctx, 1)
	r, err := s.Reconsider(ctx, m)
	if err != nil || !r.Unnecessary || r.Calls != 0 {
		t.Fatal("last mask invoked model", err)
	}
	// Two valid branch-local declarations share a spelling. The compiler can
	// resolve them, while this optional model representation declines them.
	plan := structuralFixture(BranchLayout)
	plan.Base.Statements[2] = bodyplan.Stmt{Kind: "let", Name: "inside", Expr: 2}
	plan.Base.Statements[3] = bodyplan.Stmt{Kind: "let", Name: "inside", Expr: 3}
	firstRead := len(plan.Base.Expressions)
	plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "local", Name: "inside"}, bodyplan.Expr{Kind: "local", Name: "inside"})
	firstWrite := len(plan.Base.Statements)
	plan.Base.Statements = append(plan.Base.Statements, bodyplan.Stmt{Kind: "assign", Name: "first", Expr: firstRead}, bodyplan.Stmt{Kind: "assign", Name: "first", Expr: firstRead + 1})
	plan.Base.Statements[5].Then, plan.Base.Statements[5].Else = []int{2, firstWrite}, []int{3, firstWrite + 1}
	plan = pruneStructuralFixture(plan)
	p, err = Prepare(plan)
	if err != nil {
		t.Fatal("decline fixture must compile", err)
	}
	s, err = p.NewConditionSession(ctx, m, []TestCase{{1, 999}}, "")
	if err != nil {
		t.Fatal(err)
	}
	observed, err := s.Observe()
	if err != nil || !observed.Ranking.Declined || observed.Ranking.Calls != 0 || s.core.ranked {
		t.Fatal("unsupported representation used model", observed.Ranking, err)
	}
	_, _, _, feedback, err := p.SearchConditionBatches(ctx, m, []TestCase{{1, 999}}, 2, 1, "", 2)
	if err != nil || len(feedback) != 0 {
		t.Fatal("declined representation failed deterministic continuation", err)
	}
}

func TestConditionSessionNumericalFailurePreservesActualCall(t *testing.T) {
	ctx := sessionContext(t)
	p, _ := Prepare(chainedDecisionPlan(2))
	var weights [conditiondecision.ParameterCount]float32
	for i := range weights {
		weights[i] = math.MaxFloat32
	}
	m, _ := conditiondecision.New(weights)
	s, err := p.NewConditionSession(ctx, m, []TestCase{{1, 999}}, "")
	if err == nil || s == nil {
		t.Fatal("overflow accepted")
	}
	r, err := s.Observe()
	if err != nil || r.Search.Initialized || r.Ranking.Calls != 1 || r.Ranking.CumulativeCalls != 1 || r.Ranking.PredictionValid || r.Ranking.Applied || r.Search.Attempted != 0 {
		t.Fatal("failed initialization evidence lost", err)
	}
}

func TestConditionSessionObservationChangesFrontierWithoutRepeatingCandidate(t *testing.T) {
	ctx := sessionContext(t)
	p, _ := Prepare(conditionContractPlan())
	var weights [conditiondecision.ParameterCount]float32
	weights[192] = 8                                                                                    // first hidden unit reads the condition-present flag
	weights[conditiondecision.FeatureDim*conditiondecision.HiddenDim+2*conditiondecision.HiddenDim] = 2 // option-one head
	m, _ := conditiondecision.New(weights)
	s, err := p.NewConditionSession(ctx, m, []TestCase{{-1, -1}}, "")
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := s.Advance(ctx, 1)
	if !errors.Is(err, ErrNoConditionCandidate) {
		t.Fatal(err)
	}
	r, err := s.Reconsider(ctx, m)
	if err != nil || r.Proposed != 3 || !r.AddedMask || !r.Applied || r.Logits[0][1] <= r.Logits[0][0] {
		t.Fatal("feedback score did not reorder frontier", r, err)
	}
	next, body, err := s.Advance(ctx, 1)
	if err != nil || body == nil || next.Search.Status != "TRAINING_COMPLETE" || next.Search.NewAttempts[0].Mask != 3 || first.Search.NewAttempts[0].Mask != 0 || next.Search.Attempted != 2 {
		t.Fatal("new proposal did not execute next", err)
	}
}

func TestConditionSessionCancellationDuringInitialCallKeepsReceipt(t *testing.T) {
	ctx := sessionContext(t)
	p, _ := Prepare(conditionContractPlan())
	m := zeroConditionModel(t)
	s, err := p.NewConditionSession(&conditionCancelContext{Context: ctx, limit: 6}, m, []TestCase{{1, 999}}, "")
	if !errors.Is(err, context.Canceled) || s == nil {
		t.Fatal("initial cancellation not returned", err)
	}
	r, err := s.Observe()
	if err != nil || r.Search.Initialized || r.Search.Attempted != 0 || r.Ranking.Calls != 1 || r.Ranking.Applied || !r.Ranking.PredictionValid {
		t.Fatal("initial completed prediction lost", r, err)
	}
}
