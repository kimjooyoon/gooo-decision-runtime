package pathplan

import (
	"context"
	"encoding/json"
	"errors"
)

// ProbeSnapshot separates evaluations performed now from previously observed
// outputs reused now. COMPLETE still describes a finite candidate enumeration.
type ProbeSnapshot struct {
	Schema                    string       `json:"schema"`
	Revision                  int          `json:"revision"`
	InitialRankingSHA256      string       `json:"initial_ranking_sha256"`
	Ranking                   ProbeRanking `json:"ranking"`
	ReusedProbeValues         int          `json:"reused_probe_values"`
	CachedComparisons         int          `json:"cached_comparisons"`
	TotalEvaluationAttempts   int          `json:"total_evaluation_attempts"`
	TotalConditionEvaluations int          `json:"total_condition_evaluations,omitempty"`
	TotalCachedComparisons    int          `json:"total_cached_comparisons"`
}

// ProbeSession retains one bounded observation matrix for a prepared plan.
// Call its methods serially. Separate sessions may share the immutable plan.
// Snapshots and input slices are caller-owned; changing them cannot edit a session.
// Added expected values come from the caller's oracle, never from this cache.
type ProbeSession struct {
	prepared             *PreparedPlan
	cases                [128]TestCase
	caseCount            int
	probes               [32]int64
	probeCount           int
	masks                [64]uint16
	count                int
	outputs              [64][32]int64
	declared             int
	observed             int
	typeRejected         int
	caseRejected         int
	conditionRejected    int
	conditionEvaluations int
	evaluationAttempts   int
	cachedComparisons    int
	revision             int
	initialRankingSHA256 string
}

// StartProbeSession performs the same initial work as RankProbes and retains
// only the surviving masks and their supplied probe outputs. An interrupted
// initial observation returns its receipt, but no reusable session.
func (prepared *PreparedPlan) StartProbeSession(ctx context.Context, cases []TestCase,
	probes []int64, maxCandidates int) (*ProbeSession, ProbeSnapshot, error) {
	ranking, err := prepared.RankProbes(ctx, cases, probes, maxCandidates)
	initial := ProbeSnapshot{Schema: "gooo/typed-path-probe-snapshot/v1", Ranking: ranking,
		TotalEvaluationAttempts: ranking.EvaluationAttempts, TotalConditionEvaluations: ranking.ConditionEvaluations}
	if err != nil {
		return nil, initial, err
	}
	raw, _ := json.Marshal(ranking)
	initial.InitialRankingSHA256 = hashBytes(raw)
	s := &ProbeSession{prepared: prepared, caseCount: len(cases), probeCount: len(probes),
		count: len(ranking.SurvivingMasks), declared: ranking.Declared, observed: ranking.Observed,
		typeRejected: ranking.TypeRejected, caseRejected: ranking.CaseRejected,
		conditionRejected: ranking.ConditionRejected, conditionEvaluations: ranking.ConditionEvaluations,
		evaluationAttempts: ranking.EvaluationAttempts, initialRankingSHA256: initial.InitialRankingSHA256}
	copy(s.cases[:], cases)
	copy(s.probes[:], probes)
	copy(s.masks[:], ranking.SurvivingMasks)
	for j, probe := range ranking.Probes {
		for i, value := range probe.Outputs {
			s.outputs[i][j] = value
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, initial, err
	}
	return s, initial, nil
}

func (s *ProbeSession) check(ctx context.Context) error {
	if ctx == nil {
		return errors.New("probe session requires a context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("probe session requires a deadline")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.prepared == nil || s.caseCount == 0 {
		return errors.New("initialized probe session is required")
	}
	return nil
}

// Snapshot copies the current finite evidence without invoking the evaluator.
func (s *ProbeSession) Snapshot(ctx context.Context) (ProbeSnapshot, error) {
	if err := s.check(ctx); err != nil {
		return ProbeSnapshot{}, err
	}
	r := s.snapshot(0)
	return r, ctx.Err()
}

// AppendObservation filters cached outputs for an already observed probe input.
// Previously declared inputs (including conflicting labels), unobserved inputs
// and a full 128-case suite are rejected without changing the session. No plan
// compilation, evaluator call, model call or external request is performed.
// Cancellation before the final commit leaves the previous revision intact.
func (s *ProbeSession) AppendObservation(ctx context.Context, observation TestCase) (ProbeSnapshot, error) {
	if err := s.check(ctx); err != nil {
		return ProbeSnapshot{}, err
	}
	if s.caseCount == len(s.cases) {
		return ProbeSnapshot{}, errors.New("probe session case budget exhausted")
	}
	for _, c := range s.cases[:s.caseCount] {
		if c.Input == observation.Input {
			return ProbeSnapshot{}, errors.New("probe input already belongs to the finite suite")
		}
	}
	column := -1
	for j, input := range s.probes[:s.probeCount] {
		if input == observation.Input {
			column = j
			break
		}
	}
	if column < 0 {
		return ProbeSnapshot{}, errors.New("input has no cached probe observation")
	}
	// Copy the bounded state before filtering so failure cannot partly mutate it.
	next := *s
	next.count = 0
	for i := range s.count {
		if s.outputs[i][column] == observation.Expected {
			next.masks[next.count], next.outputs[next.count] = s.masks[i], s.outputs[i]
			next.count++
		}
	}
	next.caseRejected += s.count - next.count
	next.cachedComparisons += s.count
	next.cases[next.caseCount], next.caseCount = observation, next.caseCount+1
	next.revision++
	r := next.snapshot(s.count)
	if err := ctx.Err(); err != nil {
		return ProbeSnapshot{}, err
	}
	*s = next
	return r, nil
}

func (s *ProbeSession) snapshot(comparisons int) ProbeSnapshot {
	// Marshal small owned slices instead of slices into the complete session.
	// Otherwise encoding/json makes the 16 KiB matrix escape with the case array.
	cases := append([]TestCase(nil), s.cases[:s.caseCount]...)
	probes := append([]int64(nil), s.probes[:s.probeCount]...)
	caseRaw, _ := json.Marshal(cases)
	probeRaw, _ := json.Marshal(probes)
	r := ProbeRanking{Schema: "gooo/typed-path-probe-ranking/v1", Status: "PARTIAL",
		PlanSHA256: s.prepared.sha, CasesSHA256: hashBytes(caseRaw), ProbesSHA256: hashBytes(probeRaw),
		Declared: s.declared, Observed: s.observed, Unobserved: s.declared - s.observed,
		TypeRejected: s.typeRejected, CaseRejected: s.caseRejected,
		ConditionRejected: s.conditionRejected, ReusedConditionEvaluations: s.conditionEvaluations,
		OutputStorageBytes: 64 * 32 * 8}
	if r.Unobserved == 0 {
		r.Status = "COMPLETE"
	}
	if s.count > 0 {
		r.SurvivingMasks = append([]uint16(nil), s.masks[:s.count]...)
	}
	r.rankPartitions(cases, probes, &s.outputs)
	return ProbeSnapshot{Schema: "gooo/typed-path-probe-snapshot/v1", Revision: s.revision,
		InitialRankingSHA256: s.initialRankingSHA256, Ranking: r,
		ReusedProbeValues: s.count * s.probeCount, CachedComparisons: comparisons,
		TotalEvaluationAttempts: s.evaluationAttempts, TotalConditionEvaluations: s.conditionEvaluations, TotalCachedComparisons: s.cachedComparisons}
}
