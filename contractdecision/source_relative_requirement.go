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

const SourceRelativeRequirementSchema = "gooo/source-relative-condition-research-model/v1"
const SourceRelativeOrderedSourceFeatureVersion = "canonical_branch_intent_ordered_condition_relations_v1"

// SourceRelativeRequirementModel consumes canonical source rows with explicit
// predicate/goal relations. Its research artifact format and cached fingerprint
// are byte-compatible with the published original; old model inputs stay separate.
type SourceRelativeRequirementModel struct {
	inner       *InteractionRequirementModel
	fingerprint string
}
type sourceRelativeRequirementArtifact struct {
	Schema            string                                        `json:"schema"`
	SourceFeatures    string                                        `json:"source_features"`
	RelationFeatures  string                                        `json:"relation_features"`
	OutputFeatures    string                                        `json:"output_features"`
	ConditionFeatures string                                        `json:"condition_features"`
	Pooling           string                                        `json:"pooling"`
	Weights           [InteractionRequirementParameterCount]float32 `json:"weights"`
}

func validateSourceRelativeConditions(inputs [][528]float32, conditions ConditionSource) error {
	if len(inputs) == 0 || len(inputs) > 2 || conditions == nil || conditions.ConditionCount() > 128 || conditions.ConditionCount() < 0 {
		return errors.New("bounded source-relative input required")
	}
	for _, row := range inputs {
		for _, value := range row[192:208] {
			if math.IsNaN(float64(value)) || value < 0 || value > 1 {
				return errors.New("invalid source-relative relation cell")
			}
		}
		for _, value := range row[208:384] {
			if value != 0 {
				return errors.New("reserved source-relative cells must be zero")
			}
		}
		for _, value := range row[192:200] {
			if value != 0 && value != 1 {
				return errors.New("binary source-relative header required")
			}
		}
		if row[193]+row[194]+row[195] != 1 || row[196]+row[197] != 1 || row[198]+row[199] != 1 || row[207] != 0 {
			return errors.New("source-relative header differs")
		}
		count := conditions.ConditionCount()
		if row[206] != float32(count)/128 || (row[192] == 1) != (count > 0) {
			return errors.New("source-relative declared count differs")
		}
		var total float64
		for _, value := range row[200:206] {
			total += float64(value)
		}
		want := float64(row[192])
		if math.Abs(total-want) > 1e-6 {
			return errors.New("source-relative histogram mass differs")
		}
	}
	return nil
}

func FitSourceRelativeRequirementConditioned(ctx context.Context, samples []InteractionRequirementSample, options FitOptions) (*SourceRelativeRequirementModel, []Epoch, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 {
		return nil, nil, errors.New("bounded source-relative training inputs required")
	}
	for _, sample := range samples {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if err := validateSourceRelativeConditions(sample.Inputs, sample.Conditions); err != nil {
			return nil, nil, err
		}
	}
	m, history, err := FitInteractionRequirementConditioned(ctx, samples, options, MeanPooling)
	if err != nil {
		return nil, history, err
	}
	wrapped := &SourceRelativeRequirementModel{inner: m}
	wrapped.bindFingerprint()
	return wrapped, history, nil
}

func (m *SourceRelativeRequirementModel) PredictChoicesInto(inputs [][528]float32, cases CaseSource, conditions ConditionSource,
	w *InteractionRequirementWorkspace, out *ChoicePrediction) error {
	if m == nil || m.inner == nil {
		return errors.New("source-relative model required")
	}
	if err := validateSourceRelativeConditions(inputs, conditions); err != nil {
		return err
	}
	return m.inner.PredictChoicesInto(inputs, cases, conditions, w, out)
}

func (m *SourceRelativeRequirementModel) ArtifactSchema() string {
	return SourceRelativeRequirementSchema
}
func (m *SourceRelativeRequirementModel) Marshal() ([]byte, error) {
	if m == nil || m.inner == nil {
		return nil, errors.New("source-relative model required")
	}
	return json.Marshal(sourceRelativeRequirementArtifact{Schema: SourceRelativeRequirementSchema, SourceFeatures: SourceRelativeOrderedSourceFeatureVersion,
		RelationFeatures: decision.SourceRelativeConditionFeatureVersion, OutputFeatures: decision.DeclaredCaseFeatureVersion,
		ConditionFeatures: decision.DeclaredConditionFeatureVersion, Pooling: MeanPooling, Weights: m.inner.Weights()})
}
func (m *SourceRelativeRequirementModel) Fingerprint() string {
	if m == nil {
		return ""
	}
	return m.fingerprint
}
func (m *SourceRelativeRequirementModel) bindFingerprint() {
	raw, err := m.Marshal()
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(raw)
	m.fingerprint = hex.EncodeToString(digest[:])
}

func DecodeSourceRelativeRequirementConditioned(raw []byte) (*SourceRelativeRequirementModel, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded source-relative model artifact required")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	// Decode weights through a slice first: encoding/json accepts a short array
	// into a Go array by padding zeros, which must not create a valid artifact.
	var weights []float32
	var a sourceRelativeRequirementArtifact
	if err := d.Decode(&a); err != nil {
		return nil, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing source-relative model artifact")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	// The float decoder treats null elements as zero. Numeric arrays contain no
	// other valid use of this token, so reject it before accepting model weights.
	if bytes.Contains(fields["weights"], []byte("null")) {
		return nil, errors.New("source-relative weights must be numbers")
	}
	if err := json.Unmarshal(fields["weights"], &weights); err != nil || len(weights) != len(a.Weights) {
		return nil, errors.New("source-relative model weight count differs")
	}
	if a.Schema != SourceRelativeRequirementSchema || a.SourceFeatures != SourceRelativeOrderedSourceFeatureVersion || a.RelationFeatures != decision.SourceRelativeConditionFeatureVersion ||
		a.OutputFeatures != decision.DeclaredCaseFeatureVersion || a.ConditionFeatures != decision.DeclaredConditionFeatureVersion || a.Pooling != MeanPooling {
		return nil, errors.New("source-relative artifact identity differs")
	}
	m, err := NewInteractionRequirementConditioned(a.Weights, a.Pooling)
	if err != nil {
		return nil, err
	}
	wrapped := &SourceRelativeRequirementModel{inner: m}
	wrapped.bindFingerprint()
	return wrapped, nil
}
