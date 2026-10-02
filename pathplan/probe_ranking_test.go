package pathplan

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestRankProbesSeparatesOrderAndConsumesAnOracleObservation(t *testing.T) {
	p, err := Prepare(probePlan("subtract"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cases := []TestCase{{Input: 2, Expected: 0}}
	probes := []int64{2, 3, 0, 3}
	r, err := p.RankProbes(ctx, cases, probes, 2)
	if err != nil || r.Status != "COMPLETE" || r.Observed != 2 || r.Unobserved != 0 ||
		r.CandidatePairs != 1 || r.RecommendedIndex == nil || *r.RecommendedIndex != 1 ||
		r.ModelPredictions != 0 || r.EvaluationAttempts != 10 || r.OutputStorageBytes != 16384 {
		t.Fatalf("ranking: %+v, %v", r, err)
	}
	if !r.Probes[0].AlreadyTested || r.Probes[0].SeparatedPairs != 0 ||
		!reflect.DeepEqual(r.Probes[1].Outputs, []int64{1, -1}) || r.Probes[1].Distinct != 2 {
		t.Fatal("observed partitions differ")
	}
	// The independently declared specification is 2-input. Candidate outputs
	// above do not decide the expected value.
	x := r.Probes[*r.RecommendedIndex].Input
	cases = append(cases, TestCase{Input: x, Expected: 2 - x})
	after, err := p.RankProbes(ctx, cases, probes, 2)
	if err != nil || after.CaseRejected != 1 || !reflect.DeepEqual(after.SurvivingMasks, []uint16{1}) ||
		after.RecommendedIndex != nil || after.CandidatePairs != 0 {
		t.Fatalf("oracle continuation: %+v %v", after, err)
	}
}

func TestRankProbesUnresolvedContradictoryAndPartial(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	p, _ := Prepare(probePlan("add"))
	r, err := p.RankProbes(ctx, []TestCase{{Input: 2, Expected: 4}}, []int64{0, 3}, 2)
	if err != nil || len(r.SurvivingMasks) != 2 || r.RecommendedIndex != nil || r.Probes[0].LargestGroup != 2 {
		t.Fatal("finite agreement became certainty", r, err)
	}
	r, err = p.RankProbes(ctx, []TestCase{{Input: 2, Expected: 4}, {Input: 2, Expected: 0}}, []int64{3}, 2)
	if err != nil || len(r.SurvivingMasks) != 0 || r.CaseRejected != 2 || r.RecommendedIndex != nil {
		t.Fatal("contradictory expectations were discarded", r, err)
	}
	r, err = p.RankProbes(ctx, []TestCase{{Input: 2, Expected: 4}}, []int64{3}, 1)
	if err != nil || r.Status != "PARTIAL" || r.Unobserved != 1 || r.RecommendedIndex != nil {
		t.Fatal("budget exhaustion hid unobserved candidates", r, err)
	}
	p, _ = Prepare(interactingPlan())
	r, err = p.RankProbes(ctx, []TestCase{{Input: 3, Expected: 16}}, []int64{2, 3}, 4)
	if err != nil || r.TypeRejected != 1 || r.Observed != 4 {
		t.Fatal("combined type rejection missing", r, err)
	}
}

func TestProbePartitionsCountPairsIndependently(t *testing.T) {
	// Three synthetic probe columns: 3+1, 2+2 and 1+1+1+1 partitions.
	var matrix [64][32]int64
	columns := [3][4]int64{{0, 0, 0, 1}, {0, 0, 1, 1}, {0, 1, 2, 3}}
	for j, col := range columns {
		for i, v := range col {
			matrix[i][j] = v
		}
	}
	r := ProbeRanking{SurvivingMasks: []uint16{0, 1, 2, 3}}
	r.rankPartitions(nil, []int64{10, 11, 12}, &matrix)
	if r.CandidatePairs != 6 || r.RecommendedIndex == nil || *r.RecommendedIndex != 2 {
		t.Fatal(r)
	}
	for j, want := range []int{3, 4, 6} {
		if r.Probes[j].SeparatedPairs != want {
			t.Fatal(r.Probes)
		}
	}
	// Exhaustively compare all 3^4 small output vectors to a pairwise oracle.
	for encoded := 0; encoded < 81; encoded++ {
		v := encoded
		for i := 0; i < 4; i++ {
			matrix[i][0] = int64(v % 3)
			v /= 3
		}
		r := ProbeRanking{SurvivingMasks: []uint16{0, 1, 2, 3}}
		r.rankPartitions(nil, []int64{10}, &matrix)
		want := 0
		for a := 0; a < 4; a++ {
			for b := a + 1; b < 4; b++ {
				if matrix[a][0] != matrix[b][0] {
					want++
				}
			}
		}
		if r.Probes[0].SeparatedPairs != want {
			t.Fatal(encoded, r)
		}
	}
}

func TestRankProbesRejectsUnboundedWorkAndOwnsConcurrentResults(t *testing.T) {
	p, _ := Prepare(probePlan("subtract"))
	cases, probes := []TestCase{{Input: 2, Expected: 0}}, []int64{3}
	if _, err := p.RankProbes(context.Background(), cases, probes, 2); err == nil {
		t.Fatal("missing deadline")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, budget := range []int{0, 65} {
		if _, err := p.RankProbes(ctx, cases, probes, budget); err == nil {
			t.Fatal("invalid budget")
		}
	}
	if _, err := p.RankProbes(ctx, cases, make([]int64, 33), 2); err == nil {
		t.Fatal("probe bound")
	}
	want, _ := p.RankProbes(ctx, cases, probes, 2)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			got, err := p.RankProbes(ctx, cases, probes, 2)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Error(got, err)
				return
			}
			got.Probes[0].Outputs[0] = 999
			got.SurvivingMasks[0] = 999
		})
	}
	wg.Wait()
	got, err := p.RankProbes(ctx, cases, probes, 2)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("shared state mutated")
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if r, err := p.RankProbes(canceled, cases, probes, 2); err == nil || r.Observed != 0 || r.RecommendedIndex != nil {
		t.Fatal(r, err)
	}
}
