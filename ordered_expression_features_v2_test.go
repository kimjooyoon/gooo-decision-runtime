package decision

import (
	"reflect"
	"testing"
)

func TestOrderedExpressionV2KeepsOldCellsAndSeparatesDivide(t *testing.T) {
	input := FlowValue{Present: true, Kind: "input"}
	constant := FlowValue{Present: true, Kind: "int", Int: 3}
	expressions := [3]OrderedExpression{{Operation: "less_than", Operands: [2]FlowValue{input, constant}},
		{Operation: "multiply", Operands: [2]FlowValue{input, constant}}, {Operation: "add", Operands: [2]FlowValue{input, constant}}}
	var v1, v2 [OrderedExpressionFeatureDim]float32
	if err := OrderedExpressionFeaturesInto(expressions, &v1); err != nil {
		t.Fatal(err)
	}
	if err := OrderedExpressionV2FeaturesInto(expressions, &v2); err != nil || v1 != v2 {
		t.Fatal("old cells changed", err)
	}
	expressions[1].Operation = "divide"
	before := v1
	if err := OrderedExpressionFeaturesInto(expressions, &v1); err == nil || v1 != before {
		t.Fatal("old ABI interpreted divide")
	}
	if err := OrderedExpressionV2FeaturesInto(expressions, &v2); err != nil {
		t.Fatal(err)
	}
	if v2[48+9] != 1.0/8 || v2[48+3] != 0 {
		t.Fatal("division cell differs")
	}
	if !reflect.DeepEqual(v2[48+16:48+48], before[48+16:48+48]) {
		t.Fatal("division operands changed")
	}
	expressions[1].Operands[1] = FlowValue{Present: true, Kind: "bool", Int: 1}
	beforeV2 := v2
	if err := OrderedExpressionV2FeaturesInto(expressions, &v2); err == nil || v2 != beforeV2 {
		t.Fatal("invalid division modified destination")
	}
}

func TestOrderedExpressionV2AllExistingOperationsKeepTheirCells(t *testing.T) {
	integer := FlowValue{Present: true, Kind: "int", Int: -7}
	boolean := FlowValue{Present: true, Kind: "bool", Int: 1}
	for _, operation := range orderedOperations {
		atom := integer
		if operation == "and" || operation == "or" {
			atom = boolean
		}
		e := OrderedExpression{Operation: operation, Operands: [2]FlowValue{atom, atom}}
		if operation == "value" {
			e.Operands[1] = FlowValue{}
		}
		expressions := [3]OrderedExpression{{Operation: "value", Operands: [2]FlowValue{boolean}}, e, e}
		var before, after [OrderedExpressionFeatureDim]float32
		if err := OrderedExpressionFeaturesInto(expressions, &before); err != nil {
			t.Fatal(operation, err)
		}
		if err := OrderedExpressionV2FeaturesInto(expressions, &after); err != nil || before != after {
			t.Fatal("existing operation cells changed", operation, err)
		}
	}
}
