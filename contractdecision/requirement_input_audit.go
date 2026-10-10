package contractdecision

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const RequirementInputAuditSchema = "gooo/requirement-input-audit/v1"

// AuditRequirements includes all condition rows alongside the original source,
// output cases and candidate order. The bound applies to a decision rule that
// receives all three channels; AuditInputs keeps its original two-channel ABI.
func AuditRequirements(ctx context.Context, samples []RequirementSample) (*InputAudit, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 {
		return nil, errors.New("requirement audit needs 1..4096 labelled samples")
	}
	base := make([]Sample, len(samples))
	conditions := make([]ConditionSource, len(samples))
	for i, sample := range samples {
		if err := validateRequirements(sample.Inputs, sample.Cases, sample.Conditions, sample.Masks); err != nil {
			return nil, err
		}
		base[i], conditions[i] = sample.Sample, sample.Conditions
	}
	return auditInputs(ctx, base, conditions)
}

func requirementAuditDigest(ctx context.Context, base [32]byte, source ConditionSource) ([32]byte, error) {
	var result [32]byte
	if source == nil || source.ConditionFeatureVersion() != decision.DeclaredConditionFeatureVersion ||
		source.ConditionCount() < 0 || source.ConditionCount() > MaxConditions {
		return result, errors.New("explicit bounded declared condition reader required")
	}
	count := source.ConditionCount()
	if count < 0 || count > MaxConditions {
		return result, errors.New("condition count changed outside audit bounds")
	}
	h := sha256.New()
	h.Write([]byte(RequirementInputAuditSchema + "\x00" + decision.DeclaredConditionFeatureVersion + "\x00"))
	h.Write(base[:])
	var cells [ConditionDim * 4]byte
	binary.LittleEndian.PutUint32(cells[:4], uint32(count))
	h.Write(cells[:4])
	for i := range count {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		row, err := source.ConditionFeatures(i)
		if err != nil {
			return result, err
		}
		if !finite(row[:]) {
			return result, errors.New("nonfinite audited condition")
		}
		for j, value := range row {
			binary.LittleEndian.PutUint32(cells[j*4:], math.Float32bits(value))
		}
		h.Write(cells[:])
	}
	copy(result[:], h.Sum(nil))
	return result, nil
}

func sameRequirementConditions(ctx context.Context, first, second ConditionSource) (bool, error) {
	if first.ConditionCount() != second.ConditionCount() || first.ConditionFeatureVersion() != second.ConditionFeatureVersion() {
		return false, nil
	}
	for i := range first.ConditionCount() {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		a, err := first.ConditionFeatures(i)
		if err != nil {
			return false, err
		}
		b, err := second.ConditionFeatures(i)
		if err != nil {
			return false, err
		}
		if !sameAuditCells(a[:], b[:]) {
			return false, nil
		}
	}
	return true, nil
}
