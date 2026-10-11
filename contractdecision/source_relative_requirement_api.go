package contractdecision

import (
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// NewSourceRelativeRequirementConditioned uses the exact numerical and artifact
// contract of the published mean-pooling research model. It holds one weight
// array. Constructing or decoding does not fit weights or make predictions.
func NewSourceRelativeRequirementConditioned(weights [InteractionRequirementParameterCount]float32) (*SourceRelativeRequirementModel, error) {
	inner, err := NewInteractionRequirementConditioned(weights, MeanPooling)
	if err != nil {
		return nil, err
	}
	model := &SourceRelativeRequirementModel{inner: inner}
	model.bindFingerprint()
	return model, nil
}

func (m *SourceRelativeRequirementModel) SourceFeatureVersion() string {
	return SourceRelativeOrderedSourceFeatureVersion
}
func (m *SourceRelativeRequirementModel) CaseFeatureVersion() string {
	return decision.DeclaredCaseFeatureVersion
}
func (m *SourceRelativeRequirementModel) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}
func (m *SourceRelativeRequirementModel) Pooling() string { return MeanPooling }

func (m *SourceRelativeRequirementModel) PredictInto(inputs [][OrderedFeatureDim]float32, cases CaseSource, conditions ConditionSource,
	masks []uint16, workspace *InteractionRequirementWorkspace, output *Prediction) error {
	if m == nil || m.inner == nil {
		return errors.New("source-relative model required")
	}
	if err := validateSourceRelativeConditions(inputs, conditions); err != nil {
		return err
	}
	return m.inner.PredictInto(inputs, cases, conditions, masks, workspace, output)
}
