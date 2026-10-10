package contractdecision

import (
	"context"
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// CaseFeatureVersion is part of the artifact and source-reader contract.
// Readers without an explicit version retain the original declared-case v1 ABI.
func (m *Model) CaseFeatureVersion() string {
	if m != nil && m.sourceLiterals {
		return decision.SourceLiteralCaseFeatureVersion
	}
	return decision.DeclaredCaseFeatureVersion
}

func NewForCaseFeatures(weights [ParameterCount]float32, pooling, version string) (*Model, error) {
	if version != decision.DeclaredCaseFeatureVersion && version != decision.SourceLiteralCaseFeatureVersion {
		return nil, errors.New("unsupported contract case feature version")
	}
	model, err := NewForPooling(weights, pooling)
	if err == nil {
		model.sourceLiterals = version == decision.SourceLiteralCaseFeatureVersion
	}
	return model, err
}

func validateCaseVersion(cases CaseSource, want string) error {
	version := decision.DeclaredCaseFeatureVersion
	if reader, ok := cases.(interface{ CaseFeatureVersion() string }); ok {
		version = reader.CaseFeatureVersion()
	}
	if version != want {
		return errors.New("contract case reader and model feature versions differ")
	}
	return nil
}

// FitForCaseFeatures uses the ordinary candidate objective with an explicit
// case representation. The parameter layout and CPU scratch bounds are fixed.
func FitForCaseFeatures(ctx context.Context, samples []Sample, options FitOptions, pooling, version string) (*Model, []Epoch, error) {
	return fit(ctx, samples, options, pooling, nil, GoalPairOptions{}, version)
}
