package flowdecision

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const Schema = "gooo/flow-candidate-decision/v1"
const ActivationSchema = "gooo/flow-candidate-decision/v2"

type artifact struct {
	Schema       string          `json:"schema"`
	Features     string          `json:"feature_version"`
	Architecture [3]int          `json:"architecture"`
	Weights      []float32       `json:"weights_fp32"`
	Activation   json.RawMessage `json:"activation,omitempty"`
}

// Marshal exports only this closed FP32 ranking ABI. It is deliberately distinct
// from previous path-model and ternary artifacts. Fit does not write files.
func (m *Model) Marshal() ([]byte, error) {
	if m == nil {
		return nil, errors.New("flow model required")
	}
	a := artifact{Schema: Schema, Features: m.FeatureVersion(), Architecture: [3]int{FeatureDim, HiddenDim, 2}, Weights: m.weights[:]}
	if m.Activation() == LeakyReLUActivation {
		a.Schema = ActivationSchema
		a.Activation, _ = json.Marshal(m.Activation())
	}
	return json.Marshal(a)
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
	if !supportedFeatures(a.Features) || a.Architecture != [3]int{FeatureDim, HiddenDim, 2} || len(a.Weights) != ParameterCount {
		return nil, errors.New("flow model artifact contract differs")
	}
	activation := ReLUActivation
	switch a.Schema {
	case Schema:
		if len(a.Activation) != 0 {
			return nil, errors.New("legacy flow artifact must omit activation")
		}
	case ActivationSchema:
		if err := json.Unmarshal(a.Activation, &activation); err != nil || activation != LeakyReLUActivation {
			return nil, errors.New("explicit leaky flow activation required")
		}
	default:
		return nil, errors.New("flow model activation contract differs")
	}
	var weights [ParameterCount]float32
	copy(weights[:], a.Weights)
	return NewForActivation(weights, a.Features, activation)
}
