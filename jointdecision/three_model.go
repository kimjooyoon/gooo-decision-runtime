package jointdecision

import (
	"errors"
	"math"
)

type ThreeWorkspace struct {
	Features [ThreeFeatureDim]float32
	Hidden   [HiddenDim]float32
	Logits   [ThreeLabelCount]float32
}
type ThreePrediction struct {
	Logits        [ThreeLabelCount]float32 `json:"logits"`
	Probabilities [ThreeLabelCount]float32 `json:"probabilities"`
	Mask          uint16                   `json:"proposed_mask"`
}

// ThreeModel exposes a separate ABI and never exposes a v1 prediction view.
type ThreeModel struct {
	inner   *Model
	shared  bool
	feature string
}

var threeContract = modelContract{ThreeSchema, ThreeFeatureVersion, ThreeFeatureDim, ThreeLabelCount, ThreeInputMaxBytes, 128 << 10}

func LoadThree(name string) (*ThreeModel, error) {
	m, err := loadContract(name, threeContract)
	if err != nil {
		return nil, err
	}
	return &ThreeModel{inner: m}, nil
}
func (m *ThreeModel) Schema() string {
	if m != nil && m.shared {
		return SharedThreeSchema
	}
	return ThreeSchema
}
func (m *ThreeModel) FeatureVersion() string {
	if m != nil && m.feature != "" {
		return m.feature
	}
	return ThreeFeatureVersion
}
func (m *ThreeModel) Variant() string          { return m.inner.Variant() }
func (m *ThreeModel) MetadataSHA256() string   { return m.inner.MetadataSHA256() }
func (m *ThreeModel) WeightsSHA256() string    { return m.inner.WeightsSHA256() }
func (m *ThreeModel) PackedFileBytes() int     { return m.inner.PackedFileBytes() }
func (m *ThreeModel) ResidentTensorBytes() int { return m.inner.ResidentTensorBytes() }
func (m *ThreeModel) MatrixScaleBytes() int    { return m.inner.MatrixScaleBytes() }
func (m *ThreeModel) ArithmeticVersion() string {
	if m == nil || m.inner == nil {
		return ""
	}
	return m.inner.arithmetic
}

func (m *ThreeModel) first(x *[ThreeFeatureDim]float32, y *[HiddenDim]float32) {
	if m.inner.arithmetic == SeparateArithmeticVersion {
		m.separateFirst(x, y)
		return
	}
	if m.shared {
		m.sharedFirst(x, y)
		return
	}
	for row := range y {
		var sum float32
		if len(m.inner.codes) == 0 {
			for col, v := range x {
				sum += v * m.inner.floatWeights[row*ThreeFeatureDim+col]
			}
			sum += m.inner.floatWeights[ThreeFeatureDim*HiddenDim+row]
		} else {
			for col, v := range x {
				sum += v * float32(m.inner.codes[row*ThreeFeatureDim+col])
			}
			sum = sum*m.inner.w1Scale + m.inner.biases[row]
		}
		y[row] = max(sum, 0)
	}
}
func (m *ThreeModel) last(x *[HiddenDim]float32, y *[ThreeLabelCount]float32) {
	if m.inner.arithmetic == SeparateArithmeticVersion {
		m.separateLast(x, y)
		return
	}
	if m.shared {
		m.sharedLast(x, y)
		return
	}
	for row := range y {
		var sum float32
		if len(m.inner.codes) == 0 {
			start := ThreeFeatureDim*HiddenDim + HiddenDim + row*HiddenDim
			for col, v := range x {
				sum += v * m.inner.floatWeights[start+col]
			}
			sum += m.inner.floatWeights[ThreeFeatureDim*HiddenDim+HiddenDim+HiddenDim*ThreeLabelCount+row]
		} else {
			start := ThreeFeatureDim*HiddenDim + row*HiddenDim
			for col, v := range x {
				sum += v * float32(m.inner.codes[start+col])
			}
			sum = sum*m.inner.w2Scale + m.inner.biases[HiddenDim+row]
		}
		y[row] = sum
	}
}

func (m *ThreeModel) PredictInto(text string, workspace *ThreeWorkspace, output *ThreePrediction) error {
	if m == nil || m.inner == nil || workspace == nil || output == nil {
		return errors.New("three-choice model/workspace/output required")
	}
	var candidate ThreeWorkspace
	if err := threeFeatures(text, m.FeatureVersion(), &candidate.Features); err != nil {
		return err
	}
	m.first(&candidate.Features, &candidate.Hidden)
	m.last(&candidate.Hidden, &candidate.Logits)
	if !finite(candidate.Hidden[:]) || !finite(candidate.Logits[:]) {
		return errors.New("three-choice prediction has nonfinite activations")
	}
	var prediction ThreePrediction
	prediction.Logits = candidate.Logits
	maximum := float64(candidate.Logits[0]) / float64(m.inner.temperature)
	for _, v := range candidate.Logits[1:] {
		maximum = math.Max(maximum, float64(v)/float64(m.inner.temperature))
	}
	var weights [ThreeLabelCount]float64
	total := 0.0
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
