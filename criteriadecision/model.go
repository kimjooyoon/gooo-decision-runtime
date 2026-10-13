// Package criteriadecision ranks Gooo's declared scalar construction paths using
// source-only numerical rows. Models are immutable; each concurrent call needs
// its own workspace. Source requirements retain their compiler-owned evaluation.
package criteriadecision

import "github.com/kimjooyoon/gooo-decision-runtime/criteriainput"

const (
	Schema         = "gooo/scalar-criteria-model/v1"
	RankingSchema  = "gooo/scalar-criteria-path-ranking/v1"
	Architecture   = "shared_rows_pair_energy_v1"
	HiddenDim      = 32
	ParameterCount = pairWeights + HiddenDim
	WeightBytes    = ParameterCount * 4
	MaxCandidates  = 1 << criteriainput.MaxChoices
)

const (
	choiceWeights    = 0
	choiceBias       = choiceWeights + HiddenDim*criteriainput.ChoiceFeatureDim
	predicateWeights = choiceBias + HiddenDim
	predicateBias    = predicateWeights + HiddenDim*criteriainput.PredicateFeatureDim
	mixWeights       = predicateBias + HiddenDim
	mixBias          = mixWeights + HiddenDim*2*HiddenDim
	unaryWeights     = mixBias + HiddenDim
	unaryBias        = unaryWeights + 2*HiddenDim
	pairWeights      = unaryBias + 2
)

type Metadata struct {
	Schema        string `json:"schema"`
	FeatureSchema string `json:"feature_schema"`
	Architecture  string `json:"architecture"`
	Variant       string `json:"variant"`
	HiddenDim     int    `json:"hidden_dim"`
	Parameters    int    `json:"parameters"`
	WeightsSHA256 string `json:"weights_sha256"`
}

type Model struct {
	weights        [ParameterCount]float32
	metadata       Metadata
	metadataSHA256 string
}

// Workspace retains only numerical scratch arrays. The model owns its weights;
// caller source rows and binding metadata remain unchanged during prediction.
type Workspace struct {
	choice    [criteriainput.MaxChoices][HiddenDim]float32
	combined  [criteriainput.MaxChoices][HiddenDim]float32
	predicate [HiddenDim]float32
	context   [HiddenDim]float32
}

type Candidate struct {
	Mask        uint16  `json:"mask"`
	Score       float64 `json:"score"`
	Probability float64 `json:"probability"`
}

type Prediction struct {
	Schema              string                               `json:"schema"`
	FeatureSchema       string                               `json:"feature_schema"`
	InputSHA256         string                               `json:"input_sha256"`
	CandidatePlanSHA256 string                               `json:"candidate_plan_sha256"`
	FeatureSHA256       string                               `json:"feature_sha256"`
	ModelMetadataSHA256 string                               `json:"model_metadata_sha256"`
	ModelWeightsSHA256  string                               `json:"model_weights_sha256"`
	ChoiceCount         uint8                                `json:"choice_count"`
	PredicateCount      uint8                                `json:"predicate_count"`
	CandidateCount      uint8                                `json:"candidate_count"`
	ProposedMask        uint16                               `json:"proposed_mask"`
	Marginals           [criteriainput.MaxChoices][2]float64 `json:"marginals"`
	Candidates          [MaxCandidates]Candidate             `json:"candidates"`
}

func (m *Model) Metadata() Metadata {
	if m == nil {
		return Metadata{}
	}
	return m.metadata
}

func (m *Model) MetadataSHA256() string {
	if m == nil {
		return ""
	}
	return m.metadataSHA256
}
