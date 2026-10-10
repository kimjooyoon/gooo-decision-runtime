package contractdecision

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const Schema = "gooo/contract-candidate-decision/v1"
const activation = "leaky_relu_0.01"
const pooling = "arithmetic_mean"

type artifact struct {
	Schema         string    `json:"schema"`
	SourceFeatures string    `json:"source_feature_version"`
	CaseFeatures   string    `json:"case_feature_version"`
	Architecture   [5]int    `json:"architecture"`
	Activation     string    `json:"activation"`
	Pooling        string    `json:"pooling"`
	Weights        []float32 `json:"weights_fp32"`
}

func (m *Model) Marshal() ([]byte, error) {
	if m == nil {
		return nil, errors.New("contract model required")
	}
	return json.Marshal(artifact{Schema, decision.RelationalFlowFeatureVersion, decision.DeclaredCaseFeatureVersion, [5]int{CaseDim, PoolDim, FeatureDim, HiddenDim, 2}, activation, pooling, m.weights[:]})
}

func Decode(raw []byte) (*Model, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded contract artifact required")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var a artifact
	if err := d.Decode(&a); err != nil {
		return nil, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing contract artifact")
	}
	if a.Schema != Schema || a.SourceFeatures != decision.RelationalFlowFeatureVersion || a.CaseFeatures != decision.DeclaredCaseFeatureVersion || a.Architecture != [5]int{CaseDim, PoolDim, FeatureDim, HiddenDim, 2} || a.Activation != activation || a.Pooling != pooling || len(a.Weights) != ParameterCount {
		return nil, errors.New("contract artifact ABI differs")
	}
	var weights [ParameterCount]float32
	copy(weights[:], a.Weights)
	return New(weights)
}

func (m *Model) Fingerprint() string {
	if m == nil {
		return ""
	}
	d := sha256.New()
	_, _ = d.Write([]byte(Schema + "\x00" + decision.RelationalFlowFeatureVersion + "\x00" + decision.DeclaredCaseFeatureVersion + "\x00" + activation + "\x00" + pooling + "\x00"))
	var raw [ParameterCount * 4]byte
	for i, w := range m.weights {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(w))
	}
	_, _ = d.Write(raw[:])
	return hex.EncodeToString(d.Sum(nil))
}
