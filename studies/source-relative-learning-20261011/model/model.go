// Package model is the study-only adapter for source-relative condition inputs.
// It uses the existing numerical kernel with a separate artifact/input identity.
package model

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
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

const Schema = "gooo/source-relative-condition-research-model/v1"
const SourceFeatures = "canonical_branch_intent_ordered_condition_relations_v1"

type Model struct {
	inner       *contractdecision.InteractionRequirementModel
	fingerprint string
}
type artifact struct {
	Schema            string                                                         `json:"schema"`
	SourceFeatures    string                                                         `json:"source_features"`
	RelationFeatures  string                                                         `json:"relation_features"`
	OutputFeatures    string                                                         `json:"output_features"`
	ConditionFeatures string                                                         `json:"condition_features"`
	Pooling           string                                                         `json:"pooling"`
	Weights           [contractdecision.InteractionRequirementParameterCount]float32 `json:"weights"`
}

func validate(inputs [][528]float32, conditions contractdecision.ConditionSource) error {
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

func Fit(ctx context.Context, samples []contractdecision.InteractionRequirementSample, options contractdecision.FitOptions) (*Model, []contractdecision.Epoch, error) {
	for _, sample := range samples {
		if err := validate(sample.Inputs, sample.Conditions); err != nil {
			return nil, nil, err
		}
	}
	m, history, err := contractdecision.FitInteractionRequirementConditioned(ctx, samples, options, contractdecision.MeanPooling)
	if err != nil {
		return nil, history, err
	}
	wrapped := &Model{inner: m}
	wrapped.bindFingerprint()
	return wrapped, history, nil
}

func (m *Model) PredictChoicesInto(inputs [][528]float32, cases contractdecision.CaseSource, conditions contractdecision.ConditionSource,
	w *contractdecision.InteractionRequirementWorkspace, out *contractdecision.ChoicePrediction) error {
	if m == nil || m.inner == nil {
		return errors.New("source-relative model required")
	}
	if err := validate(inputs, conditions); err != nil {
		return err
	}
	return m.inner.PredictChoicesInto(inputs, cases, conditions, w, out)
}

func (m *Model) ArtifactSchema() string { return Schema }
func (m *Model) Marshal() ([]byte, error) {
	if m == nil || m.inner == nil {
		return nil, errors.New("source-relative model required")
	}
	return json.Marshal(artifact{Schema: Schema, SourceFeatures: SourceFeatures,
		RelationFeatures: pathplan.SourceRelativeConditionFeatureVersion, OutputFeatures: decision.DeclaredCaseFeatureVersion,
		ConditionFeatures: decision.DeclaredConditionFeatureVersion, Pooling: contractdecision.MeanPooling, Weights: m.inner.Weights()})
}
func (m *Model) Fingerprint() string {
	if m == nil {
		return ""
	}
	return m.fingerprint
}
func (m *Model) bindFingerprint() {
	raw, err := m.Marshal()
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(raw)
	m.fingerprint = hex.EncodeToString(digest[:])
}

func Decode(raw []byte) (*Model, error) {
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
	var a artifact
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
	if err := json.Unmarshal(fields["weights"], &weights); err != nil || len(weights) != len(a.Weights) {
		return nil, errors.New("source-relative model weight count differs")
	}
	if a.Schema != Schema || a.SourceFeatures != SourceFeatures || a.RelationFeatures != pathplan.SourceRelativeConditionFeatureVersion ||
		a.OutputFeatures != decision.DeclaredCaseFeatureVersion || a.ConditionFeatures != decision.DeclaredConditionFeatureVersion || a.Pooling != contractdecision.MeanPooling {
		return nil, errors.New("source-relative artifact identity differs")
	}
	m, err := contractdecision.NewInteractionRequirementConditioned(a.Weights, a.Pooling)
	if err != nil {
		return nil, err
	}
	wrapped := &Model{inner: m}
	wrapped.bindFingerprint()
	return wrapped, nil
}
