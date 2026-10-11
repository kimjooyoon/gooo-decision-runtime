package contractdecision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const SourceRelativeV2Schema = "gooo/source-relative-condition-research-model/v2"
const SourceRelativeV2FeatureVersion = "canonical_branch_intent_ordered_condition_relations_v2"

// One parameter array is retained; V1 artifacts and input routes stay separate.
type SourceRelativeV2Model struct {
	inner       *InteractionRequirementModel
	fingerprint string
}

func NewSourceRelativeV2(weights [InteractionRequirementParameterCount]float32) (*SourceRelativeV2Model, error) {
	inner, err := NewInteractionRequirementConditioned(weights, MeanPooling)
	if err != nil {
		return nil, err
	}
	model := &SourceRelativeV2Model{inner: inner}
	raw, err := model.Marshal()
	if err != nil {
		return nil, err
	}
	sha := sha256.Sum256(raw)
	model.fingerprint = hex.EncodeToString(sha[:])
	return model, nil
}

func (m *SourceRelativeV2Model) ArtifactSchema() string       { return SourceRelativeV2Schema }
func (m *SourceRelativeV2Model) SourceFeatureVersion() string { return SourceRelativeV2FeatureVersion }
func (m *SourceRelativeV2Model) CaseFeatureVersion() string {
	return decision.DeclaredCaseFeatureVersion
}
func (m *SourceRelativeV2Model) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}
func (m *SourceRelativeV2Model) Pooling() string { return MeanPooling }
func (m *SourceRelativeV2Model) Fingerprint() string {
	if m == nil {
		return ""
	}
	return m.fingerprint
}

func (m *SourceRelativeV2Model) Marshal() ([]byte, error) {
	if m == nil || m.inner == nil {
		return nil, errors.New("source-relative V2 model required")
	}
	return json.Marshal(sourceRelativeRequirementArtifact{Schema: SourceRelativeV2Schema,
		SourceFeatures: SourceRelativeV2FeatureVersion, RelationFeatures: decision.SourceRelativeConditionFeatureVersion,
		OutputFeatures: decision.DeclaredCaseFeatureVersion, ConditionFeatures: decision.DeclaredConditionFeatureVersion,
		Pooling: MeanPooling, Weights: m.inner.Weights()})
}
