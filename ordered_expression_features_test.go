package decision

import (
	"math"
	"testing"
)

func TestOrderedExpressionExactAtomsTypesAndAtomicErrors(t *testing.T) {
	input := FlowValue{Present: true, Kind: "input"}
	for _, literal := range []int64{math.MinInt64, -9007199254740995, 0, 9007199254740993, math.MaxInt64} {
		integer := FlowValue{Present: true, Kind: "int", Int: literal}
		expressions := [3]OrderedExpression{
			{"less_than", [2]FlowValue{input, integer}},
			{"subtract", [2]FlowValue{input, integer}},
			{"subtract", [2]FlowValue{integer, input}},
		}
		var features [OrderedExpressionFeatureDim]float32
		if err := OrderedExpressionFeaturesInto(expressions, &features); err != nil {
			t.Fatal(err)
		}
		var bits uint64
		for _, cell := range features[40:48] {
			bits = bits<<8 | uint64(cell*2048)
		}
		if int64(bits) != literal || features[4] != 1.0/8 || features[48+2] != 1.0/8 {
			t.Fatal("exact integer or operator changed", literal)
		}
		before := features
		expressions[1], expressions[2] = expressions[2], expressions[1]
		if err := OrderedExpressionFeaturesInto(expressions, &features); err != nil || features == before {
			t.Fatal("return operand order collapsed", err)
		}
		for _, invalid := range []OrderedExpression{
			{}, {"divide", [2]FlowValue{input, integer}},
			{"and", [2]FlowValue{input, integer}},
			{"value", [2]FlowValue{input, integer}},
			{"subtract", [2]FlowValue{{Present: true, Kind: "unknown"}, integer}},
		} {
			bad := expressions
			bad[1] = invalid
			out := before
			if OrderedExpressionFeaturesInto(bad, &out) == nil || out != before {
				t.Fatal("invalid input changed destination", invalid)
			}
		}
		if OrderedExpressionFeaturesInto(expressions, nil) == nil {
			t.Fatal("nil destination accepted")
		}
	}
}

func TestOrderedExpressionClosedOperations(t *testing.T) {
	integer := FlowValue{Present: true, Kind: "int", Int: 1}
	boolean := FlowValue{Present: true, Kind: "bool", Int: 1}
	for _, op := range orderedOperations {
		value := integer
		if op == "and" || op == "or" {
			value = boolean
		}
		expression := OrderedExpression{op, [2]FlowValue{value, value}}
		if op == "value" {
			expression.Operands[1] = FlowValue{}
		}
		var out [OrderedExpressionFeatureDim]float32
		expressions := [3]OrderedExpression{{"value", [2]FlowValue{boolean}}, expression, expression}
		if err := OrderedExpressionFeaturesInto(expressions, &out); err != nil {
			t.Fatal(op, err)
		}
	}
}
