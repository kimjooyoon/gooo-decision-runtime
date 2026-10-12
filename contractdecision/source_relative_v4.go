package contractdecision

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const SourceRelativeV4Schema = "gooo/source-relative-branch-output-model/v4"
const SourceRelativeV4FeatureVersion = "canonical_branch_owned_output_relations_no_intent_v4"

// SourceRelativeV4Model adds branch-owned relationship inputs while retaining
// the bounded 19,034-parameter architecture. It requires its own fitted weights.
type SourceRelativeV4Model struct {
	inner       *InteractionRequirementModel
	fingerprint string
}

type sourceRelativeV4Artifact struct {
	sourceRelativeV3Artifact
	BranchRelations string `json:"branch_output_relation_features"`
	IntentPolicy    string `json:"intent_policy"`
}

func NewSourceRelativeV4(weights [InteractionRequirementParameterCount]float32) (*SourceRelativeV4Model, error) {
	inner, err := NewInteractionRequirementConditioned(weights, MeanPooling)
	if err != nil {
		return nil, err
	}
	m := &SourceRelativeV4Model{inner: inner}
	raw, err := m.Marshal()
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	m.fingerprint = hex.EncodeToString(digest[:])
	return m, nil
}
func (m *SourceRelativeV4Model) ArtifactSchema() string       { return SourceRelativeV4Schema }
func (m *SourceRelativeV4Model) SourceFeatureVersion() string { return SourceRelativeV4FeatureVersion }
func (m *SourceRelativeV4Model) CaseFeatureVersion() string {
	return decision.DeclaredCaseFeatureVersion
}
func (m *SourceRelativeV4Model) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}
func (m *SourceRelativeV4Model) Pooling() string { return MeanPooling }
func (m *SourceRelativeV4Model) Fingerprint() string {
	if m == nil {
		return ""
	}
	return m.fingerprint
}
func (m *SourceRelativeV4Model) Marshal() ([]byte, error) {
	if m == nil || m.inner == nil {
		return nil, errors.New("source-relative V4 model required")
	}
	return json.Marshal(sourceRelativeV4Artifact{
		sourceRelativeV3Artifact: sourceRelativeV3Artifact{
			sourceRelativeRequirementArtifact: sourceRelativeRequirementArtifact{
				Schema: SourceRelativeV4Schema, SourceFeatures: SourceRelativeV4FeatureVersion,
				RelationFeatures:  decision.SourceRelativeConditionFeatureVersion,
				OutputFeatures:    decision.DeclaredCaseFeatureVersion,
				ConditionFeatures: decision.DeclaredConditionFeatureVersion,
				Pooling:           MeanPooling, Weights: m.inner.Weights()},
			OutputRelations: decision.SourceRelativeOutputFeatureVersion},
		BranchRelations: decision.BranchOutputFeatureVersion,
		IntentPolicy:    SourceRelativeV3ProseNeutralIntentPolicy})
}

func DecodeSourceRelativeV4(raw []byte) (*SourceRelativeV4Model, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded V4 artifact required")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	var artifact sourceRelativeV4Artifact
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&artifact); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing V4 artifact")
	}
	if artifact.Schema != SourceRelativeV4Schema || artifact.SourceFeatures != SourceRelativeV4FeatureVersion ||
		artifact.BranchRelations != decision.BranchOutputFeatureVersion || artifact.IntentPolicy != SourceRelativeV3ProseNeutralIntentPolicy {
		return nil, errors.New("V4 artifact input contract differs")
	}
	// Delegate the unchanged tensor shape, pooling and older channel contracts to
	// the V3 decoder. This does not accept a V3 artifact as a V4 artifact.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	delete(fields, "branch_output_relation_features")
	delete(fields, "intent_policy")
	fields["schema"], _ = json.Marshal(SourceRelativeV3Schema)
	fields["source_features"], _ = json.Marshal(SourceRelativeV3FeatureVersion)
	translated, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	v3, err := DecodeSourceRelativeV3(translated)
	if err != nil {
		return nil, err
	}
	return NewSourceRelativeV4(v3.inner.Weights())
}

func validateSourceRelativeV4(inputs [][528]float32, cases CaseSource, conditions ConditionSource) error {
	if err := validateProseNeutralInputs(inputs); err != nil {
		return err
	}
	var legacy [2][528]float32
	for i, row := range inputs {
		for branch := range 2 {
			for bucket := range 3 {
				block := row[256+branch*48+bucket*16 : 272+branch*48+bucket*16]
				for slot, value := range block {
					if math.IsNaN(float64(value)) || value < 0 || value > 1 ||
						slot < 4 && value != row[208+bucket*16+slot] || slot >= 4 && value > block[0]+1e-6 {
						return errors.New("invalid V4 branch relationship cell")
					}
				}
			}
		}
		legacy[i] = row
		clear(legacy[i][256:352])
	}
	return validateSourceRelativeV3(legacy[:len(inputs)], cases, conditions)
}

func (m *SourceRelativeV4Model) PredictChoicesInto(inputs [][528]float32, cases CaseSource, conditions ConditionSource,
	work *InteractionRequirementWorkspace, out *ChoicePrediction) error {
	if m == nil || m.inner == nil {
		return errors.New("source-relative V4 model required")
	}
	if err := validateSourceRelativeV4(inputs, cases, conditions); err != nil {
		return err
	}
	return m.inner.PredictChoicesInto(inputs, cases, conditions, work, out)
}
func (m *SourceRelativeV4Model) PredictInto(inputs [][528]float32, cases CaseSource, conditions ConditionSource,
	masks []uint16, work *InteractionRequirementWorkspace, out *Prediction) error {
	if m == nil || m.inner == nil {
		return errors.New("source-relative V4 model required")
	}
	if err := validateSourceRelativeV4(inputs, cases, conditions); err != nil {
		return err
	}
	return m.inner.PredictInto(inputs, cases, conditions, masks, work, out)
}
func FitSourceRelativeV4(ctx context.Context, samples []InteractionRequirementSample,
	options FitOptions) (*SourceRelativeV4Model, []Epoch, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 || !validOptions(options) {
		return nil, nil, errors.New("bounded source-relative V4 training inputs required")
	}
	for _, sample := range samples {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if err := validateSourceRelativeV4(sample.Inputs, sample.Cases, sample.Conditions); err != nil {
			return nil, nil, err
		}
	}
	m, history, err := FitInteractionRequirementConditioned(ctx, samples, options, MeanPooling)
	if err != nil {
		return nil, history, err
	}
	result, err := NewSourceRelativeV4(m.Weights())
	return result, history, err
}
