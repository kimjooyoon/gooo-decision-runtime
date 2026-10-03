package pathplan

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

func branchProbePlan() Plan {
	p := Plan{Schema: Schema, Base: bodyplan.Plan{Schema: bodyplan.Schema, ID: "probe-session", Name: "Probe",
		ResultType: decision.TypeInt, Expressions: []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 0},
			{Kind: "binary", Operation: "less_than", Left: 0, Right: 1},
			{Kind: "binary", Operation: "subtract", Left: 0, Right: 1},
			{Kind: "binary", Operation: "subtract", Left: 0, Right: 1}},
		Statements: []bodyplan.Stmt{{Kind: "return", Expr: 3}, {Kind: "return", Expr: 4},
			{Kind: "if", Expr: 2, Then: []int{0}, Else: []int{1}}}, Root: []int{2}}}
	for i, id := range []string{"negative", "positive"} {
		p.Decisions = append(p.Decisions, Choice{ID: id, Kind: OperandOrder, Target: 3 + i,
			Intent: "Choose an operand order.", Fallback: "layout_forward",
			Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}})
	}
	return p
}

func TestProbeSessionMatchesFreshRankingAcrossTwoObservations(t *testing.T) {
	p, err := Prepare(branchProbePlan())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cases, inputs := []TestCase{{0, 0}}, []int64{-1, 1, 0}
	s, initial, err := p.StartProbeSession(ctx, cases, inputs, 4)
	if err != nil {
		t.Fatal(err)
	}
	if initial.Ranking.EvaluationAttempts != 16 || initial.TotalEvaluationAttempts != 16 || initial.ReusedProbeValues != 0 {
		t.Fatal("initial evaluations missing", initial)
	}
	for i, observation := range []TestCase{{-1, 1}, {1, 1}} {
		got, err := s.AppendObservation(ctx, observation)
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, observation)
		fresh, err := p.RankProbes(ctx, cases, inputs, 4)
		if err != nil {
			t.Fatal(err)
		}
		if fresh.EvaluationAttempts == 0 {
			t.Fatal("baseline did not evaluate")
		}
		fresh.EvaluationAttempts = 0 // New work differs; all finite semantics must match.
		if !reflect.DeepEqual(got.Ranking, fresh) || got.Revision != i+1 || got.InitialRankingSHA256 != initial.InitialRankingSHA256 ||
			got.TotalEvaluationAttempts != 16 || got.Ranking.ModelPredictions != 0 ||
			got.CachedComparisons != []int{4, 2}[i] || got.TotalCachedComparisons != []int{4, 6}[i] ||
			got.ReusedProbeValues != []int{6, 3}[i] {
			t.Fatal("cached filtering differs from fresh evaluation", got, fresh)
		}
	}
}

