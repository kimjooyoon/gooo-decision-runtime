package decision

import (
	"errors"
	"math"
)

// This is a representation ABI, not a trained model or an accuracy claim.
const ExecutionFlowFeatureVersion = "source_intent_condition_output_value_flow_v4"
const ExecutionFlowFeatureDim = ExecutionFeatureDim + 64

// FlowValue is a conservative static value. Present distinguishes an absent
// path/operand from an unresolved expression. Int is exact and Bool uses 0/1.
type FlowValue struct {
	Present bool   `json:"present"`
	Kind    string `json:"kind,omitempty"`
	Int     int64  `json:"int,omitempty"`
}

// BranchValueFlow assumes each syntactic arm of one branch in turn. It follows
// assignments through the containing body, including the continuation after
// that branch. It does not assert that either assumed arm is feasible at runtime.
type BranchValueFlow struct {
	Returns   [2]FlowValue `json:"returns"`
	Predicate [2]FlowValue `json:"predicate_operands"`
}

// ExecutionFlowFeaturesInto preserves all 320 v3 cells and appends four static
// value slots: then return, else return, predicate left, predicate right. Each
// slot has eight flags and eight exact integer bytes. No whole int64 is converted
// to floating point. Errors leave the destination unchanged.
func ExecutionFlowFeaturesInto(prefix [ExecutionFeatureDim]float32, flow BranchValueFlow, out *[ExecutionFlowFeatureDim]float32) error {
	if out == nil {
		return errors.New("flow feature destination required")
	}
	for _, x := range prefix {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return errors.New("finite execution prefix required")
		}
	}
	values := [4]FlowValue{flow.Returns[0], flow.Returns[1], flow.Predicate[0], flow.Predicate[1]}
	var next [ExecutionFlowFeatureDim]float32
	copy(next[:], prefix[:])
	for i, value := range values {
		if err := validateFlowValue(value); err != nil {
			return err
		}
		if value.Present {
			encodeFlowValue(value, next[ExecutionFeatureDim+i*16:ExecutionFeatureDim+(i+1)*16])
		}
	}
	*out = next
	return nil
}

func validateFlowValue(v FlowValue) error {
	if !v.Present {
		if v != (FlowValue{}) {
			return errors.New("absent flow value must have zero fields")
		}
		return nil
	}
	switch v.Kind {
	case "input", "unknown":
		if v.Int == 0 {
			return nil
		}
	case "int":
		return nil
	case "bool":
		if v.Int == 0 || v.Int == 1 {
			return nil
		}
	}
	return errors.New("closed static flow value required")
}

func encodeFlowValue(v FlowValue, fields []float32) {
	flags := [8]bool{true, v.Kind == "input", v.Kind == "int", v.Kind == "bool", v.Kind == "unknown",
		v.Kind == "int" && v.Int < 0, v.Kind == "int" && v.Int == 0, v.Kind == "int" && v.Int > 0}
	for i, set := range flags {
		if set {
			fields[i] = 1.0 / 8
		}
	}
	for i := range 8 {
		fields[8+i] = float32(byte(uint64(v.Int)>>uint(56-i*8))) / 2048
	}
}
