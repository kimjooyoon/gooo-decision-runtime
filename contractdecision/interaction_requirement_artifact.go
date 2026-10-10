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
	"slices"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const InteractionRequirementSchema = "gooo/interaction-requirement-contract/v1"
const interactionRequirementInteraction = "source_output_condition_elementwise_degree3_v1"
const interactionRequirementAssociation = "raw_plus_target_polarity_signed_input_bytes_scale8_v1"
const interactionRequirementActivation = "tanh_source_cases_leaky_relu_0.01_joint"
const interactionRequirementEmptyConditions = "zero_pool"
const interactionRequirementNumerics = "affine_fma64_round_fp32_v1"

type interactionRequirementArtifact struct {
	Schema            string    `json:"schema"`
	SourceFeatures    string    `json:"source_feature_version"`
	CaseFeatures      string    `json:"case_feature_version"`
	ConditionFeatures string    `json:"condition_feature_version"`
	Architecture      []int     `json:"architecture"`
	Bounds            []int     `json:"bounds"`
	Interaction       string    `json:"interaction"`
	Association       string    `json:"case_association"`
	InputScale        float32   `json:"input_scale"`
	EmptyConditions   string    `json:"empty_conditions"`
	Activation        string    `json:"activation"`
	Numerics          string    `json:"numerics"`
	Pooling           string    `json:"pooling"`
	Weights           []float32 `json:"weights_fp32"`
}

func (m *InteractionRequirementModel) ArtifactSchema() string { return InteractionRequirementSchema }

func (m *InteractionRequirementModel) Marshal() ([]byte, error) {
	if m == nil {
		return nil, errors.New("requirement-conditioned model required")
	}
	return json.Marshal(interactionRequirementArtifact{
		Schema: InteractionRequirementSchema, SourceFeatures: OrderedSourceFeatureVersion,
		CaseFeatures: decision.DeclaredCaseFeatureVersion, ConditionFeatures: decision.DeclaredConditionFeatureVersion,
		Architecture: []int{OrderedFeatureDim, CaseDim, ConditionDim, interactionCaseDim, interactionCaseDim + 1, PoolDim, interactionTermCount * PoolDim, interactionRequirementJointDim, HiddenDim, 2},
		Bounds:       []int{MaxChoices, MaxCases, MaxConditions}, Interaction: interactionRequirementInteraction, Association: interactionRequirementAssociation, InputScale: interactionInputScale,
		EmptyConditions: interactionRequirementEmptyConditions, Activation: interactionRequirementActivation, Numerics: interactionRequirementNumerics, Pooling: m.Pooling(), Weights: m.weights[:],
	})
}

func DecodeInteractionRequirementConditioned(raw []byte) (*InteractionRequirementModel, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded requirement artifact required")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var a interactionRequirementArtifact
	if err := d.Decode(&a); err != nil {
		return nil, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing requirement artifact")
	}
	if a.Schema != InteractionRequirementSchema || a.SourceFeatures != OrderedSourceFeatureVersion ||
		a.CaseFeatures != decision.DeclaredCaseFeatureVersion || a.ConditionFeatures != decision.DeclaredConditionFeatureVersion ||
		!slices.Equal(a.Architecture, []int{OrderedFeatureDim, CaseDim, ConditionDim, interactionCaseDim, interactionCaseDim + 1, PoolDim, interactionTermCount * PoolDim, interactionRequirementJointDim, HiddenDim, 2}) ||
		!slices.Equal(a.Bounds, []int{MaxChoices, MaxCases, MaxConditions}) || a.Interaction != interactionRequirementInteraction || a.Association != interactionRequirementAssociation || a.InputScale != interactionInputScale ||
		a.EmptyConditions != interactionRequirementEmptyConditions || a.Activation != interactionRequirementActivation || a.Numerics != interactionRequirementNumerics || len(a.Weights) != InteractionRequirementParameterCount {
		return nil, errors.New("requirement artifact ABI differs")
	}
	var weights [InteractionRequirementParameterCount]float32
	copy(weights[:], a.Weights)
	return NewInteractionRequirementConditioned(weights, a.Pooling)
}

func (m *InteractionRequirementModel) Fingerprint() string {
	if m == nil {
		return ""
	}
	d := sha256.New()
	d.Write([]byte(InteractionRequirementSchema + "\x00" + OrderedSourceFeatureVersion + "\x00" +
		decision.DeclaredCaseFeatureVersion + "\x00" + decision.DeclaredConditionFeatureVersion + "\x00" +
		interactionRequirementInteraction + "\x00" + interactionRequirementAssociation + "\x00" + interactionRequirementEmptyConditions + "\x00" + interactionRequirementActivation + "\x00" + interactionRequirementNumerics + "\x00" + m.Pooling() + "\x00"))
	var raw [InteractionRequirementParameterCount * 4]byte
	for i, value := range m.weights {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(value))
	}
	d.Write(raw[:])
	return hex.EncodeToString(d.Sum(nil))
}
