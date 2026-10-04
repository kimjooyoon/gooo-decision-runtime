package jointdecision

import (
	"errors"
	"math"
)

// LoadRecordThree loads only the record-specific feature contract. Its dense
// 768/24/8 tensor layout is unchanged; old integer weights are not accepted here.
func LoadRecordThree(name string) (*ThreeModel, error) {
	contract := threeContract
	contract.feature = RecordFieldFeatureVersion
	m, err := loadContract(name, contract)
	if err != nil {
		return nil, err
	}
	return &ThreeModel{inner: m, feature: RecordFieldFeatureVersion}, nil
}

// PredictRecordInto is the explicit record projection entry point. The frozen
// integer PredictInto entry point retains its original feature contract.
func (m *ThreeModel) PredictRecordInto(text string, workspace *ThreeWorkspace, output *ThreePrediction) error {
	if m == nil || m.inner == nil || m.FeatureVersion() != RecordFieldFeatureVersion || workspace == nil || output == nil {
		return errors.New("record model/workspace/output required")
	}
	var candidate ThreeWorkspace
	if err := FeaturesIntoRecordThree(text, &candidate.Features); err != nil {
		return err
	}
	m.first(&candidate.Features, &candidate.Hidden)
	m.last(&candidate.Hidden, &candidate.Logits)
	if !finite(candidate.Hidden[:]) || !finite(candidate.Logits[:]) {
		return errors.New("record prediction has nonfinite activations")
	}
	var prediction ThreePrediction
	prediction.Logits = candidate.Logits
	maximum := float64(candidate.Logits[0]) / float64(m.inner.temperature)
	for _, value := range candidate.Logits[1:] {
		maximum = math.Max(maximum, float64(value)/float64(m.inner.temperature))
	}
	var weights [ThreeLabelCount]float64
	var total float64
	for i, value := range candidate.Logits {
		weights[i] = math.Exp(float64(value)/float64(m.inner.temperature) - maximum)
		total += weights[i]
	}
	for i, value := range weights {
		prediction.Probabilities[i] = float32(value / total)
		if prediction.Probabilities[i] > prediction.Probabilities[prediction.Mask] {
			prediction.Mask = uint16(i)
		}
	}
	*workspace, *output = candidate, prediction
	return nil
}
