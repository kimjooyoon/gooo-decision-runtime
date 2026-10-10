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

const RequirementSchema = "gooo/requirement-conditioned-contract/v1"
const requirementInteraction = "source_conditioned_output_condition_pools_joint_hidden"
const requirementEmptyConditions = "zero_pool"

type requirementArtifact struct {
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

func (m *RequirementModel) ArtifactSchema() string { return RequirementSchema }

func (m *RequirementModel) Marshal() ([]byte, error) {
	if m == nil {
		return nil, errors.New("requirement-conditioned model required")
	}
	return json.Marshal(requirementArtifact{
		Schema: RequirementSchema, SourceFeatures: decision.RelationalFlowFeatureVersion,
		CaseFeatures: decision.DeclaredCaseFeatureVersion, ConditionFeatures: decision.DeclaredConditionFeatureVersion,
		Architecture: [7]int{FeatureDim, CaseDim, ConditionDim, 1, PoolDim, HiddenDim, 2},
		Bounds:       [3]int{MaxChoices, MaxCases, MaxConditions}, Interaction: requirementInteraction,
		EmptyConditions: requirementEmptyConditions, Activation: activation, Pooling: m.Pooling(), Weights: m.weights[:],
	})
}

func DecodeRequirementConditioned(raw []byte) (*RequirementModel, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded requirement artifact required")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var a requirementArtifact
	if err := d.Decode(&a); err != nil {
		return nil, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing requirement artifact")
	}
	if a.Schema != RequirementSchema || a.SourceFeatures != decision.RelationalFlowFeatureVersion ||
		a.CaseFeatures != decision.DeclaredCaseFeatureVersion || a.ConditionFeatures != decision.DeclaredConditionFeatureVersion ||
		a.Architecture != [7]int{FeatureDim, CaseDim, ConditionDim, 1, PoolDim, HiddenDim, 2} ||
		a.Bounds != [3]int{MaxChoices, MaxCases, MaxConditions} || a.Interaction != requirementInteraction ||
		a.EmptyConditions != requirementEmptyConditions || a.Activation != activation || len(a.Weights) != RequirementParameterCount {
		return nil, errors.New("requirement artifact ABI differs")
	}
	var weights [RequirementParameterCount]float32
	copy(weights[:], a.Weights)
	return NewRequirementConditioned(weights, a.Pooling)
}

func (m *RequirementModel) Fingerprint() string {
	if m == nil {
		return ""
	}
	d := sha256.New()
	d.Write([]byte(RequirementSchema + "\x00" + decision.RelationalFlowFeatureVersion + "\x00" +
		decision.DeclaredCaseFeatureVersion + "\x00" + decision.DeclaredConditionFeatureVersion + "\x00" +
		requirementInteraction + "\x00" + requirementEmptyConditions + "\x00" + activation + "\x00" + m.Pooling() + "\x00"))
	var raw [RequirementParameterCount * 4]byte
	for i, value := range m.weights {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(value))
	}
	d.Write(raw[:])
	return hex.EncodeToString(d.Sum(nil))
}
