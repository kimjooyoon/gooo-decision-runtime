package pathplan

import (
	"errors"
	"math"
	"slices"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const SourceRelativeOutputFeatureVersion = decision.SourceRelativeOutputFeatureVersion
const SourceRelativeOutputFeatureDim = decision.SourceRelativeOutputFeatureDim

// Relations describe authored goals and source constants, never observed body
// outputs or candidate success. Arithmetic follows the body's int64 semantics.
type SourceRelativeOutputContext struct {
	Schema         string        `json:"schema"`
	FeatureVersion string        `json:"feature_version"`
	PlanSHA256     string        `json:"plan_sha256"`
	CaseSHA256     string        `json:"case_sha256"`
	ChoiceID       string        `json:"choice_id"`
	Available      bool          `json:"available"`
	Reason         string        `json:"reason"`
	CaseCount      int           `json:"case_count"`
	Literals       []int64       `json:"literals"`
	Counts         [3][16]uint16 `json:"relation_output_counts"`
	Features       [48]float32   `json:"features"`
}

// SourceRelativeOutputs groups every declared output row by the exact relation
// of the resolved predicate's operands: less, equal, greater. The literal set
// includes all distinct resolved Int atoms in predicate/returns, at most six.
// No program is assembled/evaluated and no model is read or called.
func (input *ContractInput) SourceRelativeOutputs(id string) (SourceRelativeOutputContext, error) {
	out := SourceRelativeOutputContext{Schema: "gooo/source-relative-output-context/v1",
		FeatureVersion: SourceRelativeOutputFeatureVersion, ChoiceID: id, Literals: []int64{}}
	if input == nil || input.initial.prepared == nil || input.count < 1 || input.count > 128 {
		return out, errors.New("bounded source and declared output contract required")
	}
	out.PlanSHA256, out.CaseSHA256, out.CaseCount = input.PlanSHA256(), input.CaseSHA256(), input.count
	if input.initial.prepared.plan.Base.ResultType != decision.TypeInt {
		out.Reason = "INTEGER_OUTPUT_REQUIRED"
		return out, nil
	}
	view, err := input.initial.prepared.CanonicalOrderedBranchContextV2(id)
	if err != nil {
		return out, err
	}
	out.Reason = view.Reason
	if !view.Available {
		return out, nil
	}
	predicate := view.Expressions[0]
	if predicate.Operation != "less_than" && predicate.Operation != "less_equal" && predicate.Operation != "equal" {
		out.Reason = "INTEGER_COMPARISON_ATOMS_REQUIRED"
		return out, nil
	}
	for _, atom := range predicate.Operands {
		if _, ok := relativeInteger(atom, 0); !ok {
			out.Reason = "INTEGER_COMPARISON_ATOMS_REQUIRED"
			return out, nil
		}
	}
	for _, expression := range view.Expressions {
		for _, atom := range expression.Operands {
			if atom.Kind == "int" && !slices.Contains(out.Literals, atom.Int) {
				out.Literals = append(out.Literals, atom.Int)
			}
		}
	}
	slices.Sort(out.Literals)
	for _, c := range input.cases[:input.count] {
		left, _ := relativeInteger(predicate.Operands[0], c.Input)
		right, _ := relativeInteger(predicate.Operands[1], c.Input)
		bucket := 1
		if left < right {
			bucket = 0
		} else if left > right {
			bucket = 2
		}
		counts := &out.Counts[bucket]
		counts[0]++
		for i, matches := range [3]bool{c.Expected == c.Input, c.Expected < c.Input, c.Expected > c.Input} {
			if matches {
				counts[1+i]++
			}
		}
		for _, literal := range out.Literals {
			flags := outputLiteralRelations(c.Input, c.Expected, literal)
			for i, matches := range flags {
				if matches {
					counts[4+i]++
				}
			}
		}
	}
	for bucket, counts := range out.Counts {
		for slot, count := range counts {
			denominator := input.count
			if slot >= 4 {
				denominator *= len(out.Literals)
			}
			if denominator != 0 {
				out.Features[bucket*16+slot] = float32(count) / float32(denominator)
			}
		}
	}
	out.Available, out.Reason = true, "SOURCE_RELATIVE_DECLARED_OUTPUTS"
	return out, nil
}

func outputLiteralRelations(input, expected, literal int64) [12]bool {
	addOK := !(literal > 0 && input > math.MaxInt64-literal || literal < 0 && input < math.MinInt64-literal)
	subOK := !(literal > 0 && input < math.MinInt64+literal || literal < 0 && input > math.MaxInt64+literal)
	reverseOK := !(input > 0 && literal < math.MinInt64+input || input < 0 && literal > math.MaxInt64+input)
	product := input * literal
	mulOK := !(input == math.MinInt64 && literal == -1 || literal == math.MinInt64 && input == -1)
	if input != 0 && mulOK {
		mulOK = product/input == literal
	}
	divideMatch, reverseDivideMatch := false, false
	if literal != 0 {
		divideMatch = expected == input/literal
	}
	if input != 0 {
		reverseDivideMatch = expected == literal/input
	}
	return [12]bool{expected == literal, expected == input+literal, expected == input-literal,
		expected == literal-input, expected == product, divideMatch, reverseDivideMatch,
		!addOK, !subOK || !reverseOK, !mulOK, literal == 0, input == 0}
}

// V3 keeps the V2 source/intent/condition/ordered-expression channels intact.
// Forty-eight previously reserved cells now hold the explicit output profile.
func (input *ContractInput) SourceRelativeV3FeaturesInto(id string, out *[528]float32) error {
	if out == nil {
		return errors.New("source-relative V3 destination required")
	}
	var next [528]float32
	if err := input.SourceRelativeV2FeaturesInto(id, &next); err != nil {
		return err
	}
	outputs, err := input.SourceRelativeOutputs(id)
	if err != nil {
		return err
	}
	if !outputs.Available {
		return errors.New("SOURCE_RELATIVE_OUTPUT_CONTEXT_UNAVAILABLE: " + outputs.Reason)
	}
	copy(next[208:256], outputs.Features[:])
	*out = next
	return nil
}
