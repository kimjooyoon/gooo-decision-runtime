package decision

import (
	"math"
	"testing"
)

func TestBranchRelationsKeepExactIntegersAndUnknownInputs(t *testing.T) {
	flow := BranchValueFlow{Returns: [2]FlowValue{{true, "int", 9007199254740993}, {true, "int", 9007199254740995}}, Predicate: [2]FlowValue{{true, "input", 0}, {true, "int", 9007199254740993}}}
	r, err := BranchValueRelations(flow)
	if err != nil || r[0].Kind != "less" || r[1].Kind != "unresolved" || r[2].Kind != "equal" || r[4].Kind != "greater" {
		t.Fatal(r, err)
	}
	flow.Returns = [2]FlowValue{{true, "int", math.MinInt64}, {true, "int", math.MaxInt64}}
	r, err = BranchValueRelations(flow)
	if err != nil || r[0].Kind != "less" {
		t.Fatal("overflow in ordering", r, err)
	}
	flow.Returns = [2]FlowValue{{true, "input", 0}, {true, "input", 0}}
	r, err = BranchValueRelations(flow)
	if err != nil || r[0].Kind != "equal" {
		t.Fatal("same source input", r, err)
	}
}

func TestBranchRelationsDoNotInventOrderOrCrossTypeEquality(t *testing.T) {
	flow := BranchValueFlow{Returns: [2]FlowValue{{true, "bool", 1}, {true, "int", 1}}, Predicate: [2]FlowValue{{true, "bool", 0}, {true, "unknown", 0}}}
	r, err := BranchValueRelations(flow)
	if err != nil {
		t.Fatal(err)
	}
	for _, relation := range r {
		if relation.Kind != "unresolved" {
			t.Fatal("invented relation", relation)
		}
	}
	flow.Returns[1] = FlowValue{true, "bool", 1}
	r, err = BranchValueRelations(flow)
	if err != nil || r[0].Kind != "equal" {
		t.Fatal(r, err)
	}
	flow.Returns[0] = FlowValue{true, "bool", 2}
	if _, err = BranchValueRelations(flow); err == nil {
		t.Fatal("invalid atom")
	}
}

func TestRelationalInputPreservesChannelsAndRejectsMismatchTransactionally(t *testing.T) {
	flow := BranchValueFlow{Returns: [2]FlowValue{{true, "input", 0}, {true, "int", 11}}, Predicate: [2]FlowValue{{true, "input", 0}, {true, "int", 11}}}
	var prefix [ExecutionFeatureDim]float32
	prefix[255] = 2
	prefix[64] = .5
	prefix[256] = 1
	var old, next [ExecutionFlowFeatureDim]float32
	if err := ExecutionFlowFeaturesInto(prefix, flow, &old); err != nil {
		t.Fatal(err)
	}
	if err := RelationalFlowFeaturesInto(old, flow, &next); err != nil {
		t.Fatal(err)
	}
	for i, v := range old {
		if (i < 236 || i >= 256) && next[i] != v {
			t.Fatal("changed preserved input", i)
		}
	}
	if next[239] != 1 || next[248] != 1 || next[255] != 3 || next[254] != 0 {
		t.Fatal("relation slots")
	}
	want := next
	flow.Returns[1].Int = 12
	if RelationalFlowFeaturesInto(old, flow, &next) == nil || next != want {
		t.Fatal("mismatched suffix changed output")
	}
	old[255] = 0
	if RelationalFlowFeaturesInto(old, flow, &next) == nil || next != want {
		t.Fatal("ineligible input changed output")
	}
}
