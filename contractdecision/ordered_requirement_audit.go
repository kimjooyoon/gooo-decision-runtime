package contractdecision

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const OrderedRequirementInputAuditSchema = "gooo/ordered-requirement-input-audit/v1"

func orderedAuditBase(sample OrderedRequirementSample) Sample {
	base := Sample{Inputs: make([][FeatureDim]float32, len(sample.Inputs)), Cases: sample.Cases, Masks: sample.Masks, Acceptable: sample.Acceptable}
	for i, row := range sample.Inputs {
		copy(base.Inputs[i][:], row[:FeatureDim])
	}
	return base
}

func orderedAuditKey(ctx context.Context, sample OrderedRequirementSample) ([32]byte, error) {
	if err := validateOrderedRequirements(sample.Inputs, sample.Cases, sample.Conditions, sample.Masks); err != nil {
		return [32]byte{}, err
	}
	base, err := auditInputDigest(ctx, orderedAuditBase(sample))
	if err != nil {
		return base, err
	}
	base, err = requirementAuditDigest(ctx, base, sample.Conditions)
	if err != nil {
		return base, err
	}
	h := sha256.New()
	h.Write([]byte(OrderedRequirementInputAuditSchema + "\x00" + OrderedSourceFeatureVersion + "\x00"))
	h.Write(base[:])
	var raw [(OrderedFeatureDim - FeatureDim) * 4]byte
	for _, source := range sample.Inputs {
		for i, value := range source[FeatureDim:] {
			binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(value))
		}
		h.Write(raw[:])
	}
	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result, nil
}

func sameOrderedAudit(ctx context.Context, a, b OrderedRequirementSample) (bool, error) {
	if len(a.Inputs) != len(b.Inputs) {
		return false, nil
	}
	for i := range a.Inputs {
		if !sameAuditCells(a.Inputs[i][:], b.Inputs[i][:]) {
			return false, nil
		}
	}
	same, err := sameAuditInput(ctx, orderedAuditBase(a), orderedAuditBase(b))
	if err != nil || !same {
		return same, err
	}
	return sameRequirementConditions(ctx, a.Conditions, b.Conditions)
}

// AuditOrderedRequirements includes all528 source cells, every output and
// condition row, and candidate order. Its bound measures input distinguishability;
// learning and eventual checked completion are separate observations.
func AuditOrderedRequirements(ctx context.Context, samples []OrderedRequirementSample) (*InputAudit, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 {
		return nil, errors.New("ordered audit requires context and 1..4096 samples")
	}
	report := &InputAudit{Schema: OrderedRequirementInputAuditSchema, Samples: len(samples)}
	buckets := make(map[[32]byte][]int)
	var counts [][MaxCandidates]int
	for index, sample := range samples {
		key, err := orderedAuditKey(ctx, sample)
		if err != nil {
			return nil, fmt.Errorf("sample %d: %w", index, err)
		}
		groupIndex := -1
		for _, candidate := range buckets[key] {
			first := report.Groups[candidate].SampleIndices[0]
			same, err := sameOrderedAudit(ctx, samples[first], sample)
			if err != nil {
				return nil, err
			}
			if same {
				groupIndex = candidate
				break
			}
		}
		if groupIndex < 0 {
			groupIndex = len(report.Groups)
			buckets[key] = append(buckets[key], groupIndex)
			report.Groups = append(report.Groups, InputGroup{InputSHA256: fmt.Sprintf("%x", key),
				CandidateMasks: append([]uint16(nil), sample.Masks...), CommonAcceptable: allCandidates(len(sample.Masks))})
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
