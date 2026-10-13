package criteriadecision

import (
	"errors"
	"math"
	"slices"

	"github.com/kimjooyoon/gooo-decision-runtime/criteriainput"
)

// PredictInto ranks every declared mask. Inputs are source numerical rows only;
// IDs and labels never enter the network; digests bind the receipt to its input
// and model. It executes no source program and leaves output unchanged on error.
func (m *Model) PredictInto(features *criteriainput.Features, work *Workspace, output *Prediction) error {
	if m == nil || m.metadataSHA256 == "" || work == nil || output == nil {
		return errors.New("loaded model, workspace and prediction output required")
	}
	if err := validateFeatures(features); err != nil {
		return err
	}
	featureSHA := featureDigest(features)
	clear(work.context[:])
	for _, row := range features.Predicates[:features.PredicateCount] {
		m.encode(row.Values[:], predicateWeights, predicateBias, &work.predicate)
		for i, value := range work.predicate {
			work.context[i] += value / float32(features.PredicateCount)
		}
	}
	for i, row := range features.Choices[:features.ChoiceCount] {
		m.encode(row.Values[:], choiceWeights, choiceBias, &work.choice[i])
		m.combine(&work.choice[i], &work.context, &work.combined[i])
	}
	result := Prediction{Schema: RankingSchema, FeatureSchema: criteriainput.FeatureSchema,
		InputSHA256: features.InputSHA256, CandidatePlanSHA256: features.CandidatePlanSHA256,
		FeatureSHA256:       featureSHA,
		ModelMetadataSHA256: m.metadataSHA256, ModelWeightsSHA256: m.metadata.WeightsSHA256,
		ChoiceCount: features.ChoiceCount, PredicateCount: features.PredicateCount,
		CandidateCount: 1 << features.ChoiceCount}
	for mask := range result.CandidateCount {
		result.Candidates[mask] = Candidate{Mask: uint16(mask), Score: m.energy(uint16(mask), int(features.ChoiceCount), work)}
	}
	slices.SortFunc(result.Candidates[:result.CandidateCount], func(a, b Candidate) int {
		if a.Score > b.Score || a.Score == b.Score && a.Mask < b.Mask {
			return -1
		}
		if a.Score < b.Score || a.Mask > b.Mask {
			return 1
		}
		return 0
	})
	result.ProposedMask = result.Candidates[0].Mask
	var total float64
	for i := range result.CandidateCount {
		result.Candidates[i].Probability = math.Exp(result.Candidates[i].Score - result.Candidates[0].Score)
		total += result.Candidates[i].Probability
	}
	for i := range result.CandidateCount {
		candidate := &result.Candidates[i]
		candidate.Probability /= total
		for choice := range result.ChoiceCount {
			result.Marginals[choice][candidate.Mask>>choice&1] += candidate.Probability
		}
	}
	*output = result
	return nil
}

func validateFeatures(features *criteriainput.Features) error {
	if features == nil || features.Schema != criteriainput.FeatureSchema ||
		features.ChoiceCount == 0 || int(features.ChoiceCount) > criteriainput.MaxChoices ||
		features.PredicateCount == 0 || int(features.PredicateCount) > criteriainput.MaxPredicates {
		return errors.New("complete scalar source feature rows required")
	}
	if !validDigest(features.InputSHA256, true) || !validDigest(features.CandidatePlanSHA256, false) {
		return errors.New("complete source and candidate plan bindings required")
	}
	for _, row := range features.Choices[:features.ChoiceCount] {
		if !finiteRow(row.Values[:]) {
			return errors.New("finite choice feature row required")
		}
	}
	for _, row := range features.Predicates[:features.PredicateCount] {
		if !finiteRow(row.Values[:]) {
			return errors.New("finite predicate feature row required")
		}
	}
	return nil
}

func finiteRow(values []float32) bool {
	for _, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return false
		}
	}
	return true
}
