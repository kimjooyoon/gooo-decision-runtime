package flowdecision

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const Schema = "gooo/flow-candidate-decision/v1"

type artifact struct {
	Schema       string    `json:"schema"`
	Features     string    `json:"feature_version"`
	Architecture [3]int    `json:"architecture"`
	Weights      []float32 `json:"weights_fp32"`
}

// Marshal exports only this closed FP32 ranking ABI. It is deliberately distinct
// from previous path-model and ternary artifacts. Fit does not write files.
func (m *Model) Marshal() ([]byte, error) {
	if m == nil {
		return nil, errors.New("flow model required")
	}
	return json.Marshal(artifact{Schema, m.FeatureVersion(), [3]int{FeatureDim, HiddenDim, 2}, m.weights[:]})
}

func Decode(raw []byte) (*Model, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded flow model artifact required")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var a artifact
	if err := decoder.Decode(&a); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing flow model artifact")
	}
	if a.Schema != Schema || !supportedFeatures(a.Features) || a.Architecture != [3]int{FeatureDim, HiddenDim, 2} || len(a.Weights) != ParameterCount {
		return nil, errors.New("flow model artifact contract differs")
	}
	var weights [ParameterCount]float32
	copy(weights[:], a.Weights)
	return NewForFeatures(weights, a.Features)
}
