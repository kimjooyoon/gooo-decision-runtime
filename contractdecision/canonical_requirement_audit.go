package contractdecision

import (
	"context"
	"errors"
)

const CanonicalRequirementInputAuditSchema = "gooo/canonical-requirement-input-audit/v1"

// AuditCanonicalRequirements compares complete stored inputs and acceptable
// candidate sets. It does not fit or predict, and cannot establish correctness
// outside the supplied labels. Reserved channels must match the canonical ABI.
func AuditCanonicalRequirements(ctx context.Context, samples []OrderedRequirementSample) (*InputAudit, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 {
		return nil, errors.New("canonical audit requires context and 1..4096 samples")
	}
	for _, sample := range samples {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := validateCanonicalSourceRows(sample.Inputs); err != nil {
			return nil, err
		}
	}
	audit, err := AuditOrderedRequirements(ctx, samples)
	if err != nil {
		return nil, err
	}
	audit.Schema = CanonicalRequirementInputAuditSchema
	return audit, nil
}
