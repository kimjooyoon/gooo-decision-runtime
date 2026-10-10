package contractdecision

import (
	"context"
	"errors"
	"math"
)

type RequirementSample struct {
	Sample
	Conditions ConditionSource
}

func initialRequirements(seed uint64) [RequirementParameterCount]float32 {
	var weights [RequirementParameterCount]float32
	state := seed
	if state == 0 {
		state = 1
	}
	for i := range weights {
		if i >= requirementOutputBias && i < requirementConditionStart ||
			i >= requirementConditionBias && i < requirementContextStart ||
			i >= requirementHiddenBias && i < requirementScoreStart || i >= requirementScoreBias {
			continue
		}
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		weights[i] = (float32(state>>40)/float32(1<<24) - .5) * .5
	}
	return weights
}

// FitRequirementConditioned learns both encoders, their shared source context
// and the joint decision layer from Gooo-validated complete candidate labels.
// Each sample's 128 output and 128 condition rows are captured once per epoch.
// Training retains bounded scratch instead of a dataset-sized feature tensor.
func FitRequirementConditioned(ctx context.Context, samples []RequirementSample, options FitOptions, pooling string) (*RequirementModel, []Epoch, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 || !validOptions(options) {
		return nil, nil, errors.New("bounded requirement training inputs required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	for _, sample := range samples {
		if err := validateRequirements(sample.Inputs, sample.Cases, sample.Conditions, sample.Masks); err != nil {
			return nil, nil, err
		}
		if sample.Acceptable == 0 || sample.Acceptable & ^allCandidates(len(sample.Masks)) != 0 {
			return nil, nil, errors.New("acceptable requirement set must name complete candidates")
		}
	}
	model, err := NewRequirementConditioned(initialRequirements(options.Seed), pooling)
	if err != nil {
		return nil, nil, err
	}
	history := make([]Epoch, 0, options.Epochs)
	var work RequirementWorkspace
	for epoch := range options.Epochs {
		var gradient [RequirementParameterCount]float64
		loss := 0.0
		for _, sample := range samples {
			if err := ctx.Err(); err != nil {
				return nil, history, err
			}
			work = RequirementWorkspace{}
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
