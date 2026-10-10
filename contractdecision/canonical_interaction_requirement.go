package contractdecision

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const CanonicalOrderedSourceFeatureVersion = "canonical_branch_intent_ordered_expressions_v1"
const CanonicalInteractionRequirementSchema = "gooo/canonical-interaction-requirement-contract/v1"

// CanonicalInteractionRequirementModel has the same 19,034 weights and learning
// equations as InteractionRequirementModel, but consumes a different source ABI.
// Its separate type, decoder and fingerprint keep old artifacts on their old
// input path. The wrapper retains one model, not a second weight array.
type CanonicalInteractionRequirementModel struct{ model *InteractionRequirementModel }

func NewCanonicalInteractionRequirementConditioned(weights [InteractionRequirementParameterCount]float32, pooling string) (*CanonicalInteractionRequirementModel, error) {
	m, err := NewInteractionRequirementConditioned(weights, pooling)
	if err != nil {
		return nil, err
	}
	return &CanonicalInteractionRequirementModel{model: m}, nil
}

func FitCanonicalInteractionRequirementConditioned(ctx context.Context, samples []InteractionRequirementSample, options FitOptions, pooling string) (*CanonicalInteractionRequirementModel, []Epoch, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 {
		return nil, nil, errors.New("bounded canonical requirement training inputs required")
	}
	for _, sample := range samples {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if err := validateCanonicalSourceRows(sample.Inputs); err != nil {
			return nil, nil, err
		}
	}
	m, history, err := FitInteractionRequirementConditioned(ctx, samples, options, pooling)
	if err != nil {
		return nil, history, err
	}
	return &CanonicalInteractionRequirementModel{model: m}, history, nil
}

func (m *CanonicalInteractionRequirementModel) SourceFeatureVersion() string {
	return CanonicalOrderedSourceFeatureVersion
}
func (m *CanonicalInteractionRequirementModel) CaseFeatureVersion() string {
	return decision.DeclaredCaseFeatureVersion
}
func (m *CanonicalInteractionRequirementModel) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}
func (m *CanonicalInteractionRequirementModel) ArtifactSchema() string {
	return CanonicalInteractionRequirementSchema
}
func (m *CanonicalInteractionRequirementModel) Pooling() string {
	if m == nil {
		return MeanPooling
	}
	return m.model.Pooling()
}
func (m *CanonicalInteractionRequirementModel) Weights() [InteractionRequirementParameterCount]float32 {
	return m.model.Weights()
}

func (m *CanonicalInteractionRequirementModel) PredictInto(inputs [][OrderedFeatureDim]float32, cases CaseSource, conditions ConditionSource,
	masks []uint16, workspace *InteractionRequirementWorkspace, output *Prediction) error {
	if m == nil {
		return errors.New("canonical requirement model required")
	}
	if err := validateCanonicalSourceRows(inputs); err != nil {
		return err
	}
	return m.model.PredictInto(inputs, cases, conditions, masks, workspace, output)
}

func (m *CanonicalInteractionRequirementModel) PredictChoicesInto(inputs [][OrderedFeatureDim]float32, cases CaseSource, conditions ConditionSource,
	workspace *InteractionRequirementWorkspace, output *ChoicePrediction) error {
	if m == nil {
		return errors.New("canonical requirement model required")
	}
	if err := validateCanonicalSourceRows(inputs); err != nil {
		return err
	}
	return m.model.PredictChoicesInto(inputs, cases, conditions, workspace, output)
}

func validateCanonicalSourceRows(inputs [][OrderedFeatureDim]float32) error {
	if len(inputs) < 1 || len(inputs) > MaxChoices {
		return errors.New("canonical source requires 1..16 choices")
	}
	for _, row := range inputs {
		for _, value := range row[192:384] {
			if value != 0 {
				return errors.New("canonical source reserved channels must be zero")
			}
		}
	}
	return nil
}

func (m *CanonicalInteractionRequirementModel) Marshal() ([]byte, error) {
	if m == nil {
		return nil, errors.New("canonical requirement model required")
	}
	raw, err := m.model.Marshal()
	if err != nil {
		return nil, err
	}
	var artifact interactionRequirementArtifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		return nil, err
	}
	artifact.Schema, artifact.SourceFeatures = CanonicalInteractionRequirementSchema, CanonicalOrderedSourceFeatureVersion
	return json.Marshal(artifact)
}

func DecodeCanonicalInteractionRequirementConditioned(raw []byte) (*CanonicalInteractionRequirementModel, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded canonical requirement artifact required")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var artifact interactionRequirementArtifact
	if err := decoder.Decode(&artifact); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing canonical requirement artifact")
	}
	if artifact.Schema != CanonicalInteractionRequirementSchema || artifact.SourceFeatures != CanonicalOrderedSourceFeatureVersion {
		return nil, errors.New("canonical requirement artifact ABI differs")
	}
	// Reuse the exact numerical/shape validation without permitting old source
	// metadata at this public boundary. No fitting or prediction occurs here.
	artifact.Schema, artifact.SourceFeatures = InteractionRequirementSchema, OrderedSourceFeatureVersion
	base, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	m, err := DecodeInteractionRequirementConditioned(base)
	if err != nil {
		return nil, err
	}
	return &CanonicalInteractionRequirementModel{model: m}, nil
}

func (m *CanonicalInteractionRequirementModel) Fingerprint() string {
	if m == nil || m.model == nil {
		return ""
	}
	digest := sha256.Sum256([]byte(CanonicalInteractionRequirementSchema + "\x00" + CanonicalOrderedSourceFeatureVersion + "\x00" + m.model.Fingerprint()))
	return hex.EncodeToString(digest[:])
}
