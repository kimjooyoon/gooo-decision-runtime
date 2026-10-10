package contractdecision

import (
	"context"
	"errors"
	"math"
)

type OrderedRequirementSample struct {
	Inputs     [][OrderedFeatureDim]float32
	Cases      CaseSource
	Masks      []uint16
	Acceptable uint64
	Conditions ConditionSource
}

func initialOrderedRequirements(seed uint64) [OrderedRequirementParameterCount]float32 {
	var weights [OrderedRequirementParameterCount]float32
	state := seed
	if state == 0 {
		state = 1
	}
	for i := range weights {
		if i >= orderedRequirementOutputBias && i < orderedRequirementConditionStart ||
			i >= orderedRequirementConditionBias && i < orderedRequirementContextStart ||
			i >= orderedRequirementHiddenBias && i < orderedRequirementScoreStart || i >= orderedRequirementScoreBias {
			continue
		}
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		weights[i] = (float32(state>>40)/float32(1<<24) - .5) * .5
	}
	return weights
}

// FitOrderedRequirementConditioned learns both encoders, their shared source context
// and the joint decision layer from Gooo-validated complete candidate labels.
// Each sample's 128 output and 128 condition rows are captured once per epoch.
// Training retains bounded scratch instead of a dataset-sized feature tensor.
func FitOrderedRequirementConditioned(ctx context.Context, samples []OrderedRequirementSample, options FitOptions, pooling string) (*OrderedRequirementModel, []Epoch, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 || !validOptions(options) {
		return nil, nil, errors.New("bounded requirement training inputs required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	for _, sample := range samples {
		if err := validateOrderedRequirements(sample.Inputs, sample.Cases, sample.Conditions, sample.Masks); err != nil {
			return nil, nil, err
		}
		if sample.Acceptable == 0 || sample.Acceptable & ^allCandidates(len(sample.Masks)) != 0 {
			return nil, nil, errors.New("acceptable requirement set must name complete candidates")
		}
	}
	model, err := NewOrderedRequirementConditioned(initialOrderedRequirements(options.Seed), pooling)
	if err != nil {
		return nil, nil, err
	}
	history := make([]Epoch, 0, options.Epochs)
	var work OrderedRequirementWorkspace
	for epoch := range options.Epochs {
		var gradient [OrderedRequirementParameterCount]float64
		loss := 0.0
		for _, sample := range samples {
			if err := ctx.Err(); err != nil {
				return nil, history, err
			}
			work = OrderedRequirementWorkspace{}
			if err := work.capture(sample.Cases, sample.Conditions); err != nil {
				return nil, history, err
			}
			if err := model.forwardRequirements(sample.Inputs, sample.Masks, &work); err != nil {
				return nil, history, err
			}
			all, a := distribution(work.scores[:len(sample.Masks)], allCandidates(len(sample.Masks)))
			good, b := distribution(work.scores[:len(sample.Masks)], sample.Acceptable)
			loss += a - b
			var residual [MaxChoices][2]float64
			for i, mask := range sample.Masks {
				for choice := range sample.Inputs {
					residual[choice][mask>>choice&1] += all[i] - good[i]
				}
			}
			model.backwardRequirements(sample.Inputs, &work, &residual, &gradient)
		}
		if err := ctx.Err(); err != nil {
			return nil, history, err
		}
		history = append(history, Epoch{epoch + 1, loss / float64(len(samples))})
		for i, value := range model.weights {
			updated := float64(value) - options.LearningRate*(gradient[i]/float64(len(samples))+options.L2*float64(value))
			if math.IsNaN(updated) || math.IsInf(updated, 0) || math.Abs(updated) > math.MaxFloat32 {
				return nil, history, errors.New("nonfinite requirement training update")
			}
			model.weights[i] = float32(updated)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, history, err
	}
	return model, history, nil
}
