package pathplan

import (
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const SourceRelativeConditionSchema = "gooo/source-relative-condition-context/v1"
const SourceRelativeConditionFeatureVersion = decision.SourceRelativeConditionFeatureVersion
const SourceRelativeConditionFeatureDim = 16

// SourceRelativeConditionContext relates declared Boolean goals to the resolved
// predicate operands. Buckets are [less/equal/greater][false/true], after exact
// int64 substitution. This is input preparation, not candidate acceptance.
type SourceRelativeConditionContext struct {
	Schema         string                                     `json:"schema"`
	FeatureVersion string                                     `json:"feature_version"`
	PlanSHA256     string                                     `json:"plan_sha256"`
	ChoiceID       string                                     `json:"choice_id"`
	Available      bool                                       `json:"available"`
	Reason         string                                     `json:"reason"`
	Count          int                                        `json:"condition_count"`
	Buckets        [3][2]uint16                               `json:"relation_goal_counts"`
	Features       [SourceRelativeConditionFeatureDim]float32 `json:"features"`
}

// SourceRelativeConditions uses the same immutable source/requirements snapshot
// as canonical inputs. Direct returns and resolved assignments share a view.
// One root branch with integer less_than/less_equal/equal atoms is supported.
// Conditions naming either structural choice of that same if are included;
// distinct rows sharing a relation remain counted, including equality and
// differing Boolean goals. Prepare retains its duplicate-input rejection.
// No body is compiled/evaluated and no model is loaded or called by this method.
func (input *ContractInput) SourceRelativeConditions(id string) (SourceRelativeConditionContext, error) {
	return input.sourceRelativeConditions(id, false)
}

func (input *ContractInput) SourceRelativeV2Conditions(id string) (SourceRelativeConditionContext, error) {
	return input.sourceRelativeConditions(id, true)
}

func (input *ContractInput) sourceRelativeConditions(id string, v2 bool) (SourceRelativeConditionContext, error) {
	out := SourceRelativeConditionContext{Schema: SourceRelativeConditionSchema,
		FeatureVersion: SourceRelativeConditionFeatureVersion, ChoiceID: id}
	if input == nil || input.initial.prepared == nil {
		return out, errors.New("source-bound declared contract required")
	}
	p := input.initial.prepared
	out.PlanSHA256 = p.sha
	view, err := p.canonicalOrderedBranchContext(id, v2)
	if err != nil {
		return out, err
	}
	out.Reason = view.Reason
	if !view.Available {
		return out, nil
	}
	predicate := view.Expressions[0]
	op := -1
	for i, name := range [3]string{"less_than", "less_equal", "equal"} {
		if predicate.Operation == name {
			op = i
		}
	}
	_, leftOK := relativeInteger(predicate.Operands[0], 0)
	_, rightOK := relativeInteger(predicate.Operands[1], 0)
	if op < 0 || !leftOK || !rightOK {
		out.Reason = "INTEGER_COMPARISON_ATOMS_REQUIRED"
		return out, nil
	}
	target, err := p.conditionTarget(id)
	if err != nil {
		return out, err
	}
	var buckets [3][2]uint16
	for _, condition := range p.plan.ConditionCases {
		observed, err := p.conditionTarget(condition.ChoiceID)
		if err != nil || observed != target {
			return out, errors.New("condition relation requires one source-bound if target")
		}
		left, _ := relativeInteger(predicate.Operands[0], condition.Input)
		right, _ := relativeInteger(predicate.Operands[1], condition.Input)
		relation := 1
		if left < right {
			relation = 0
		} else if left > right {
			relation = 2
		}
		goal := 0
		if condition.Expected {
			goal = 1
		}
		buckets[relation][goal]++
	}
	out.Available, out.Reason = true, "SOURCE_RELATIVE_DECLARED_CONDITIONS"
	out.Count, out.Buckets = len(p.plan.ConditionCases), buckets
	out.Features[1+op] = 1
	for _, choice := range p.plan.Decisions {
		if choice.ID != id {
			continue
		}
		kind := 4
		if choice.Kind == BranchLayout {
			kind = 5
		}
		out.Features[kind] = 1
		for i, option := range choice.Options {
			if option.Reverse != fallbackReverse(choice) {
				out.Features[6+i] = 1
			}
		}
	}
	if out.Count > 0 {
		out.Features[0] = 1
		out.Features[14] = float32(out.Count) / 128
		for relation, goals := range buckets {
			for goal, count := range goals {
				out.Features[8+2*relation+goal] = float32(count) / float32(out.Count)
			}
		}
	}
	return out, nil
}

// Only exact comparison follows substitution: no int64 subtraction, float
// conversion, arithmetic folding, candidate execution or expected-label choice.
func relativeInteger(atom decision.FlowValue, input int64) (int64, bool) {
	if !atom.Present {
		return 0, false
	}
	switch atom.Kind {
	case "input":
		return input, true
	case "int":
		return atom.Int, true
	default:
		return 0, false
	}
}
