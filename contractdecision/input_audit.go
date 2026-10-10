package contractdecision

import (
	"context"
	"errors"
	"fmt"
)

// InputGroup describes samples whose complete ordered model inputs and
// candidate masks are bit-identical. Indices refer to the supplied samples.
type InputGroup struct {
	InputSHA256       string   `json:"input_sha256"`
	SampleIndices     []int    `json:"sample_indices"`
	CandidateMasks    []uint16 `json:"candidate_masks"`
	CommonAcceptable  uint64   `json:"common_acceptable_candidate_bits"`
	BestFirstPasses   int      `json:"best_possible_first_choice_passes"`
	UnavoidableMisses int      `json:"unavoidable_first_choice_misses"`
}

// InputAudit bounds first-choice validity on the supplied labelled rows for a
// deterministic decision rule using these inputs. It is not measured accuracy,
// an attainable neural-model score, or eventual finite-search completeness.
type InputAudit struct {
	Schema            string       `json:"schema"`
	Samples           int          `json:"samples"`
	RepeatedGroups    int          `json:"repeated_input_groups"`
	ConflictingGroups int          `json:"conflicting_input_groups"`
	BestFirstPasses   int          `json:"best_possible_first_choice_passes"`
	UnavoidableMisses int          `json:"unavoidable_first_choice_misses"`
	Groups            []InputGroup `json:"groups"`
}

// AuditInputs finds requirements that cannot all receive a valid first choice
// from identical model inputs. It neither trains nor predicts, executes no
// candidates, and does not reject valid but conflicting training labels.
// Samples and case readers must remain immutable throughout the call. Readers
// may be revisited to compare matching hashes byte for byte; no dataset-sized
// case tensor is retained. The owning compiler supplies the acceptable sets.
func AuditInputs(ctx context.Context, samples []Sample) (*InputAudit, error) {
	return auditInputs(ctx, samples, nil)
}

func auditInputs(ctx context.Context, samples []Sample, conditions []ConditionSource) (*InputAudit, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 {
		return nil, errors.New("input audit requires a context and 1..4096 labelled samples")
	}
	report := &InputAudit{Schema: "gooo/decision-input-audit/v1", Samples: len(samples)}
	if conditions != nil {
		if len(conditions) != len(samples) {
			return nil, errors.New("condition audit sources must match sample count")
		}
		report.Schema = RequirementInputAuditSchema
	}
	buckets := make(map[[32]byte][]int)
	var counts [][MaxCandidates]int
	for index, sample := range samples {
		key, err := auditInputDigest(ctx, sample)
		if err != nil {
			return nil, fmt.Errorf("sample %d: %w", index, err)
		}
		if conditions != nil {
			key, err = requirementAuditDigest(ctx, key, conditions[index])
			if err != nil {
				return nil, fmt.Errorf("sample %d conditions: %w", index, err)
			}
		}
		groupIndex := -1
		for _, candidate := range buckets[key] {
			first := report.Groups[candidate].SampleIndices[0]
			same, err := sameAuditInput(ctx, samples[first], sample)
			if err != nil {
				return nil, fmt.Errorf("sample %d comparison: %w", index, err)
			}
			if same && conditions != nil {
				same, err = sameRequirementConditions(ctx, conditions[first], conditions[index])
				if err != nil {
					return nil, fmt.Errorf("sample %d condition comparison: %w", index, err)
				}
			}
			if same {
				groupIndex = candidate
				break
			}
		}
		if groupIndex < 0 {
			groupIndex = len(report.Groups)
			buckets[key] = append(buckets[key], groupIndex)
			report.Groups = append(report.Groups, InputGroup{
				InputSHA256: fmt.Sprintf("%x", key), CandidateMasks: append([]uint16(nil), sample.Masks...),
				CommonAcceptable: allCandidates(len(sample.Masks)),
			})
			counts = append(counts, [MaxCandidates]int{})
		}
		group := &report.Groups[groupIndex]
		group.SampleIndices = append(group.SampleIndices, index)
		group.CommonAcceptable &= sample.Acceptable
		for candidate := range sample.Masks {
			if sample.Acceptable>>candidate&1 != 0 {
				counts[groupIndex][candidate]++
				group.BestFirstPasses = max(group.BestFirstPasses, counts[groupIndex][candidate])
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for i := range report.Groups {
		group := &report.Groups[i]
		group.UnavoidableMisses = len(group.SampleIndices) - group.BestFirstPasses
		report.BestFirstPasses += group.BestFirstPasses
		report.UnavoidableMisses += group.UnavoidableMisses
		if len(group.SampleIndices) > 1 {
			report.RepeatedGroups++
		}
		if group.CommonAcceptable == 0 {
			report.ConflictingGroups++
		}
	}
	return report, nil
}
