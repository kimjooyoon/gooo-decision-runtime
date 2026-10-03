package orderjudge

import (
	"errors"
	"math"
)

const Schema = "gooo/two-update-candidate-judge/v1"
const ArithmeticVersion = "float32_separate_v1"

// Model owns one intent-to-source matrix. Only New constructs runtime weights.
type Model struct{ weights [ParameterCount]float32 }

type Workspace struct{ projection [SourceDim]float32 }
type Prediction struct {
	Logits        [8]float32 `json:"logits"`
	Probabilities [8]float32 `json:"probabilities"`
	Ranking       [8]uint8   `json:"ranking"`
}

func New(weights [ParameterCount]float32) (*Model, error) {
	for _, w := range weights {
		if math.IsNaN(float64(w)) || math.IsInf(float64(w), 0) {
			return nil, errors.New("finite weights required")
		}
	}
	return &Model{weights: weights}, nil
}

// Predict uses complete candidate features, not separate per-choice heads.
// FP32 casts make product/add boundaries explicit. Output is committed only on
// success; the supplied workspace belongs to one caller and may be overwritten.
func (m *Model) Predict(intent *[IntentDim]float32, candidates *[8][SourceDim]float32, work *Workspace, out *Prediction) error {
	if m == nil || intent == nil || candidates == nil || work == nil || out == nil {
		return errors.New("model, input, workspace and output required")
	}
	for _, v := range intent {
		if !bounded(v) {
			return errors.New("invalid intent feature")
		}
	}
	for _, candidate := range candidates {
		for _, v := range candidate {
			if !bounded(v) {
				return errors.New("invalid source feature")
			}
		}
	}
	for j := range SourceDim {
		var sum float32
		for i, v := range intent {
			product := float32(v * m.weights[i*SourceDim+j])
			sum = float32(sum + product)
		}
		work.projection[j] = sum
	}
	var result Prediction
	for mask, candidate := range candidates {
		var sum float32
		for j, v := range candidate {
			product := float32(v * work.projection[j])
			sum = float32(sum + product)
		}
		if math.IsNaN(float64(sum)) || math.IsInf(float64(sum), 0) {
			return errors.New("nonfinite candidate score")
		}
		result.Logits[mask], result.Ranking[mask] = sum, uint8(mask)
	}
	for i := 1; i < 8; i++ {
		for j := i; j > 0 && result.Logits[result.Ranking[j]] > result.Logits[result.Ranking[j-1]]; j-- {
			result.Ranking[j], result.Ranking[j-1] = result.Ranking[j-1], result.Ranking[j]
		}
	}
	maximum := result.Logits[result.Ranking[0]]
	var mass float64
	for i, v := range result.Logits {
		result.Probabilities[i] = float32(math.Exp(float64(v) - float64(maximum)))
		mass += float64(result.Probabilities[i])
	}
	for i := range result.Probabilities {
		result.Probabilities[i] = float32(float64(result.Probabilities[i]) / mass)
	}
	*out = result
	return nil
}

func bounded(v float32) bool { return !math.IsNaN(float64(v)) && v >= -1 && v <= 1 }
