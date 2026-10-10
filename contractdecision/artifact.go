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
const PoolingSchema = "gooo/contract-candidate-decision/v2"
const activation = "leaky_relu_0.01"
const MeanPooling = "arithmetic_mean"
const ExtremePooling = "signed_max_abs"
const pooling = MeanPooling

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
	return json.Marshal(artifact{m.ArtifactSchema(), decision.RelationalFlowFeatureVersion, decision.DeclaredCaseFeatureVersion, [5]int{CaseDim, PoolDim, FeatureDim, HiddenDim, 2}, activation, m.Pooling(), m.weights[:]})
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
	validPooling := a.Schema == Schema && a.Pooling == MeanPooling || a.Schema == PoolingSchema && a.Pooling == ExtremePooling
	if !validPooling || a.SourceFeatures != decision.RelationalFlowFeatureVersion || a.CaseFeatures != decision.DeclaredCaseFeatureVersion || a.Architecture != [5]int{CaseDim, PoolDim, FeatureDim, HiddenDim, 2} || a.Activation != activation || len(a.Weights) != ParameterCount {
		return nil, errors.New("contract artifact ABI differs")
	}
	var weights [ParameterCount]float32
	copy(weights[:], a.Weights)
	return NewForPooling(weights, a.Pooling)
}

func (m *Model) Fingerprint() string {
	if m == nil {
		return ""
	}
	d := sha256.New()
	_, _ = d.Write([]byte(m.ArtifactSchema() + "\x00" + decision.RelationalFlowFeatureVersion + "\x00" + decision.DeclaredCaseFeatureVersion + "\x00" + activation + "\x00" + m.Pooling() + "\x00"))
	var raw [ParameterCount * 4]byte
	for i, w := range m.weights {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(w))
	}
	_, _ = d.Write(raw[:])
	return hex.EncodeToString(d.Sum(nil))
}
