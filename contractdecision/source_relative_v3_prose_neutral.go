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

const SourceRelativeV3ProseNeutralSchema = "gooo/source-relative-v3-prose-neutral-model/v1"
const SourceRelativeV3ProseNeutralFeatureVersion = "canonical_branch_condition_output_relations_no_intent_v1"
const SourceRelativeV3ProseNeutralIntentPolicy = "DECLARED_REQUIREMENTS_ONLY_V1"
const SourceRelativeV3ProseNeutralIntentStart = 64
const SourceRelativeV3ProseNeutralIntentEnd = 192

// SourceRelativeV3ProseNeutralModel uses the same bounded numerical model as V3,
// with an explicit requirement that the original intent channel is zeroed.
// Source-owned outputs and Boolean requirements remain independent inputs.
type SourceRelativeV3ProseNeutralModel struct {
	inner       *SourceRelativeV3Model
	fingerprint string
}
type proseNeutralV3Artifact struct {
	sourceRelativeV3Artifact
	IntentPolicy  string `json:"intent_policy"`
	V3Fingerprint string `json:"source_v3_fingerprint"`
}

func NewSourceRelativeV3ProseNeutral(weights [InteractionRequirementParameterCount]float32) (*SourceRelativeV3ProseNeutralModel, error) {
	inner, err := NewSourceRelativeV3(weights)
	if err != nil {
		return nil, err
	}
	return proseNeutralV3(inner)
}
func proseNeutralV3(inner *SourceRelativeV3Model) (*SourceRelativeV3ProseNeutralModel, error) {
	m := &SourceRelativeV3ProseNeutralModel{inner: inner}
	raw, err := m.Marshal()
	if err != nil {
		return nil, err
	}
	sha := sha256.Sum256(raw)
	m.fingerprint = hex.EncodeToString(sha[:])
	return m, nil
}
func (m *SourceRelativeV3ProseNeutralModel) ArtifactSchema() string {
	return SourceRelativeV3ProseNeutralSchema
}
func (m *SourceRelativeV3ProseNeutralModel) SourceFeatureVersion() string {
	return SourceRelativeV3ProseNeutralFeatureVersion
}
func (m *SourceRelativeV3ProseNeutralModel) CaseFeatureVersion() string {
	return decision.DeclaredCaseFeatureVersion
}
func (m *SourceRelativeV3ProseNeutralModel) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}
func (m *SourceRelativeV3ProseNeutralModel) Pooling() string { return MeanPooling }
func (m *SourceRelativeV3ProseNeutralModel) Fingerprint() string {
	if m == nil {
		return ""
	}
	return m.fingerprint
}
func (m *SourceRelativeV3ProseNeutralModel) SourceV3Fingerprint() string {
	if m == nil || m.inner == nil {
		return ""
	}
	return m.inner.Fingerprint()
}
func (m *SourceRelativeV3ProseNeutralModel) Marshal() ([]byte, error) {
	if m == nil || m.inner == nil {
		return nil, errors.New("explicit prose-neutral V3 model required")
	}
	return json.Marshal(proseNeutralV3Artifact{sourceRelativeV3Artifact: sourceRelativeV3Artifact{
		sourceRelativeRequirementArtifact: sourceRelativeRequirementArtifact{
			Schema: SourceRelativeV3ProseNeutralSchema, SourceFeatures: SourceRelativeV3ProseNeutralFeatureVersion,
			RelationFeatures: decision.SourceRelativeConditionFeatureVersion, OutputFeatures: decision.DeclaredCaseFeatureVersion,
			ConditionFeatures: decision.DeclaredConditionFeatureVersion, Pooling: MeanPooling, Weights: m.inner.inner.Weights()},
		OutputRelations: decision.SourceRelativeOutputFeatureVersion}, IntentPolicy: SourceRelativeV3ProseNeutralIntentPolicy,
		V3Fingerprint: m.inner.Fingerprint()})
}

func DecodeSourceRelativeV3ProseNeutral(raw []byte) (*SourceRelativeV3ProseNeutralModel, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded prose-neutral V3 artifact required")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	var artifact proseNeutralV3Artifact
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&artifact); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing prose-neutral artifact")
	}
	if artifact.Schema != SourceRelativeV3ProseNeutralSchema || artifact.SourceFeatures != SourceRelativeV3ProseNeutralFeatureVersion || artifact.IntentPolicy != SourceRelativeV3ProseNeutralIntentPolicy {
		return nil, errors.New("prose-neutral artifact input policy differs")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	delete(fields, "intent_policy")
	delete(fields, "source_v3_fingerprint")
	fields["schema"], _ = json.Marshal(SourceRelativeV3Schema)
	fields["source_features"], _ = json.Marshal(SourceRelativeV3FeatureVersion)
	translated, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	inner, err := DecodeSourceRelativeV3(translated)
	if err != nil {
		return nil, err
	}
	if artifact.V3Fingerprint != inner.Fingerprint() {
		return nil, errors.New("prose-neutral weights differ from their V3 fingerprint")
	}
	return proseNeutralV3(inner)
}

// RepackageSourceRelativeV3ProseNeutral records the input policy without fitting,
// predicting or changing tensors. The caller must use weights trained under that
// policy; repackaging cannot prove training history or decision quality.
func RepackageSourceRelativeV3ProseNeutral(raw []byte) ([]byte, error) {
	inner, err := DecodeSourceRelativeV3(raw)
	if err != nil {
		return nil, err
	}
	m, err := proseNeutralV3(inner)
	if err != nil {
		return nil, err
	}
	return m.Marshal()
}
func validateProseNeutralInputs(inputs [][528]float32) error {
	if len(inputs) < 1 || len(inputs) > 2 {
		return errors.New("one or two bounded prose-neutral inputs required")
	}
	for _, row := range inputs {
		for _, value := range row[SourceRelativeV3ProseNeutralIntentStart:SourceRelativeV3ProseNeutralIntentEnd] {
			if value != 0 {
				return errors.New("prose-neutral intent cells must be zero")
			}
		}
	}
	return nil
}
func (m *SourceRelativeV3ProseNeutralModel) PredictChoicesInto(inputs [][528]float32, cases CaseSource, conditions ConditionSource, work *InteractionRequirementWorkspace, out *ChoicePrediction) error {
	if m == nil || m.inner == nil {
		return errors.New("prose-neutral V3 model required")
	}
	if err := validateProseNeutralInputs(inputs); err != nil {
		return err
	}
	return m.inner.PredictChoicesInto(inputs, cases, conditions, work, out)
}
func (m *SourceRelativeV3ProseNeutralModel) PredictInto(inputs [][528]float32, cases CaseSource, conditions ConditionSource, masks []uint16, work *InteractionRequirementWorkspace, out *Prediction) error {
	if m == nil || m.inner == nil {
		return errors.New("prose-neutral V3 model required")
	}
	if err := validateProseNeutralInputs(inputs); err != nil {
		return err
	}
	return m.inner.PredictInto(inputs, cases, conditions, masks, work, out)
}
func FitSourceRelativeV3ProseNeutral(ctx context.Context, samples []InteractionRequirementSample, options FitOptions) (*SourceRelativeV3ProseNeutralModel, []Epoch, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 || !validOptions(options) {
		return nil, nil, errors.New("bounded prose-neutral V3 training inputs required")
	}
	for _, sample := range samples {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if err := validateProseNeutralInputs(sample.Inputs); err != nil {
			return nil, nil, err
		}
	}
	inner, history, err := FitSourceRelativeV3(ctx, samples, options)
	if err != nil {
		return nil, history, err
	}
	m, err := proseNeutralV3(inner)
	return m, history, err
}
