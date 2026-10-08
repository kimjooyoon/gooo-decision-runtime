package jointdecision

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/constant"
	"go/token"
	"strconv"
	"unicode/utf8"
)

const RecordGraphSharedFeatureVersion = "triple_record_value_graph_v3_shared_v1"
const RecordGraphInputMaxBytes = 64 << 10
const RecordGraphNodeLimit = 512
const recordGraphPrefix = "gooo;record-graph3-v3|"

// RecordGraphNode is a source relation, never an observed input or expected
// result. IDs are implicit one-based array positions; all edges point backward.
// Literal contains canonical source spelling; Input numbers root parameters.
type RecordGraphNode struct {
	Kind      string    `json:"kind"`
	Operator  string    `json:"operator,omitempty"`
	Literal   string    `json:"literal,omitempty"`
	Input     uint16    `json:"input,omitempty"`
	InputType string    `json:"input_type,omitempty"`
	FieldID   string    `json:"field_id,omitempty"`
	Parents   [2]uint16 `json:"parents"`
	Condition uint16    `json:"condition,omitempty"`
	Guard     uint16    `json:"guard,omitempty"`
}

type RecordGraphChoice struct {
	RecordChoice
	FieldID string    `json:"field_id"`
	Roots   [2]uint16 `json:"roots"`
}

// The compiler owns source provenance and converts its graph to this bounded
// contract. The SDK validates structure without executing the represented code.
type RecordGraphInput struct {
	Choices [3]RecordGraphChoice `json:"choices"`
	Nodes   []RecordGraphNode    `json:"nodes"`
}

func EncodeRecordGraphThree(input RecordGraphInput) (string, error) {
	if err := validateRecordGraph(input); err != nil {
		return "", err
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	if len(raw)+len(recordGraphPrefix) > RecordGraphInputMaxBytes {
		return "", errors.New("complete source graph exceeds byte bound")
	}
	return recordGraphPrefix + string(raw), nil
}

func DecodeRecordGraphThree(text string) (RecordGraphInput, error) {
	var input RecordGraphInput
	if len(text) > RecordGraphInputMaxBytes || !bytes.HasPrefix([]byte(text), []byte(recordGraphPrefix)) {
		return input, errors.New("canonical bounded record graph input required")
	}
	raw := []byte(text[len(recordGraphPrefix):])
	if err := json.Unmarshal(raw, &input); err != nil {
		return RecordGraphInput{}, err
	}
	canonical, err := EncodeRecordGraphThree(input)
	if err != nil || canonical != text {
		return RecordGraphInput{}, errors.New("complete canonical source graph required")
	}
	return input, nil
}

func validateRecordGraph(input RecordGraphInput) error {
	if len(input.Nodes) == 0 || len(input.Nodes) > RecordGraphNodeLimit {
		return errors.New("source graph requires 1..512 nodes")
	}
	for _, choice := range input.Choices {
		if !token.IsIdentifier(choice.Field) || choice.Intent == "" || choice.First == "" || choice.Second == "" || choice.FieldID == "" {
			return errors.New("complete field identity, expressions and intent required")
		}
		for _, value := range [5]string{choice.Field, choice.First, choice.Second, choice.Intent, choice.FieldID} {
			if !utf8.ValidString(value) || len(value) > 1024 {
				return errors.New("bounded UTF-8 source choice required")
			}
		}
		for _, root := range choice.Roots {
			if root == 0 || int(root) > len(input.Nodes) {
				return errors.New("source choice root outside graph")
			}
		}
	}
	for i, node := range input.Nodes {
		if err := validateGraphNode(node, uint16(i+1)); err != nil {
			return err
		}
	}
	return nil
}

func validateGraphNode(n RecordGraphNode, id uint16) error {
	for _, edge := range [4]uint16{n.Parents[0], n.Parents[1], n.Condition, n.Guard} {
		if edge >= id {
			return errors.New("source graph edges must precede their node")
		}
	}
	if !utf8.ValidString(n.FieldID) || len(n.FieldID) > 1024 || !utf8.ValidString(n.Literal) || len(n.Literal) > 1024 {
		return errors.New("bounded UTF-8 source node required")
	}
	if n.Kind != "literal" && n.Literal != "" || n.Kind != "input" && (n.Input != 0 || n.InputType != "") || n.Kind != "join" && n.Condition != 0 {
		return errors.New("source node contains an unrelated value or relation")
	}
	a, b := n.Parents[0], n.Parents[1]
	valid := false
	switch n.Kind {
	case "input":
		valid = n.Input > 0 && n.Input <= 16 && a == 0 && b == 0 && n.Guard == 0 && n.Operator == "" &&
			(n.InputType == "bool" || n.InputType == "int64" || n.InputType == "string")
	case "literal":
		valid = a == 0 && b == 0 && canonicalGraphLiteral(n.Operator, n.Literal)
	case "read", "copy", "write", "return", "call", "call_parameter":
		valid = a != 0 && b == 0 && n.Operator == ""
	case "expression":
		valid = a != 0 && graphExpressionArity(n.Operator, b != 0)
	case "choice":
		valid = a != 0 && b != 0 && n.Operator == ""
	case "join":
		valid = a != 0 && b != 0 && n.Condition != 0 && n.Operator == ""
	case "guard":
		valid = a != 0 && (n.Operator == "true" || n.Operator == "false")
	case "guard_join":
		valid = n.Operator == "or"
	}
	if !valid {
		return errors.New("source node kind, operator or arity unsupported")
	}
	return nil
}

func canonicalGraphLiteral(kind, text string) bool {
	switch kind {
	case "BOOL":
		return text == "true" || text == "false"
	case "STRING":
		value, err := strconv.Unquote(text)
		return err == nil && utf8.ValidString(value) && strconv.Quote(value) == text
	case "INT":
		value := constant.MakeFromLiteral(text, token.INT, 0)
		return value.Kind() == constant.Int && value.ExactString() == text
	}
	return false
}

func graphExpressionArity(op string, binary bool) bool {
	switch op {
	case "+", "-":
		return true
	case "!", "len", "int64":
		return !binary
	case "*", "/", "%", "==", "!=", "<", "<=", ">", ">=", "&&", "||", "slice_bounds", "slice":
		return binary
	}
	return false
}
