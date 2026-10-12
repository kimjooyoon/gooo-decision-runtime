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

const SourceRelativeV3Schema = "gooo/source-relative-condition-research-model/v3"
const SourceRelativeV3FeatureVersion = "canonical_branch_intent_condition_output_relations_v3"

// V3 reuses one numerical parameter array and adds an explicit source-owned
// output relationship channel. V2 artifacts and their reserved cells stay intact.
type SourceRelativeV3Model struct {
	inner       *InteractionRequirementModel
	fingerprint string
}

type sourceRelativeV3Artifact struct {
	sourceRelativeRequirementArtifact
	OutputRelations string `json:"output_relation_features"`
}

func NewSourceRelativeV3(weights [InteractionRequirementParameterCount]float32) (*SourceRelativeV3Model, error) {
	inner, err := NewInteractionRequirementConditioned(weights, MeanPooling)
	if err != nil {
		return nil, err
	}
	m := &SourceRelativeV3Model{inner: inner}
	raw, err := m.Marshal()
	if err != nil {
		return nil, err
	}
	sha := sha256.Sum256(raw)
	m.fingerprint = hex.EncodeToString(sha[:])
	return m, nil
}

func (m *SourceRelativeV3Model) ArtifactSchema() string       { return SourceRelativeV3Schema }
func (m *SourceRelativeV3Model) SourceFeatureVersion() string { return SourceRelativeV3FeatureVersion }
func (m *SourceRelativeV3Model) CaseFeatureVersion() string {
	return decision.DeclaredCaseFeatureVersion
}
func (m *SourceRelativeV3Model) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}
func (m *SourceRelativeV3Model) Pooling() string { return MeanPooling }
func (m *SourceRelativeV3Model) Fingerprint() string {
	if m == nil {
		return ""
	}
	return m.fingerprint
}
func (m *SourceRelativeV3Model) Marshal() ([]byte, error) {
	if m == nil || m.inner == nil {
		return nil, errors.New("source-relative V3 model required")
	}
	return json.Marshal(sourceRelativeV3Artifact{sourceRelativeRequirementArtifact: sourceRelativeRequirementArtifact{
		Schema: SourceRelativeV3Schema, SourceFeatures: SourceRelativeV3FeatureVersion,
		RelationFeatures: decision.SourceRelativeConditionFeatureVersion, OutputFeatures: decision.DeclaredCaseFeatureVersion,
		ConditionFeatures: decision.DeclaredConditionFeatureVersion, Pooling: MeanPooling, Weights: m.inner.Weights()},
		OutputRelations: decision.SourceRelativeOutputFeatureVersion})
}

func DecodeSourceRelativeV3(raw []byte) (*SourceRelativeV3Model, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded source-relative V3 artifact required")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	var artifact sourceRelativeV3Artifact
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&artifact); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing V3 artifact")
	}
	if artifact.Schema != SourceRelativeV3Schema || artifact.SourceFeatures != SourceRelativeV3FeatureVersion ||
		artifact.OutputRelations != decision.SourceRelativeOutputFeatureVersion {
		return nil, errors.New("source-relative V3 artifact identity differs")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	delete(fields, "output_relation_features")
	fields["schema"], _ = json.Marshal(SourceRelativeV2Schema)
	fields["source_features"], _ = json.Marshal(SourceRelativeV2FeatureVersion)
	translated, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	v2, err := DecodeSourceRelativeV2(translated)
	if err != nil {
		return nil, err
	}
	return NewSourceRelativeV3(v2.inner.Weights())
}

func validateSourceRelativeV3(inputs [][528]float32, cases CaseSource, conditions ConditionSource) error {
	if len(inputs) < 1 || len(inputs) > 2 || cases == nil || cases.CaseCount() < 1 || cases.CaseCount() > MaxCases {
		return errors.New("bounded V3 source and declared cases required")
	}
	if err := validateCaseVersion(cases, decision.DeclaredCaseFeatureVersion); err != nil {
		return err
	}
	var legacy [2][528]float32
	for i, row := range inputs {
		var mass float64
		for slot, value := range row[208:256] {
			if math.IsNaN(float64(value)) || value < 0 || value > 1 {
				return errors.New("invalid V3 output relation cell")
			}
			if slot%16 == 0 {
				mass += float64(value)
			}
		}
		if math.Abs(mass-1) > 1e-6 {
			return errors.New("V3 output relation case mass differs")
		}
		for bucket := range 3 {
			block := row[208+16*bucket : 224+16*bucket]
			if math.Abs(float64(block[1])+float64(block[2])+float64(block[3])-float64(block[0])) > 1e-6 {
				return errors.New("V3 output comparison partition differs")
			}
			for _, value := range block[4:] {
				if value > block[0]+1e-6 {
					return errors.New("V3 literal relationship exceeds its source region")
				}
			}
		}
		legacy[i] = row
		clear(legacy[i][208:256])
	}
	return validateSourceRelativeV2(legacy[:len(inputs)], conditions)
}

func (m *SourceRelativeV3Model) PredictChoicesInto(inputs [][528]float32, cases CaseSource, conditions ConditionSource,
	work *InteractionRequirementWorkspace, out *ChoicePrediction) error {
	if m == nil || m.inner == nil {
		return errors.New("source-relative V3 model required")
	}
	if err := validateSourceRelativeV3(inputs, cases, conditions); err != nil {
		return err
	}
	return m.inner.PredictChoicesInto(inputs, cases, conditions, work, out)
}

func (m *SourceRelativeV3Model) PredictInto(inputs [][528]float32, cases CaseSource, conditions ConditionSource,
	masks []uint16, work *InteractionRequirementWorkspace, out *Prediction) error {
	if m == nil || m.inner == nil {
		return errors.New("source-relative V3 model required")
	}
	if err := validateSourceRelativeV3(inputs, cases, conditions); err != nil {
		return err
	}
	return m.inner.PredictInto(inputs, cases, conditions, masks, work, out)
}

func FitSourceRelativeV3(ctx context.Context, samples []InteractionRequirementSample,
	options FitOptions) (*SourceRelativeV3Model, []Epoch, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 || !validOptions(options) {
		return nil, nil, errors.New("bounded source-relative V3 training inputs required")
	}
	for _, sample := range samples {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if err := validateSourceRelativeV3(sample.Inputs, sample.Cases, sample.Conditions); err != nil {
			return nil, nil, err
		}
	}
	m, history, err := FitInteractionRequirementConditioned(ctx, samples, options, MeanPooling)
	if err != nil {
		return nil, history, err
	}
	result, err := NewSourceRelativeV3(m.Weights())
	return result, history, err
}
