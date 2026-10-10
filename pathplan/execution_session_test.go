package pathplan

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/executiondecision"
)

func executionMaxPlan() Plan {
	p := conditionContractPlan()
	p.Base.Expressions = []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 10}, {Kind: "binary", Operation: "less_than", Left: 0, Right: 1}}
	p.Base.Statements = []bodyplan.Stmt{{Kind: "return", Expr: 0}, {Kind: "return", Expr: 1}, {Kind: "if", Expr: 2, Then: []int{0}, Else: []int{1}}}
	p.Base.Root = []int{2}
	p.Decisions[0].Target, p.Decisions[1].Target = 2, 2
	p.Decisions[0].Intent, p.Decisions[1].Intent = "입력과 10을 비교", "큰 값을 반환 choose maximum"
	p.ConditionCases = []ConditionCase{{ChoiceID: p.Decisions[0].ID, Input: -9007199254740995, Expected: true}}
	return p
}

// This deliberately wired model proves routing, not learned accuracy. Hidden
// unit 0 activates only for choice index 1 after an output mismatch is observed.
func wiredExecutionModel(t *testing.T) *executiondecision.Model {
	t.Helper()
	var weights [executiondecision.ParameterCount]float32
	weights[256], weights[311] = 8, 128
	weights[executiondecision.FeatureDim*executiondecision.HiddenDim] = -1
	weights[executiondecision.FeatureDim*executiondecision.HiddenDim+2*executiondecision.HiddenDim] = 2
	m, err := executiondecision.New(weights)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestOutputOnlyFailureImmediatelyReranksAndExecutesNewBody(t *testing.T) {
	ctx := sessionContext(t)
	p, err := Prepare(executionMaxPlan())
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{-9007199254740995, 10}, {9007199254740993, 9007199254740993}, {18014398509481990, 18014398509481990}}
	m := wiredExecutionModel(t)
	s, err := p.NewExecutionSession(ctx, m, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	initial, _ := s.Observe()
	first, _, err := s.Advance(ctx, 1)
	if err != nil || first.Search.NewAttempts[0].Mask != 0 || !ConditionsPassed(first.Search.NewAttempts[0].Conditions) || first.Search.NewAttempts[0].Passed != 0 {
		t.Fatal(first, err)
	}
	input, err := s.ExecutionInput()
	if err != nil {
		t.Fatal(err)
	}
	direct, err := p.ObserveExecutionInput(ctx, first.Search.NewAttempts[0].Choices, cases)
	if err != nil || !reflect.DeepEqual(input, direct) {
		t.Fatal("committed observation differs from direct evaluation", err)
	}
	r, err := s.ReconsiderExecution(ctx, m)
	if err != nil || r.HasFailure || r.OutputFailure == nil || r.OutputFailure.Result.Actual != -9007199254740995 || r.Calls != 1 || r.CumulativeCalls != 2 || r.Reused || !r.Applied || r.Proposed != 2 || r.AddedMask {
		t.Fatal(r, err)
	}
	if r.FeatureSHA == initial.Ranking.FeatureSHA || r.FeatureVersion != decision.ExecutionFeatureVersion {
		t.Fatal("output failure not in model input")
	}
	copy := r.OutputFailure.Result
	r.OutputFailure.Result.Actual = 0
	first.Search.NewAttempts[0].Results[0].Actual = 0
	input.outputFailure.Result.Actual = 0
	if s.latest.OutputFailure.Result != copy || s.input.outputFailure.Result != copy {
		t.Fatal("caller mutated retained evidence")
	}
	observed, _ := s.Observe()
	observed.Ranking.OutputFailure.Result.Input = 0
	if s.latest.OutputFailure.Result.Input == 0 {
		t.Fatal("borrowed observation")
	}
	next, body, err := s.Advance(ctx, 1)
	if err != nil || body == nil || next.Search.Status != "TRAINING_COMPLETE" || next.Search.NewAttempts[0].Mask != 2 || next.Search.Attempted != 2 {
		t.Fatal(next, err)
	}
	if s.input.hasOutputFailure || s.input.present {
		t.Fatal("stale failure after success")
	}
	value, err := body.Evaluate(18014398509481990)
	if err != nil || value.Int != 18014398509481990 {
		t.Fatal(value, err)
	}
	if _, err := s.ReconsiderExecution(ctx, m); err == nil {
		t.Fatal("reranked completed session")
	}
	result, _, records, rankings, err := p.SearchExecutionBatches(ctx, m, cases, 4, 1, "", 2)
	if err != nil || result.Status != "TRAINING_COMPLETE" || len(result.Attempts) != 2 || len(rankings) != 1 || result.Selection.ModelCalls != 2 {
		t.Fatal(result, err)
	}
	for _, row := range records {
		saved := row.SHA
		row.SHA = ""
		raw, _ := json.Marshal(row)
		if hash(raw) != saved {
			t.Fatal("progress digest")
		}
	}
}

func TestExecutionInputHasSeparateCasesAndSimultaneousFailures(t *testing.T) {
	ctx := sessionContext(t)
	plan := executionMaxPlan()
	plan.ConditionCases[0].Expected = false
	p, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{-9007199254740995, 9007199254740993}}
	initial, err := p.InitialExecutionInput(cases)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := p.InitialExecutionInput([]TestCase{{2, 3}})
	var a, b [decision.ExecutionFeatureDim]float32
	if err := initial.ExecutionFeaturesInto("branches", &a); err != nil {
		t.Fatal(err)
	}
	if err := other.ExecutionFeaturesInto("branches", &b); err != nil {
		t.Fatal(err)
	}
	if initial.CaseSHA256() == other.CaseSHA256() || a != b {
		t.Fatal("case identity missing or unseen expected output leaked")
	}
	input, err := p.ObserveExecutionInput(ctx, p.Defaults(), cases)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := input.Failure(); !ok {
		t.Fatal("condition failure lost")
	}
	failure, ok := input.OutputFailure()
	if !ok || failure.Result.Expected != 9007199254740993 {
		t.Fatal("output failure lost")
	}
	if err := input.ExecutionFeaturesInto("branches", &b); err != nil {
		t.Fatal(err)
	}
	if b[192] != 0.125 || b[256] != 0.125 {
		t.Fatal("simultaneous channels missing")
	}
	legacy, _ := p.InitialConditionInput()
	want := b
	if err := legacy.ExecutionFeaturesInto("branches", &b); err == nil || b != want {
		t.Fatal("unbound execution input accepted")
	}
	if err := input.ExecutionFeaturesInto("missing", &b); err == nil || b != want {
		t.Fatal("invalid choice changed output")
	}
	if _, err := p.ObserveExecutionInput(context.Background(), p.Defaults(), cases); err == nil {
		t.Fatal("unbounded evaluation accepted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := p.ObserveExecutionInput(cancelled, p.Defaults(), cases); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled observation retained")
	}
	invalid, _ := Prepare(interactingPlan())
	if got, err := invalid.ObserveExecutionInput(ctx, map[string]string{"reference": "reference_second", "order": "schedule_reverse"}, cases); err == nil || got != nil {
		t.Fatal("invalid combined selection observed")
	}
}

func TestExecutionSessionNilModelParityAndWrongABI(t *testing.T) {
	ctx := sessionContext(t)
	p, _ := Prepare(executionMaxPlan())
	cases := []TestCase{{-1, 10}, {20, 20}}
	want, body, _, err := p.SearchBatches(ctx, nil, cases, 4, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	got, other, _, r, err := p.SearchExecutionBatches(ctx, nil, cases, 4, 1, "", 0)
	if err != nil || !reflect.DeepEqual(want, got) || body.GoSource() != other.GoSource() || len(r) != 0 {
		t.Fatal("deterministic path changed", err)
	}
	s, err := p.NewExecutionSession(ctx, wiredExecutionModel(t), cases, "")
	if err != nil {
		t.Fatal(err)
	}
	s.Advance(ctx, 1)
	if _, err := s.Reconsider(ctx, zeroConditionModel(t)); err == nil {
		t.Fatal("legacy model accepted")
	}
	s.lock.Lock()
	if _, err := s.ExecutionInput(); !errors.Is(err, ErrSessionBusy) {
		t.Fatal("busy observation waited")
	}
	s.lock.Unlock()
}
