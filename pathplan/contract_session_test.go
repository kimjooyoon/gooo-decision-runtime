package pathplan

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func pairedContractPlan() Plan {
	p := executionMaxPlan()
	p.Decisions = p.Decisions[1:]
	p.Decisions[0].Intent = "선언된 입출력 사례에 맞는 값을 반환 return the value required by the declared cases"
	p.ConditionCases = nil
	return p
}

func TestContractModelLearnsSameSourceWithDifferentDeclaredGoals(t *testing.T) {
	ctx := sessionContext(t)
	p, err := Prepare(pairedContractPlan())
	if err != nil {
		t.Fatal(err)
	}
	suites := [][]TestCase{
		{{-9007199254740995, 10}, {9007199254740993, 9007199254740993}, {18014398509481990, 18014398509481990}},
		{{-9007199254740995, -9007199254740995}, {9007199254740993, 10}, {18014398509481990, 10}},
	}
	samples := make([]contractdecision.Sample, 2)
	for i, cases := range suites {
		input, err := p.InitialContractInput(cases)
		if err != nil {
			t.Fatal(err)
		}
		features := make([][384]float32, 1)
		if err := input.RelationalSourceFeaturesInto(p.plan.Decisions[0].ID, &features[0]); err != nil {
			t.Fatal(err)
		}
		samples[i] = contractdecision.Sample{Inputs: features, Cases: input, Masks: []uint16{0, 1}}
		for mask := range 2 {
			choice := p.plan.Decisions[0]
			compiled, err := p.Compile(map[string]string{choice.ID: choice.Options[mask].Label})
			if err != nil {
				t.Fatal(err)
			}
			all := true
			for _, c := range cases {
				v, err := compiled.Evaluate(c.Input)
				all = all && err == nil && v.Int == c.Expected
			}
			if all {
				samples[i].Acceptable |= 1 << mask
			}
		}
		if samples[i].Acceptable == 0 {
			t.Fatal("no compiled oracle label")
		}
	}
	if !reflect.DeepEqual(samples[0].Inputs, samples[1].Inputs) || samples[0].Acceptable == samples[1].Acceptable {
		t.Fatal("paired source fixture is not ambiguous")
	}
	model, _, err := contractdecision.Fit(ctx, samples, contractdecision.FitOptions{Epochs: 2000, LearningRate: 0.3, Seed: 17})
	if err != nil {
		t.Fatal(err)
	}
	var rankings [2]ContractRanking
	for i, cases := range suites {
		s, err := p.NewContractSession(ctx, model, cases)
		if err != nil {
			t.Fatal(err)
		}
		r := s.Ranking()
		rankings[i] = r
		initial, err := s.Observe()
		if err != nil || initial.Attempted != 0 || r.Calls != 1 || !r.Applied || r.Declined || r.ModelFingerprint != model.Fingerprint() || r.CaseCount != len(cases) || r.SourceFeatures != decision.RelationalFlowFeatureVersion || r.CaseFeatures != decision.DeclaredCaseFeatureVersion {
			t.Fatal("initial binding", r, err)
		}
		if samples[i].Acceptable>>r.Proposed&1 == 0 {
			t.Fatal("declared requirement did not select its learned path", i, r.Proposed)
		}
		r.SHA = ""
		raw, _ := json.Marshal(r)
		if hash(raw) != s.Ranking().SHA {
			t.Fatal("ranking digest")
		}
		r.Logits[0][0] = 99
		if s.Ranking().Logits == r.Logits {
			t.Fatal("borrowed ranking")
		}
		progress, body, err := s.Advance(ctx, 1)
		if err != nil || body == nil || progress.Status != "TRAINING_COMPLETE" || progress.Attempted != 1 || progress.Selection.ModelCalls != 1 || progress.PredictionsThisAdvance != 0 || progress.RankingSHA != s.Ranking().SHA || progress.LatestFeedbackSHA != "" {
			t.Fatal("predicted body was not immediately checked", progress, err)
		}
		for _, row := range []ContractProgress{initial, progress} {
			saved := row.SHA
			row.SHA = ""
			raw, _ := json.Marshal(row)
			if saved != hash(raw) {
				t.Fatal("progress digest")
			}
		}
		for _, c := range cases {
			v, err := body.Evaluate(c.Input)
			if err != nil || v.Int != c.Expected {
				t.Fatal("exact result", v, err)
			}
		}
	}
	if rankings[0].PlanSHA != rankings[1].PlanSHA || rankings[0].FeatureSHA != rankings[1].FeatureSHA || rankings[0].CaseSHA == rankings[1].CaseSHA || rankings[0].Proposed == rankings[1].Proposed {
		t.Fatal("source/case channels did not separate")
	}
}

