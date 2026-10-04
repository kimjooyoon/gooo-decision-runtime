package jointdecision

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strconv"
	"unicode/utf8"
)

// RecordFieldFeatureVersion uses actual string-field alternatives, separately
// from the integer semantic/ordinal contract. It requires its own trained file.
const RecordFieldFeatureVersion = "triple_record_field_context_v1_joint_v1"

type RecordChoice struct {
	Field  string `json:"field"`
	First  string `json:"first"`
	Second string `json:"second"`
	Intent string `json:"intent"`
}

// EncodeRecordThree preserves all expressions and intentions. Inputs and test
// expectations are not members of this context. Oversize inputs are rejected.
func EncodeRecordThree(choices [3]RecordChoice) (string, error) {
	var parts [3]string
	for i, choice := range choices {
		for _, value := range [4]string{choice.Field, choice.First, choice.Second, choice.Intent} {
			if !utf8.ValidString(value) {
				return "", errors.New("record choice contains invalid UTF-8")
			}
		}
		raw, err := json.Marshal(choice)
		if err != nil {
			return "", err
		}
		parts[i] = string(raw)
		var scratch [256]float32
		if err = recordFeatures(parts[i], &scratch); err != nil {
			return "", err
		}
	}
	var buffer [ThreeInputMaxBytes]byte
	n := copy(buffer[:], threePrefix)
	for _, part := range parts {
		length := strconv.AppendInt(buffer[n:n], int64(len(part)), 10)
		n += len(length)
		if n+1+len(part) > len(buffer) {
			return "", errors.New("complete record input exceeds byte bound")
		}
		buffer[n] = ':'
		n++
		n += copy(buffer[n:], part)
	}
	return string(buffer[:n]), nil
}

// FeaturesIntoRecordThree commits only after all complete source parts validate.
// Each part has 32 structural slots per alternative and 192 intent slots. Field
// names identify same-field selectors; renaming all references preserves them.
func FeaturesIntoRecordThree(text string, output *[ThreeFeatureDim]float32) error {
	if output == nil {
		return errors.New("record feature output required")
	}
	parts, err := ThreeParts(text)
	if err != nil {
		return err
	}
	var candidate [ThreeFeatureDim]float32
	var local [256]float32
	scale := float32(1 / math.Sqrt(3))
	for i, part := range parts {
		if err = recordFeatures(part, &local); err != nil {
			return err
		}
		for j, value := range local {
			candidate[i*256+j] = value * scale
		}
	}
	*output = candidate
	return nil
}

func recordFeatures(text string, output *[256]float32) error {
	if len(text) == 0 || len(text) > 512 || !utf8.ValidString(text) {
		return errors.New("complete record choice exceeds UTF-8 byte bound")
	}
	var choice RecordChoice
	if err := json.Unmarshal([]byte(text), &choice); err != nil {
		return err
	}
	raw, _ := json.Marshal(choice)
	if !bytes.Equal(raw, []byte(text)) || !token.IsIdentifier(choice.Field) || choice.Intent == "" {
		return errors.New("canonical complete field/alternatives/intent required")
	}
	var candidate [256]float32
	for i, expression := range [2]string{choice.First, choice.Second} {
		node, err := parser.ParseExpr(expression)
		if err != nil {
			return err
		}
		recordExpression(node, choice.Field, candidate[i*32:(i+1)*32])
	}
	for _, width := range [2]int{2, 3} {
		for start := 0; start+width <= len(choice.Intent); start++ {
			candidate[64+int(recordNgramHash(choice.Intent, start, width)%192)]++
		}
	}
	// Equal normalized source/intent channels retain candidate order without
	// allowing expression length to dominate the instruction representation.
	active := 0
	if normalizeRecord(candidate[:64]) {
		active++
	}
	if normalizeRecord(candidate[64:]) {
		active++
	}
	for i := range candidate {
		candidate[i] *= float32(1 / math.Sqrt(float64(active)))
	}
	*output = candidate
	return nil
}

func recordExpression(node ast.Expr, field string, output []float32) {
	switch expression := node.(type) {
	case *ast.ParenExpr:
		recordExpression(expression.X, field, output)
	case *ast.BasicLit:
		output[0]++
		if expression.Kind == token.STRING {
			value, _ := strconv.Unquote(expression.Value)
			for _, width := range [2]int{2, 3} {
				for start := 0; start+width <= len(value); start++ {
					output[8+int(recordNgramHash(value, start, width)%16)]++
				}
			}
		}
	case *ast.SelectorExpr:
		output[1]++
		if expression.Sel.Name == field {
			output[2]++
		} else {
			output[3]++
		}
	case *ast.Ident:
		output[4]++
	case *ast.BinaryExpr:
		output[5]++
		if expression.Op != token.ADD {
			output[6]++
		}
		// Ordered child kinds distinguish prefixing from suffixing a field.
		output[24+recordRootKind(expression.X)]++
		output[28+recordRootKind(expression.Y)]++
		recordExpression(expression.X, field, output)
		recordExpression(expression.Y, field, output)
	default:
		output[7]++
	}
}

func recordRootKind(node ast.Expr) int {
	switch expression := node.(type) {
	case *ast.ParenExpr:
		return recordRootKind(expression.X)
	case *ast.BasicLit:
		return 0
	case *ast.SelectorExpr:
		return 1
	case *ast.Ident:
		return 2
	default:
		return 3
	}
}

func normalizeRecord(values []float32) bool {
	var squared float64
	for _, value := range values {
		squared += float64(value) * float64(value)
	}
	if squared == 0 {
		return false
	}
	scale := float32(1 / math.Sqrt(squared))
	for i := range values {
		values[i] *= scale
	}
	return true
}

func recordNgramHash(text string, start, width int) uint32 {
	hash := uint32(2166136261)
	for _, value := range []byte(text[start : start+width]) {
		if value >= 'A' && value <= 'Z' {
			value += 'a' - 'A'
		}
		hash = (hash ^ uint32(value)) * 16777619
	}
	return hash
}
