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

const ChoiceSchema = "gooo/choice-conditioned-contract/v1"
const choiceInteraction = "source_bias_before_case_activation"

type choiceArtifact struct {
	Schema         string    `json:"schema"`
	SourceFeatures string    `json:"source_feature_version"`
	CaseFeatures   string    `json:"case_feature_version"`
	Architecture   [6]int    `json:"architecture"`
	Interaction    string    `json:"interaction"`
	Activation     string    `json:"activation"`
	Pooling        string    `json:"pooling"`
	Weights        []float32 `json:"weights_fp32"`
	Context        []float32 `json:"source_case_weights_fp32"`
}

func (m *ChoiceModel) ArtifactSchema() string { return ChoiceSchema }

func (m *ChoiceModel) Marshal() ([]byte, error) {
	if m == nil {
		return nil, errors.New("choice-conditioned model required")
	}
	return json.Marshal(choiceArtifact{ChoiceSchema, decision.RelationalFlowFeatureVersion, decision.DeclaredCaseFeatureVersion,
		[6]int{FeatureDim, CaseDim, PoolDim, FeatureDim, HiddenDim, 2}, choiceInteraction, activation, m.Pooling(), m.base.weights[:], m.context[:]})
}

func DecodeChoiceConditioned(raw []byte) (*ChoiceModel, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded choice-conditioned artifact required")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var a choiceArtifact
	if err := d.Decode(&a); err != nil {
		return nil, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing choice-conditioned artifact")
	}
	if a.Schema != ChoiceSchema || a.SourceFeatures != decision.RelationalFlowFeatureVersion || a.CaseFeatures != decision.DeclaredCaseFeatureVersion || a.Architecture != [6]int{FeatureDim, CaseDim, PoolDim, FeatureDim, HiddenDim, 2} || a.Interaction != choiceInteraction || a.Activation != activation || len(a.Weights) != ParameterCount || len(a.Context) != ChoiceContextParameterCount {
		return nil, errors.New("choice-conditioned artifact ABI differs")
	}
	var weights [ParameterCount]float32
	var contextWeights [ChoiceContextParameterCount]float32
	copy(weights[:], a.Weights)
	copy(contextWeights[:], a.Context)
	return NewChoiceConditioned(weights, contextWeights, a.Pooling)
}

func (m *ChoiceModel) Fingerprint() string {
	if m == nil {
		return ""
	}
	d := sha256.New()
	_, _ = d.Write([]byte(ChoiceSchema + "\x00" + decision.RelationalFlowFeatureVersion + "\x00" + decision.DeclaredCaseFeatureVersion + "\x00" + choiceInteraction + "\x00" + activation + "\x00" + m.Pooling() + "\x00"))
	var raw [ChoiceParameterCount * 4]byte
	for i, weight := range m.base.weights {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(weight))
	}
	for i, weight := range m.context {
		binary.LittleEndian.PutUint32(raw[(ParameterCount+i)*4:], math.Float32bits(weight))
	}
	_, _ = d.Write(raw[:])
	return hex.EncodeToString(d.Sum(nil))
}
