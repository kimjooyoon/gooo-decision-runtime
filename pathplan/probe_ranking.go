package pathplan

import (
	"context"
	"encoding/json"
)

// ProbePartition records outputs of surviving declared candidates on one input.
// Outputs align with SurvivingMasks. They are observations, not expected answers.
type ProbePartition struct {
	Input          int64   `json:"input"`
	AlreadyTested  bool    `json:"already_tested"`
	Outputs        []int64 `json:"candidate_outputs"`
	Distinct       int     `json:"distinct_outputs"`
	SeparatedPairs int     `json:"separated_pairs"`
	LargestGroup   int     `json:"largest_output_group"`
}

// ProbeRanking recommends where an existing oracle could provide useful evidence.
// COMPLETE means every declared candidate was observed, not semantic completeness.
type CandidateConditions struct {
	Mask    uint16            `json:"choice_mask"`
	Results []ConditionResult `json:"results"`
}

type ProbeRanking struct {
	Schema                     string                `json:"schema"`
	Status                     string                `json:"status"`
	PlanSHA256                 string                `json:"plan_sha256"`
	CasesSHA256                string                `json:"cases_sha256"`
	ProbesSHA256               string                `json:"probe_inputs_sha256"`
	Declared                   int                   `json:"declared_combinations"`
	Observed                   int                   `json:"observed_combinations"`
	Unobserved                 int                   `json:"unobserved_combinations"`
	TypeRejected               int                   `json:"type_rejected_candidates"`
	CaseRejected               int                   `json:"case_rejected_candidates"`
	SurvivingMasks             []uint16              `json:"surviving_masks"`
	CandidatePairs             int                   `json:"candidate_pairs"`
	Probes                     []ProbePartition      `json:"probes"`
	RecommendedIndex           *int                  `json:"recommended_probe_index"`
	ModelPredictions           int                   `json:"model_predictions"`
	EvaluationAttempts         int                   `json:"evaluation_attempts"`
	OutputStorageBytes         int                   `json:"output_matrix_bytes"`
	ConditionRejected          int                   `json:"condition_rejected_candidates,omitempty"`
	ConditionEvaluations       int                   `json:"condition_evaluations,omitempty"`
	ReusedConditionEvaluations int                   `json:"reused_condition_evaluations,omitempty"`
	ConditionObservations      []CandidateConditions `json:"condition_observations,omitempty"`
}

// RankProbes evaluates at most 64 ascending choice masks against up to 128
// supplied cases. For survivors it evaluates up to 32 probe inputs and ranks
// them by differing unordered candidate pairs, then the smaller largest output
// group, then caller input order. No probabilities or extra training are used.
//
// A deadline is required. The prepared source and caller's cases are unchanged.
// Caller-owned slices are copied after bounds checks; synchronize while calling.
// The fixed output matrix is 16 KiB, separate from compiled programs/receipts.
// On interruption, only fully observed candidates enter the result, and no
// recommendation is emitted. A limited candidate budget is explicitly PARTIAL.
func (prepared *PreparedPlan) RankProbes(ctx context.Context, cases []TestCase, probes []int64, maxCandidates int) (ProbeRanking, error) {
	if err := diagnosisBounds(ctx, prepared, cases, probes, maxCandidates); err != nil {
		return ProbeRanking{}, err
	}
	cases, probes = append([]TestCase(nil), cases...), append([]int64(nil), probes...)
	caseRaw, _ := json.Marshal(cases)
	probeRaw, _ := json.Marshal(probes)
	r := ProbeRanking{Schema: "gooo/typed-path-probe-ranking/v1", Status: "PARTIAL",
		PlanSHA256: prepared.sha, CasesSHA256: hashBytes(caseRaw), ProbesSHA256: hashBytes(probeRaw),
		Declared: 1 << len(prepared.plan.Decisions), OutputStorageBytes: 64 * 32 * 8}
	r.Unobserved = r.Declared
	var outputs [64][32]int64
	for mask := 0; mask < r.Declared && r.Observed < maxCandidates; mask++ {
		if err := ctx.Err(); err != nil {
			r.Status = "INTERRUPTED"
			return r, err
		}
		program, compileErr := prepared.Compile(diagnosisChoices(prepared.plan, uint16(mask)))
		if err := ctx.Err(); err != nil {
			r.Status = "INTERRUPTED"
			return r, err
		}
		if compileErr != nil {
			r.TypeRejected++
		} else {
			conditions, err := prepared.checkProgramConditions(ctx, program)
			if err != nil {
				r.Status = "INTERRUPTED"
				return r, err
			}
			if len(conditions) > 0 {
				r.ConditionObservations = append(r.ConditionObservations, CandidateConditions{Mask: uint16(mask), Results: conditions})
				r.ConditionEvaluations += len(conditions)
			}
			conditionMatch := ConditionsPassed(conditions)
			matches := true
			for _, test := range cases {
				r.EvaluationAttempts++
				value, err := diagnosisValue(ctx, program, test.Input)
				if err != nil {
					r.Status = "INTERRUPTED"
					return r, err
				}
				matches = matches && value == test.Expected
			}
			if !conditionMatch {
				r.ConditionRejected++
			}
			if !matches {
				r.CaseRejected++
			}
			if matches && conditionMatch {
				row := len(r.SurvivingMasks)
				for j, input := range probes {
					r.EvaluationAttempts++
					value, err := diagnosisValue(ctx, program, input)
					if err != nil {
						r.Status = "INTERRUPTED"
						return r, err
					}
					outputs[row][j] = value
				}
				r.SurvivingMasks = append(r.SurvivingMasks, uint16(mask))
			}
		}
		r.Observed++
		r.Unobserved--
	}
	if err := ctx.Err(); err != nil {
		r.Status = "INTERRUPTED"
		return r, err
	}
	if r.Unobserved == 0 {
		r.Status = "COMPLETE"
	}
	r.rankPartitions(cases, probes, &outputs)
	if err := ctx.Err(); err != nil {
		r.Status, r.RecommendedIndex = "INTERRUPTED", nil
		return r, err
	}
	return r, nil
}

func (r *ProbeRanking) rankPartitions(cases []TestCase, probes []int64, matrix *[64][32]int64) {
	n := len(r.SurvivingMasks)
	r.CandidatePairs = n * (n - 1) / 2
	best := -1
	for j, input := range probes {
		p := ProbePartition{Input: input, Outputs: make([]int64, n)}
		for _, test := range cases {
			p.AlreadyTested = p.AlreadyTested || input == test.Input
		}
		for i := 0; i < n; i++ {
			p.Outputs[i] = matrix[i][j]
			count, first := 1, true
			for k := 0; k < n; k++ {
				if k != i && matrix[k][j] == matrix[i][j] {
					count++
					first = first && k > i
				}
				if k < i && matrix[k][j] != matrix[i][j] {
					p.SeparatedPairs++
				}
			}
			if first {
				p.Distinct++
			}
			p.LargestGroup = max(p.LargestGroup, count)
		}
		r.Probes = append(r.Probes, p)
		if p.AlreadyTested || p.SeparatedPairs == 0 {
			continue
		}
		if best < 0 || p.SeparatedPairs > r.Probes[best].SeparatedPairs ||
			(p.SeparatedPairs == r.Probes[best].SeparatedPairs && p.LargestGroup < r.Probes[best].LargestGroup) {
			best = j
		}
	}
	if best >= 0 {
		r.RecommendedIndex = &best
	}
}
