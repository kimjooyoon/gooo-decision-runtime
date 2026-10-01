package jointdecision

import (
	"errors"
	"math"
)

type Workspace struct {
	Features [FeatureDim]float32
	Hidden   [HiddenDim]float32
	Logits   [LabelCount]float32
}
type Prediction struct {
	Logits        [LabelCount]float32 `json:"logits"`
	Probabilities [LabelCount]float32 `json:"probabilities"`
	Mask          uint16              `json:"proposed_mask"`
}
type Model struct {
	variant, metadataSHA, weightsSHA string
	temperature, w1Scale, w2Scale    float32
	floatWeights                     []float32
	codes                            []int8
	biases                           []float32
	packed                           int
}

func (m *Model) Schema() string         { return Schema }
func (m *Model) FeatureVersion() string { return FeatureVersion }
func (m *Model) Variant() string        { return m.variant }
func (m *Model) MetadataSHA256() string { return m.metadataSHA }
func (m *Model) WeightsSHA256() string  { return m.weightsSHA }
func (m *Model) PackedFileBytes() int   { return m.packed }
func (m *Model) ResidentTensorBytes() int {
	return len(m.floatWeights)*4 + len(m.codes) + len(m.biases)*4
}
func (m *Model) MatrixScaleBytes() int {
	if len(m.codes) > 0 {
		return 8
	}
	return 0
}

func (m *Model) first(x *[FeatureDim]float32, y *[HiddenDim]float32) {
	for row := range y {
		var sum float32
		if len(m.codes) == 0 {
			for col, v := range x {
				sum += v * m.floatWeights[row*FeatureDim+col]
			}
			sum += m.floatWeights[FeatureDim*HiddenDim+row]
		} else {
			for col, v := range x {
				sum += v * float32(m.codes[row*FeatureDim+col])
			}
			sum = sum*m.w1Scale + m.biases[row]
		}
		y[row] = max(sum, 0)
	}
}
func (m *Model) last(x *[HiddenDim]float32, y *[LabelCount]float32) {
	for row := range y {
		var sum float32
		if len(m.codes) == 0 {
			start := FeatureDim*HiddenDim + HiddenDim + row*HiddenDim
			for col, v := range x {
				sum += v * m.floatWeights[start+col]
			}
			sum += m.floatWeights[FeatureDim*HiddenDim+HiddenDim+HiddenDim*LabelCount+row]
		} else {
			start := FeatureDim*HiddenDim + row*HiddenDim
			for col, v := range x {
				sum += v * float32(m.codes[start+col])
			}
			sum = sum*m.w2Scale + m.biases[HiddenDim+row]
		}
		y[row] = sum
	}
}
func finite(values []float32) bool {
	for _, v := range values {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return false
		}
	}
	return true
}

// PredictInto uses caller-owned fixed arrays. Invalid/numerical failures leave
// both workspace and prediction unchanged; no model pointer is retained elsewhere.
func (m *Model) PredictInto(text string, workspace *Workspace, output *Prediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("joint model/workspace/output required")
	}
	var candidate Workspace
	if err := FeaturesInto(text, &candidate.Features); err != nil {
		return err
	}
	m.first(&candidate.Features, &candidate.Hidden)
	m.last(&candidate.Hidden, &candidate.Logits)
	if !finite(candidate.Hidden[:]) || !finite(candidate.Logits[:]) {
		return errors.New("joint prediction has nonfinite activations")
	}
	var prediction Prediction
	prediction.Logits = candidate.Logits
	maximum := float64(candidate.Logits[0]) / float64(m.temperature)
	for _, v := range candidate.Logits[1:] {
		maximum = math.Max(maximum, float64(v)/float64(m.temperature))
	}
	var weights [LabelCount]float64
	total := 0.0
	for i, v := range candidate.Logits {
		weights[i] = math.Exp(float64(v)/float64(m.temperature) - maximum)
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
