package jointdecision

import (
	"errors"
	"math"
)

// PredictRecordSharedInto projects complete source choices once, then applies
// the same judge independently to each ordered field. One request returns the
// eight composed mask scores; mutable scratch remains owned by the caller.
func (m *ThreeModel) PredictRecordSharedInto(text string, workspace *ThreeWorkspace, output *ThreePrediction) error {
	if m == nil || m.inner == nil || !m.shared || m.FeatureVersion() != RecordSharedFeatureVersion || workspace == nil || output == nil {
		return errors.New("shared record model/workspace/output required")
	}
	var features [ThreeFeatureDim]float32
	if err := FeaturesIntoRecordThree(text, &features); err != nil {
		return err
	}
	return m.PredictRecordSharedFeaturesInto(&features, workspace, output)
}

// PredictRecordSharedFeaturesInto supports a prepared array without text/AST
// parsing. Array identity/provenance belongs to the preparing caller. This ABI
// uses the unchanged FeaturesIntoRecordThree projection, including 1/sqrt(3).
func (m *ThreeModel) PredictRecordSharedFeaturesInto(features *[ThreeFeatureDim]float32, workspace *ThreeWorkspace, output *ThreePrediction) error {
	if m == nil || m.inner == nil || !m.shared || m.FeatureVersion() != RecordSharedFeatureVersion ||
		features == nil || workspace == nil || output == nil || !finite(features[:]) {
		return errors.New("finite prepared shared record model/features/workspace/output required")
	}
	var candidate ThreeWorkspace
	candidate.Features = *features
	m.first(&candidate.Features, &candidate.Hidden)
	m.last(&candidate.Hidden, &candidate.Logits)
	if !finite(candidate.Hidden[:]) || !finite(candidate.Logits[:]) {
		return errors.New("shared record prediction has nonfinite activations")
	}
	var prediction ThreePrediction
	prediction.Logits = candidate.Logits
	maximum := float64(candidate.Logits[0]) / float64(m.inner.temperature)
	for _, v := range candidate.Logits[1:] {
		maximum = math.Max(maximum, float64(v)/float64(m.inner.temperature))
	}
	var weights [ThreeLabelCount]float64
	total := 0.
	for i, v := range candidate.Logits {
		weights[i] = math.Exp(float64(v)/float64(m.inner.temperature) - maximum)
		total += weights[i]
	}
	for i, v := range weights {
		prediction.Probabilities[i] = float32(v / total)
		if prediction.Probabilities[i] > prediction.Probabilities[prediction.Mask] {
			prediction.Mask = uint16(i)
		}
	}
	*workspace, *output = candidate, prediction
	return nil
}

// RecordChoiceMarginals reads the probability assigned to each field's first
// and second choices. Finite execution determines completeness independently.
func RecordChoiceMarginals(prediction ThreePrediction) ([3][2]float64, error) {
	var result [3][2]float64
	sum := 0.
	for mask, p := range prediction.Probabilities {
		if math.IsNaN(float64(p)) || math.IsInf(float64(p), 0) || p < 0 || p > 1 {
			return [3][2]float64{}, errors.New("finite composed probabilities required")
		}
		sum += float64(p)
		for part := range result {
			result[part][(mask>>part)&1] += float64(p)
		}
	}
	if math.Abs(sum-1) > 1e-5 {
		return [3][2]float64{}, errors.New("normalized composed probabilities required")
	}
	return result, nil
}
