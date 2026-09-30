package decision

import (
	"errors"
	"fmt"
	"go/token"
)

const TypedBinaryIRSchema = "gooo/typed-binary-ir/v1"

type ValueType string

const (
	TypeInt  ValueType = "Int"
	TypeBool ValueType = "Bool"
)

type Identifier struct {
	Name string    `json:"name"`
	Type ValueType `json:"type"`
}

type TypedBinaryIR struct {
	Schema     string     `json:"schema"`
	Kind       string     `json:"kind"`
	Operation  string     `json:"operation"`
	Left       Identifier `json:"left"`
	Right      Identifier `json:"right"`
	ResultType ValueType  `json:"result_type"`
}

// BuildTypedBinary translates a classifier label to a closed, typed IR node.
// It never accepts or emits a source expression.
func BuildTypedBinary(operation string, left, right Identifier) (TypedBinaryIR, error) {
	if !validIdentifier(left.Name) || !validIdentifier(right.Name) {
		return TypedBinaryIR{}, errors.New("left and right must be valid identifier names")
	}
	if !validValueType(left.Type) || !validValueType(right.Type) {
		return TypedBinaryIR{}, errors.New("identifier types must be Int or Bool")
	}
	var result ValueType
	switch operation {
	case "add", "subtract", "multiply":
		if left.Type != TypeInt || right.Type != TypeInt {
			return TypedBinaryIR{}, fmt.Errorf("%s requires Int operands", operation)
		}
		result = TypeInt
	case "less_than", "less_equal":
		if left.Type != TypeInt || right.Type != TypeInt {
			return TypedBinaryIR{}, fmt.Errorf("%s requires Int operands", operation)
		}
		result = TypeBool
	case "equal":
		if left.Type != right.Type {
			return TypedBinaryIR{}, errors.New("equal requires operands with the same type")
		}
		result = TypeBool
	case "and", "or":
		if left.Type != TypeBool || right.Type != TypeBool {
			return TypedBinaryIR{}, fmt.Errorf("%s requires Bool operands", operation)
		}
		result = TypeBool
	default:
		return TypedBinaryIR{}, fmt.Errorf("unsupported binary operation %q", operation)
	}
	return TypedBinaryIR{
		Schema: TypedBinaryIRSchema, Kind: "binary", Operation: operation,
		Left: left, Right: right, ResultType: result,
	}, nil
}

// AssembleGoExpression renders the closed binary IR using only validated
// identifiers and a fixed operator table. It does not accept source fragments.
func AssembleGoExpression(ir TypedBinaryIR) (string, error) {
	if ir.Schema != TypedBinaryIRSchema || ir.Kind != "binary" {
		return "", errors.New("typed binary IR schema or kind is invalid")
	}
	validated, err := BuildTypedBinary(ir.Operation, ir.Left, ir.Right)
	if err != nil {
		return "", err
	}
	if ir.ResultType != validated.ResultType {
		return "", errors.New("typed binary IR result type is inconsistent")
	}
	var operator string
	switch ir.Operation {
	case "add":
		operator = "+"
	case "subtract":
		operator = "-"
	case "multiply":
		operator = "*"
	case "less_than":
		operator = "<"
	case "less_equal":
		operator = "<="
	case "equal":
		operator = "=="
	case "and":
		operator = "&&"
	case "or":
		operator = "||"
	default:
		return "", errors.New("unsupported typed binary operation")
	}
	return ir.Left.Name + " " + operator + " " + ir.Right.Name, nil
}

func validValueType(value ValueType) bool {
	return value == TypeInt || value == TypeBool
}

func validIdentifier(value string) bool {
	if value == "" || value == "_" || token.IsKeyword(value) {
		return false
	}
	for i := 0; i < len(value); i++ {
		b := value[i]
		if i == 0 {
			if !((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_') {
				return false
			}
			continue
		}
		if !((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_') {
			return false
		}
	}
	return true
}