func TestProbeSessionOwnsInputsAndSnapshots(t *testing.T) {
	p, _ := Prepare(probePlan("subtract"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cases, inputs := []TestCase{{2, 0}}, []int64{3, 0}
	s, initial, err := p.StartProbeSession(ctx, cases, inputs, 2)
	if err != nil {
		t.Fatal(err)
	}
	initial.Ranking.SurvivingMasks[0] = 99
	initial.Ranking.Probes[0].Outputs[0] = 999
	*initial.Ranking.RecommendedIndex = 1
	cases[0].Expected, inputs[0] = 999, 999
	got, err := s.AppendObservation(ctx, TestCase{3, -1})
	if err != nil || !reflect.DeepEqual(got.Ranking.SurvivingMasks, []uint16{1}) || got.Ranking.Probes[0].Input != 3 {
		t.Fatal("caller mutation reached session", got, err)
	}
	got.Ranking.Probes[0].Outputs[0] = 999
	got.Ranking.SurvivingMasks[0] = 99
	again, err := s.Snapshot(ctx)
	if err != nil || again.Ranking.Probes[0].Outputs[0] != -1 || again.Ranking.SurvivingMasks[0] != 1 ||
		again.CachedComparisons != 0 || again.Ranking.EvaluationAttempts != 0 {
		t.Fatal("snapshot is not owned", again, err)
	}
}

func TestProbeSessionFailuresLeaveStateUnchanged(t *testing.T) {
	p, _ := Prepare(probePlan("subtract"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s, _, _ := p.StartProbeSession(ctx, []TestCase{{2, 0}}, []int64{3, 0}, 2)
	before, _ := s.Snapshot(ctx)
	for _, c := range []TestCase{{2, 0}, {2, 99}, {7, 0}} {
		if _, err := s.AppendObservation(ctx, c); err == nil {
			t.Fatal("invalid addition accepted", c)
		}
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	for _, invalid := range []context.Context{nil, context.Background(), canceled} {
		if _, err := s.AppendObservation(invalid, TestCase{3, -1}); err == nil {
			t.Fatal("invalid context")
		}
		if _, err := s.Snapshot(invalid); err == nil {
			t.Fatal("invalid snapshot context")
		}
	}
	after, _ := s.Snapshot(ctx)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed update changed state")
	}
	var absent *ProbeSession
	if _, err := absent.Snapshot(ctx); err == nil {
		t.Fatal("nil session")
	}
	if _, err := new(ProbeSession).AppendObservation(ctx, TestCase{3, -1}); err == nil {
		t.Fatal("zero session")
	}
	if session, _, err := p.StartProbeSession(canceled, []TestCase{{2, 0}}, []int64{3}, 2); err == nil || session != nil {
		t.Fatal("canceled initial work was retained")
	}
	cases := make([]TestCase, 128)
	for i := range cases {
		cases[i] = TestCase{2, 0}
	}
	full, _, _ := p.StartProbeSession(ctx, cases, []int64{3}, 2)
	if _, err := full.AppendObservation(ctx, TestCase{3, -1}); err == nil || full.caseCount != 128 {
		t.Fatal("case bound")
	}
}

type cancelAtProbeCommit struct {
	context.Context
	reads int
}

func (c *cancelAtProbeCommit) Err() error {
	c.reads++
	if c.reads > 1 {
		return context.Canceled
	}
	return nil
}

func TestProbeSessionCancellationAfterFilteringIsTransactional(t *testing.T) {
	p, _ := Prepare(probePlan("subtract"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s, _, err := p.StartProbeSession(ctx, []TestCase{{2, 0}}, []int64{3}, 2)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := s.Snapshot(ctx)
	if _, err := s.AppendObservation(&cancelAtProbeCommit{Context: ctx}, TestCase{3, -1}); err != context.Canceled {
		t.Fatal("cancellation missed final commit boundary", err)
	}
	after, _ := s.Snapshot(ctx)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("interrupted append committed filtered state")
	}
}

func TestProbeSessionPartialAndEmptyCandidateSetsStayExplicit(t *testing.T) {
	p, _ := Prepare(probePlan("subtract"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, budget := range []int{1, 2} {
		s, _, err := p.StartProbeSession(ctx, []TestCase{{2, 0}}, []int64{3, 0}, budget)
		if err != nil {
			t.Fatal(err)
		}
		got, err := s.AppendObservation(ctx, TestCase{3, 999})
		if err != nil || len(got.Ranking.SurvivingMasks) != 0 || got.Ranking.RecommendedIndex != nil ||
			got.Ranking.Unobserved != 2-budget || got.Ranking.CaseRejected != budget {
			t.Fatal(got, err)
		}
		if budget == 1 && got.Ranking.Status != "PARTIAL" {
			t.Fatal("unobserved candidates disappeared")
		}
		fresh, _ := p.RankProbes(ctx, []TestCase{{2, 0}, {3, 999}}, []int64{3, 0}, budget)
		fresh.EvaluationAttempts = 0
		if !reflect.DeepEqual(got.Ranking, fresh) {
			t.Fatal("empty ranking differs", got.Ranking, fresh)
		}
	}
}

func TestIndependentProbeSessionsSharePreparedPlan(t *testing.T) {
	p, _ := Prepare(probePlan("subtract"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			s, _, err := p.StartProbeSession(ctx, []TestCase{{2, 0}}, []int64{3}, 2)
			if err != nil {
				t.Error(err)
				return
			}
			got, err := s.AppendObservation(ctx, TestCase{3, -1})
			if err != nil || !reflect.DeepEqual(got.Ranking.SurvivingMasks, []uint16{1}) {
				t.Error(got, err)
			}
		})
	}
	wg.Wait()
}

func TestProbeSessionRetainsTypeRejectionsAndRepeatedProbeInputs(t *testing.T) {
	p, err := Prepare(interactingPlan())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cases, inputs := []TestCase{{3, 16}}, []int64{2, 2, 0}
	s, initial, err := p.StartProbeSession(ctx, cases, inputs, 4)
	if err != nil || initial.Ranking.TypeRejected != 1 || len(initial.Ranking.SurvivingMasks) == 0 {
		t.Fatal(initial, err)
	}
	// Any caller-provided expected output is accepted as a finite contract; use a
	// declared candidate here only to test cache parity, not as an intent oracle.
	c := TestCase{2, initial.Ranking.Probes[0].Outputs[0]}
	got, err := s.AppendObservation(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := p.RankProbes(ctx, append(cases, c), inputs, 4)
	if err != nil {
		t.Fatal(err)
	}
	fresh.EvaluationAttempts = 0
	if !reflect.DeepEqual(got.Ranking, fresh) {
		t.Fatal(got.Ranking, fresh)
	}
}

// Kernel-only comparison after the initial observation. The cached arm includes
// copying the bounded session for each iteration; neither arm runs a model.
func BenchmarkProbeObservationContinuation(b *testing.B) {
	p, err := Prepare(branchProbePlan())
	if err != nil {
		b.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cases, inputs := []TestCase{{0, 0}}, []int64{-1, 1, 0}
	s, _, err := p.StartProbeSession(ctx, cases, inputs, 4)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("fresh_rank", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := p.RankProbes(ctx, []TestCase{{0, 0}, {-1, 1}}, inputs, 4); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("cached_append", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			next := *s
			if _, err := next.AppendObservation(ctx, TestCase{-1, 1}); err != nil {
				b.Fatal(err)
			}
		}
	})
}
