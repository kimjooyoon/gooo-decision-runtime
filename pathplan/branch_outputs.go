package pathplan

import (
	"errors"
	"slices"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// BranchOutputContext keeps authored then/else literals separate. Counts compare
// declared input/output rows with each literal across all 12 arithmetic relations;
// they do not evaluate a selected return operation, candidate or held-out probe.
type BranchOutputContext struct {
	Schema         string           `json:"schema"`
	FeatureVersion string           `json:"feature_version"`
	PlanSHA256     string           `json:"plan_sha256"`
	CaseSHA256     string           `json:"case_sha256"`
	ChoiceID       string           `json:"choice_id"`
	Available      bool             `json:"available"`
	Reason         string           `json:"reason"`
	CaseCount      int              `json:"case_count"`
	Literals       [2][]int64       `json:"branch_literals"`
	Counts         [2][3][16]uint16 `json:"branch_relation_output_counts"`
	Features       [96]float32      `json:"features"`
}

func (input *ContractInput) BranchOutputs(id string) (BranchOutputContext, error) {
	out := BranchOutputContext{Schema: "gooo/branch-output-context/v1",
		FeatureVersion: decision.BranchOutputFeatureVersion, ChoiceID: id,
		Literals: [2][]int64{{}, {}}}
	// Reuse the exact V3 source/requirement support boundary and predicate regions.
	pooled, err := input.SourceRelativeOutputs(id)
	if err != nil {
		return out, err
	}
	out.PlanSHA256, out.CaseSHA256, out.CaseCount = pooled.PlanSHA256, pooled.CaseSHA256, pooled.CaseCount
	out.Reason = pooled.Reason
	if !pooled.Available {
		return out, nil
	}
	view, err := input.initial.prepared.CanonicalOrderedBranchContextV2(id)
	if err != nil {
		return out, err
	}
	for branch := range 2 {
		expression := view.Expressions[branch+1]
		atoms := expression.Operands[:]
		if expression.Operation == "value" {
			atoms = atoms[:1]
		}
		for _, atom := range atoms {
			if atom.Kind == "int" && !slices.Contains(out.Literals[branch], atom.Int) {
				out.Literals[branch] = append(out.Literals[branch], atom.Int)
			}
		}
		slices.Sort(out.Literals[branch])
		for bucket := range 3 {
			copy(out.Counts[branch][bucket][:4], pooled.Counts[bucket][:4])
		}
	}
	predicate := view.Expressions[0]
	for _, c := range input.cases[:input.count] {
		left, _ := relativeInteger(predicate.Operands[0], c.Input)
		right, _ := relativeInteger(predicate.Operands[1], c.Input)
		bucket := 1
		if left < right {
			bucket = 0
		} else if left > right {
			bucket = 2
		}
		for branch := range 2 {
			for _, literal := range out.Literals[branch] {
				for slot, matches := range outputLiteralRelations(c.Input, c.Expected, literal) {
					if matches {
						out.Counts[branch][bucket][4+slot]++
					}
				}
			}
		}
	}
	for branch, buckets := range out.Counts {
		for bucket, counts := range buckets {
			for slot, count := range counts {
				denominator := input.count
				if slot >= 4 {
					denominator *= len(out.Literals[branch])
				}
				if denominator > 0 {
					out.Features[branch*48+bucket*16+slot] = float32(count) / float32(denominator)
				}
			}
		}
	}
	out.Available, out.Reason = true, "SOURCE_BRANCH_OWNED_DECLARED_OUTPUTS"
	return out, nil
}

// V4 keeps the neutral V3 channels and fills reserved cells [256,352) with the
// explicit branch-owned relations. Destination mutation happens only on success.
func (input *ContractInput) SourceRelativeV4FeaturesInto(id string, out *[528]float32) error {
	if out == nil {
		return errors.New("source-relative V4 destination required")
	}
	var next [528]float32
	if err := input.SourceRelativeV3ProseNeutralFeaturesInto(id, &next); err != nil {
		return err
	}
	branches, err := input.BranchOutputs(id)
	if err != nil {
		return err
	}
	if !branches.Available {
		return errors.New("BRANCH_OUTPUT_CONTEXT_UNAVAILABLE: " + branches.Reason)
	}
	copy(next[256:352], branches.Features[:])
	*out = next
	return nil
}
