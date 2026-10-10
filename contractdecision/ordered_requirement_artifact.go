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

const OrderedRequirementSchema = "gooo/ordered-requirement-contract/v1"
const orderedRequirementInteraction = "source_conditioned_output_condition_pools_joint_hidden"
const orderedRequirementEmptyConditions = "zero_pool"

type orderedRequirementArtifact struct {
	Schema            string    `json:"schema"`
	SourceFeatures    string    `json:"source_feature_version"`
	CaseFeatures      string    `json:"case_feature_version"`
	ConditionFeatures string    `json:"condition_feature_version"`
	Architecture      [7]int    `json:"architecture"`
	Bounds            [3]int    `json:"bounds"`
	Interaction       string    `json:"interaction"`
	EmptyConditions   string    `json:"empty_conditions"`
	Activation        string    `json:"activation"`
	Pooling           string    `json:"pooling"`
	Weights           []float32 `json:"weights_fp32"`
}

func (m *OrderedRequirementModel) ArtifactSchema() string { return OrderedRequirementSchema }

func (m *OrderedRequirementModel) Marshal() ([]byte, error) {
	if m == nil {
		return nil, errors.New("requirement-conditioned model required")
	}
	return json.Marshal(orderedRequirementArtifact{
		Schema: OrderedRequirementSchema, SourceFeatures: OrderedSourceFeatureVersion,
		CaseFeatures: decision.DeclaredCaseFeatureVersion, ConditionFeatures: decision.DeclaredConditionFeatureVersion,
		Architecture: [7]int{OrderedFeatureDim, CaseDim, ConditionDim, 1, PoolDim, HiddenDim, 2},
		Bounds:       [3]int{MaxChoices, MaxCases, MaxConditions}, Interaction: orderedRequirementInteraction,
		EmptyConditions: orderedRequirementEmptyConditions, Activation: activation, Pooling: m.Pooling(), Weights: m.weights[:],
	})
}

func DecodeOrderedRequirementConditioned(raw []byte) (*OrderedRequirementModel, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded requirement artifact required")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var a orderedRequirementArtifact
	if err := d.Decode(&a); err != nil {
		return nil, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing requirement artifact")
	}
	if a.Schema != OrderedRequirementSchema || a.SourceFeatures != OrderedSourceFeatureVersion ||
		a.CaseFeatures != decision.DeclaredCaseFeatureVersion || a.ConditionFeatures != decision.DeclaredConditionFeatureVersion ||
		a.Architecture != [7]int{OrderedFeatureDim, CaseDim, ConditionDim, 1, PoolDim, HiddenDim, 2} ||
		a.Bounds != [3]int{MaxChoices, MaxCases, MaxConditions} || a.Interaction != orderedRequirementInteraction ||
		a.EmptyConditions != orderedRequirementEmptyConditions || a.Activation != activation || len(a.Weights) != OrderedRequirementParameterCount {
		return nil, errors.New("requirement artifact ABI differs")
	}
	var weights [OrderedRequirementParameterCount]float32
	copy(weights[:], a.Weights)
	return NewOrderedRequirementConditioned(weights, a.Pooling)
}

func (m *OrderedRequirementModel) Fingerprint() string {
	if m == nil {
		return ""
	}
	d := sha256.New()
	d.Write([]byte(OrderedRequirementSchema + "\x00" + OrderedSourceFeatureVersion + "\x00" +
		decision.DeclaredCaseFeatureVersion + "\x00" + decision.DeclaredConditionFeatureVersion + "\x00" +
		orderedRequirementInteraction + "\x00" + orderedRequirementEmptyConditions + "\x00" + activation + "\x00" + m.Pooling() + "\x00"))
	var raw [OrderedRequirementParameterCount * 4]byte
	for i, value := range m.weights {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(value))
	}
	d.Write(raw[:])
	return hex.EncodeToString(d.Sum(nil))
}
