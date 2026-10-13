// Package criteriainput prepares source-only scalar construction packets for
// local decision models. It does not train, predict or evaluate requirements.
package criteriainput

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

const (
	Schema          = "gooo/scalar-criteria-source-input/v1"
	MaxBytes        = 128 << 10
	MaxPredicates   = 8
	MaxChoices      = 6
	MaxNodes        = MaxPredicates * jointdecision.RecordGraphNodeLimit
	MaxLiteralBytes = 64 << 10
)

// IDs and digests bind the source carrier; they are not numerical features.
// Candidate contains declared alternatives and intent. Finite condition cases,
// expected outputs, held inputs and observed outcomes are outside this schema.
type Input struct {
	Schema              string        `json:"schema"`
	SourceSHA256        string        `json:"source_sha256"`
	ContractSHA256      string        `json:"contract_sha256"`
	ActivityID          string        `json:"activity_id"`
	Candidate           pathplan.Plan `json:"candidate"`
	CandidatePlanSHA256 string        `json:"candidate_plan_sha256"`
	Predicates          []Predicate   `json:"predicates"`
}

type Predicate struct {
	ID          string                          `json:"id"`
	ActivityID  string                          `json:"activity_id"`
	GraphSHA256 string                          `json:"graph_sha256"`
	Inputs      [2]InputRole                    `json:"inputs"`
	Nodes       []jointdecision.RecordGraphNode `json:"nodes"`
}

type InputRole struct {
	Input uint16 `json:"input"`
	Role  string `json:"role"`
	Type  string `json:"type"`
}

// Decode accepts the complete packet, rejecting unknown fields and trailing
// data so an accidentally supplied outcome cannot silently enter the contract.
func Decode(raw []byte) (*Prepared, error) {
	if len(raw) == 0 || len(raw) > MaxBytes {
		return nil, errors.New("bounded complete scalar criteria input required")
	}
	var input Input
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("scalar criteria input contains trailing data")
	}
	canonical, err := json.Marshal(input)
	var compact bytes.Buffer
	if err != nil || json.Compact(&compact, raw) != nil || !bytes.Equal(compact.Bytes(), canonical) {
		return nil, errors.New("complete canonical scalar criteria packet required")
	}
	return Prepare(input)
}
