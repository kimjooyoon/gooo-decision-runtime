package contractdecision

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const TypedSourceRelativeSchema = "gooo/typed-source-relative-branch-model/v1"
const TypedSourceRelativeRouting = "canonical_operand_order_and_branch_layout/v1"

type typedRelativeMetadata struct {
	Schema             string `json:"schema"`
	SourceFeatures     string `json:"source_features"`
	OutputFeatures     string `json:"output_features"`
	ConditionFeatures  string `json:"condition_features"`
	ConditionRelations string `json:"condition_relation_features"`
	OutputRelations    string `json:"output_relation_features"`
	BranchRelations    string `json:"branch_output_relation_features"`
	IntentPolicy       string `json:"intent_policy"`
	Routing            string `json:"head_routing"`
	Pooling            string `json:"pooling"`
	PoolWidth          int    `json:"pool_width_per_head"`
	HiddenWidth        int    `json:"hidden_width_per_head"`
}
type typedRelativeArtifact struct {
	typedRelativeMetadata
	Weights [][]float32 `json:"weights"`
}

func typedRelativeContract() typedRelativeMetadata {
	return typedRelativeMetadata{
		Schema: TypedSourceRelativeSchema, SourceFeatures: SourceRelativeV4FeatureVersion,
		OutputFeatures: decision.DeclaredCaseFeatureVersion, ConditionFeatures: decision.DeclaredConditionFeatureVersion,
		ConditionRelations: decision.SourceRelativeConditionFeatureVersion, OutputRelations: decision.SourceRelativeOutputFeatureVersion,
		BranchRelations: decision.BranchOutputFeatureVersion, IntentPolicy: SourceRelativeV3ProseNeutralIntentPolicy,
		Routing: TypedSourceRelativeRouting, Pooling: MeanPooling, PoolWidth: typedPoolDim, HiddenWidth: typedHiddenDim,
	}
}
func (m *TypedSourceRelativeModel) Marshal() ([]byte, error) {
	if m == nil {
		return nil, errors.New("typed source model required")
	}
	return json.Marshal(typedRelativeArtifact{typedRelativeMetadata: typedRelativeContract(), Weights: [][]float32{m.weights[0][:], m.weights[1][:]}})
}
func (m *TypedSourceRelativeModel) bindFingerprint() error {
	raw, err := m.Marshal()
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	m.fingerprint = hex.EncodeToString(digest[:])
	return nil
}
func DecodeTypedSourceRelative(raw []byte) (*TypedSourceRelativeModel, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded typed source artifact required")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	var artifact typedRelativeArtifact
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&artifact); err != nil {
		return nil, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing typed source artifact")
	}
	if artifact.typedRelativeMetadata != typedRelativeContract() || len(artifact.Weights) != 2 {
		return nil, errors.New("typed source artifact contract differs")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	if bytes.Contains(fields["weights"], []byte("null")) {
		return nil, errors.New("typed source weights must be numbers")
	}
	var weights [2][TypedSourceRelativeHeadParameterCount]float32
	for i, values := range artifact.Weights {
		if len(values) != TypedSourceRelativeHeadParameterCount {
			return nil, errors.New("typed source weight count differs")
		}
		copy(weights[i][:], values)
	}
	return NewTypedSourceRelative(weights)
}
