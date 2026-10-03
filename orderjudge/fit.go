package orderjudge

import (
	"context"
	"errors"
	"math"
)

type Sample struct {
	Intent     [IntentDim]float32    `json:"intent_features"`
	Candidates [8][SourceDim]float32 `json:"candidate_features"`
	Acceptable uint8                 `json:"acceptable_mask_bits"`
}
type FitOptions struct {
	Epochs       int     `json:"epochs"`
	LearningRate float64 `json:"learning_rate"`
	L2           float64 `json:"l2"`
}
type Epoch struct {
	Number int     `json:"epoch"`
	Loss   float64 `json:"pre_update_mean_loss"`
}

// Fit uses fixed full-batch updates and multi-target probability-mass loss.
// Callers supply training samples only. No evaluation set is accepted here.
func Fit(ctx context.Context, samples []Sample, options FitOptions) (*Model, []Epoch, error) {
	if ctx == nil || len(samples) == 0 || len(samples) > 4096 || options.Epochs < 1 || options.Epochs > 10000 ||
		!finitePositive(options.LearningRate) || math.IsNaN(options.L2) || math.IsInf(options.L2, 0) || options.L2 < 0 {
		return nil, nil, errors.New("bounded training samples and finite fit options required")
	}
	m, _ := New([ParameterCount]float32{})
	var work Workspace
	var prediction Prediction
	for _, sample := range samples {
		if sample.Acceptable == 0 {
			return nil, nil, errors.New("every training sample needs an acceptable candidate")
		}
		if err := m.Predict(&sample.Intent, &sample.Candidates, &work, &prediction); err != nil {
			return nil, nil, err
		}
	}
	history := make([]Epoch, 0, options.Epochs)
	for epoch := range options.Epochs {
		if err := ctx.Err(); err != nil {
			return nil, history, err
		}
		var gradient [ParameterCount]float64
		loss := 0.0
		for _, sample := range samples {
			if err := m.Predict(&sample.Intent, &sample.Candidates, &work, &prediction); err != nil {
				return nil, history, err
			}
			mass := 0.0
			for mask, p := range prediction.Probabilities {
				if sample.Acceptable&(1<<mask) != 0 {
					mass += float64(p)
				}
			}
			if mass <= 0 {
				return nil, history, errors.New("acceptable probability mass underflowed")
			}
			loss -= math.Log(mass)
			var residual [SourceDim]float64
			for mask, p := range prediction.Probabilities {
				delta := float64(p)
				if sample.Acceptable&(1<<mask) != 0 {
					delta -= float64(p) / mass
				}
				for j, value := range sample.Candidates[mask] {
					residual[j] += delta * float64(value)
				}
			}
			for i, value := range sample.Intent {
				for j, delta := range residual {
					gradient[i*SourceDim+j] += float64(value) * delta
				}
			}
		}
		history = append(history, Epoch{epoch + 1, loss / float64(len(samples))})
		for i, w := range m.weights {
			updated := float64(w) - options.LearningRate*(gradient[i]/float64(len(samples))+options.L2*float64(w))
			if math.IsNaN(updated) || math.IsInf(updated, 0) || math.Abs(updated) > math.MaxFloat32 {
				return nil, history, errors.New("nonfinite training update")
			}
			m.weights[i] = float32(updated)
		}
	}
	return m, history, nil
}

func finitePositive(v float64) bool { return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }

// Weights returns an owned array for explicit publication, never mutable state.
func (m *Model) Weights() [ParameterCount]float32 { return m.weights }
