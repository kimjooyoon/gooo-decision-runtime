package decision

import "errors"

// RelationalFlowFeatureVersion adds source-proven equality/integer ordering to
// eligible v5 inputs without increasing their dimensions. It requires its own
// trained weights; model probability remains distinct from compiler evidence.
const RelationalFlowFeatureVersion = "source_intent_condition_output_relational_flow_v6"

// FlowRelation compares two static atoms in [then return, else return,
// predicate left, predicate right]. Pairs are 01,02,03,12,13,23 in that order.
// An unresolved relation is not evidence that the two values differ. Ordering
// applies only to integer literals. Two references to the sole input are equal;
// an input and a literal remain unresolved, even when a test input equals it.
type FlowRelation struct {
	Left  int    `json:"left"`
	Right int    `json:"right"`
	Kind  string `json:"kind"`
}

func BranchValueRelations(flow BranchValueFlow) ([6]FlowRelation, error) {
	var result [6]FlowRelation
	values := [4]FlowValue{flow.Returns[0], flow.Returns[1], flow.Predicate[0], flow.Predicate[1]}
	for _, v := range values {
		if err := validateFlowValue(v); err != nil {
			return result, err
		}
	}
	k := 0
	for i := range 4 {
		for j := i + 1; j < 4; j++ {
			a, b := values[i], values[j]
			kind := "unresolved"
			if a.Present && b.Present && a.Kind == b.Kind {
				switch {
				case a.Kind == "input", (a.Kind == "int" || a.Kind == "bool") && a.Int == b.Int:
					kind = "equal"
				case a.Kind == "int" && a.Int < b.Int:
					kind = "less"
				case a.Kind == "int" && a.Int > b.Int:
					kind = "greater"
				}
			}
			result[k] = FlowRelation{i, j, kind}
			k++
		}
	}
	return result, nil
}

// RelationalFlowFeaturesInto preserves a valid normalized v5 array except
// cells236:254 (six equality/less/greater one-hot groups) and marker255=3.
// The facts come only from flow; cases, intent and model outcomes are not read.
// Unsupported shapes are handled by the plan-level API, which retains v5 bytes.
// Errors leave the destination unchanged. Integer comparisons never subtract or
// convert a whole int64 to float, including at MinInt64 and MaxInt64.
func RelationalFlowFeaturesInto(original [ExecutionFlowFeatureDim]float32, flow BranchValueFlow,
	out *[ExecutionFlowFeatureDim]float32) error {
	if out == nil || original[255] != 2 {
		return errors.New("normalized v5 input and relation destination required")
	}
	var prefix [ExecutionFeatureDim]float32
	copy(prefix[:], original[:])
	var checked [ExecutionFlowFeatureDim]float32
	if err := ExecutionFlowFeaturesInto(prefix, flow, &checked); err != nil {
		return err
	}
	if checked != original {
		return errors.New("source flow does not match the original exact atom suffix")
	}
	relations, err := BranchValueRelations(flow)
	if err != nil {
		return err
	}
	next := original
	clear(next[236:255])
	for i, r := range relations {
		index := -1
		switch r.Kind {
		case "equal":
			index = 0
		case "less":
			index = 1
		case "greater":
			index = 2
		}
		if index >= 0 {
			next[236+i*3+index] = 1
		}
	}
	next[255] = 3
	*out = next
	return nil
}