func TestContractSessionFallbackAndWrongModelKeepCompleteFiniteChecks(t *testing.T) {
	ctx := sessionContext(t)
	p, err := Prepare(executionMaxPlan())
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{-9007199254740995, 10}, {9007199254740993, 9007199254740993}}
	s, err := p.NewContractSession(ctx, nil, cases)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := p.NewSession(ctx, nil, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	for range 4 {
		a, ab, ae := s.Advance(ctx, 1)
		b, bb, be := baseline.Advance(ctx, 1)
		if !reflect.DeepEqual(a.NewAttempts, b.NewAttempts) || a.Status != b.Status || a.Attempted != b.Attempted || (ae == nil) != (be == nil) || (ab == nil) != (bb == nil) {
			t.Fatal("fallback order differs", ae, be)
		}
		if a.Status == "TRAINING_COMPLETE" {
			break
		}
	}
	if s.Ranking().Calls != 0 || s.Ranking().Applied {
		t.Fatal("nil model inferred")
	}
	model, _ := contractdecision.New([contractdecision.ParameterCount]float32{})
	s, err = p.NewContractSession(ctx, model, cases)
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := s.Advance(ctx, 1)
	if err != nil || first.NewAttempts[0].Mask != 0 || first.SelectedPassed == len(cases) || first.Status == "TRAINING_COMPLETE" {
		t.Fatal("wrong model bypassed cases", first, err)
	}
	last, body, err := s.Advance(ctx, 3)
	if err != nil || body == nil || last.Status != "TRAINING_COMPLETE" || last.Selection.ModelCalls != 1 {
		t.Fatal("finite continuation", last, err)
	}
	// All output values can match while the authored condition still rejects.
	wrongConditions := executionMaxPlan()
	wrongConditions.ConditionCases[0].Expected = false
	p, _ = Prepare(wrongConditions)
	s, err = p.NewContractSession(ctx, model, []TestCase{{-9007199254740995, -9007199254740995}, {9007199254740993, 10}})
	if err != nil {
		t.Fatal(err)
	}
	first, body, err = s.Advance(ctx, 1)
	if !errors.Is(err, ErrNoConditionCandidate) || body != nil || first.ConditionRejected != 1 || first.NewAttempts[0].Passed != 2 {
		t.Fatal("condition bypass", first, err)
	}
}

func TestContractSessionBoundsAndNonBlockingOperations(t *testing.T) {
	ctx := sessionContext(t)
	p, _ := Prepare(executionMaxPlan())
	model, _ := contractdecision.New([contractdecision.ParameterCount]float32{})
	for _, cases := range [][]TestCase{nil, make([]TestCase, 129)} {
		if s, err := p.NewContractSession(ctx, model, cases); err == nil || s != nil {
			t.Fatal("case bound")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if s, err := p.NewContractSession(cancelled, model, []TestCase{{0, 10}}); !errors.Is(err, context.Canceled) || s != nil {
		t.Fatal(err)
	}
	s, err := p.NewContractSession(ctx, model, []TestCase{{0, 10}})
	if err != nil {
		t.Fatal(err)
	}
	s.core.lock.Lock()
	if _, _, err := s.Advance(ctx, 1); !errors.Is(err, ErrSessionBusy) {
		t.Fatal(err)
	}
	if _, err := s.Observe(); !errors.Is(err, ErrSessionBusy) {
		t.Fatal(err)
	}
	s.core.lock.Unlock()
	var missing *ContractSession
	if _, _, err := missing.Advance(ctx, 1); err == nil {
		t.Fatal("nil session")
	}
	if _, err := missing.Observe(); err == nil {
		t.Fatal("nil observe")
	}
}

func TestContractSessionChecksContradictoryTailCase(t *testing.T) {
	ctx := sessionContext(t)
	p, err := Prepare(pairedContractPlan())
	if err != nil {
		t.Fatal(err)
	}
	cases := make([]TestCase, 128)
	for i := range cases {
		cases[i] = TestCase{Input: 0, Expected: 10}
	}
	// Both paths fail at least one requirement; dropping the last case would
	// incorrectly let the maximum path pass the complete contract.
	cases[127].Expected = 0
	model, _ := contractdecision.New([contractdecision.ParameterCount]float32{})
	s, err := p.NewContractSession(ctx, model, cases)
	if err != nil {
		t.Fatal(err)
	}
	clear(cases) // The session owns its complete declared suite.
	progress, _, err := s.Advance(ctx, 2)
	if err != nil || progress.Status == "TRAINING_COMPLETE" || !progress.Exhausted || progress.Cases != 128 || progress.SelectedPassed != 127 || progress.Selection.ModelCalls != 1 {
		t.Fatal("contradictory tail lost", progress, err)
	}
	for _, attempt := range progress.NewAttempts {
		if len(attempt.Results) != 128 {
			t.Fatal("case truncation")
		}
	}
}
