package conditiondecision

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const Schema = "gooo/condition-candidate-decision/v1"

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
		return nil, errors.New("condition model required")
	}
	return json.Marshal(artifact{Schema, decision.ConditionChannelFeatureVersion, [3]int{FeatureDim, HiddenDim, 2}, m.weights[:]})
}

func Decode(raw []byte) (*Model, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded condition model artifact required")
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
		return nil, errors.New("trailing condition model artifact")
	}
	if a.Schema != Schema || a.Features != decision.ConditionChannelFeatureVersion || a.Architecture != [3]int{FeatureDim, HiddenDim, 2} || len(a.Weights) != ParameterCount {
		return nil, errors.New("condition model artifact contract differs")
	}
	var weights [ParameterCount]float32
	copy(weights[:], a.Weights)
	return New(weights)
}
