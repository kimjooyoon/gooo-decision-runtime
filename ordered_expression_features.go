package decision

import "errors"

const OrderedExpressionFeatureVersion = "ordered_predicate_return_expressions_v1"
const OrderedExpressionFeatureDim = 3 * 48

// OrderedExpression retains one operation and its ordered static operands.
// Operation "value" uses only the left operand. These are source expressions,
// including possible overflow, rather than evaluated or simplified results.
type OrderedExpression struct {
	Operation string       `json:"operation"`
	Operands  [2]FlowValue `json:"operands"`
}

var orderedOperations = [...]string{"value", "add", "subtract", "multiply", "less_than", "less_equal", "equal", "and", "or"}

// OrderedExpressionFeaturesInto encodes predicate, then return, else return.
// Each 48-cell block has nine operation flags, seven reserved zeros and two
// 16-cell exact atom encodings. The 576-byte adjunct has its own ABI; existing
// model inputs and weights do not consume it. Errors preserve the destination.
func OrderedExpressionFeaturesInto(expressions [3]OrderedExpression, out *[OrderedExpressionFeatureDim]float32) error {
	if out == nil {
		return errors.New("ordered expression destination required")
	}
	var next [OrderedExpressionFeatureDim]float32
	var types [3]string
	for i, expression := range expressions {
		typeName, operation, err := orderedExpressionType(expression)
		if err != nil {
			return err
		}
		types[i] = typeName
		block := next[i*48 : (i+1)*48]
		block[operation] = 1.0 / 8
		encodeFlowValue(expression.Operands[0], block[16:32])
		if expression.Operation != "value" {
			encodeFlowValue(expression.Operands[1], block[32:48])
		}
	}
	if types[0] != "bool" || types[1] != types[2] {
		return errors.New("Boolean predicate and matching return types required")
	}
	*out = next
	return nil
}

func orderedExpressionType(expression OrderedExpression) (string, int, error) {
	var types [2]string
	for i, atom := range expression.Operands {
		if err := validateFlowValue(atom); err != nil {
			return "", 0, err
		}
		if atom.Present && atom.Kind != "unknown" {
			types[i] = atom.Kind
			if atom.Kind == "input" {
				types[i] = "int"
			}
		}
	}
	for op, name := range orderedOperations {
		if name != expression.Operation || types[0] == "" {
			continue
		}
		switch {
		case op == 0 && expression.Operands[1] == (FlowValue{}):
			return types[0], op, nil
		case op >= 1 && op <= 3 && types == [2]string{"int", "int"}:
			return "int", op, nil
		case op >= 4 && op <= 5 && types == [2]string{"int", "int"}:
			return "bool", op, nil
		case op == 6 && types[0] == types[1]:
			return "bool", op, nil
		case op >= 7 && types == [2]string{"bool", "bool"}:
			return "bool", op, nil
		}
	}
	return "", 0, errors.New("known typed atoms and a supported ordered operation required")
}
