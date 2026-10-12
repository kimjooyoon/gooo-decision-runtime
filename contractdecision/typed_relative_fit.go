package contractdecision

import (
	"context"
	"errors"
	"math"
)

func initialTypedRelative(seed uint64) [2]typedRelativeHead {
	var weights [2]typedRelativeHead
	state := seed
	if state == 0 {
		state = 1
	}
	for head := range weights {
		for i := range weights[head] {
			if i >= typedOutputBias && i < typedConditionStart || i >= typedConditionBias && i < typedContextStart ||
				i >= typedContextBias && i < typedHiddenStart || i >= typedHiddenBias && i < typedScoreStart || i >= typedScoreBias {
				continue
			}
			state ^= state << 13
			state ^= state >> 7
			state ^= state << 17
			weights[head][i] = (float32(state>>40)/float32(1<<24) - .5) * .5
		}
	}
	return weights
}

// FitTypedSourceRelative uses complete-candidate labels, but routes each choice's
// gradient to its source kind's head. No weights are shared between the heads.
// Samples retain the complete source, output and condition inputs used by V4.
func FitTypedSourceRelative(ctx context.Context, samples []InteractionRequirementSample, options FitOptions) (*TypedSourceRelativeModel, []Epoch, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 || !validOptions(options) {
		return nil, nil, errors.New("bounded typed source fit inputs required")
	}
	for _, sample := range samples {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if err := validateTypedRelative(sample.Inputs, sample.Cases, sample.Conditions, sample.Masks); err != nil {
			return nil, nil, err
		}
		if sample.Acceptable == 0 || sample.Acceptable & ^allCandidates(len(sample.Masks)) != 0 {
			return nil, nil, errors.New("acceptable set must name complete candidates")
		}
	}
	m := &TypedSourceRelativeModel{weights: initialTypedRelative(options.Seed)}
	history := make([]Epoch, 0, options.Epochs)
	var work TypedSourceRelativeWorkspace
	var gradient [2][TypedSourceRelativeHeadParameterCount]float64
	for epoch := range options.Epochs {
		gradient = [2][TypedSourceRelativeHeadParameterCount]float64{}
		loss := 0.0
		for _, sample := range samples {
			if err := ctx.Err(); err != nil {
				return nil, history, err
			}
			work = TypedSourceRelativeWorkspace{}
			if err := work.capture(sample.Cases, sample.Conditions); err != nil {
				return nil, history, err
			}
			if err := m.forward(sample.Inputs, sample.Masks, &work); err != nil {
				return nil, history, err
			}
			all, a := distribution(work.scores[:len(sample.Masks)], allCandidates(len(sample.Masks)))
			good, b := distribution(work.scores[:len(sample.Masks)], sample.Acceptable)
			loss += a - b
			var residual [2][2]float64
			for i, mask := range sample.Masks {
				for choice := range sample.Inputs {
					residual[choice][mask>>choice&1] += all[i] - good[i]
				}
			}
			for choice, input := range sample.Inputs {
				route, err := typedRelativeRoute(input)
				if err != nil {
					return nil, history, err
				}
				m.weights[route].backward(choice, input, &work.cases, &work.choices[choice], residual[choice], &gradient[route])
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, history, err
		}
		history = append(history, Epoch{epoch + 1, loss / float64(len(samples))})
		for head := range m.weights {
			for i, value := range m.weights[head] {
				updated := float64(value) - options.LearningRate*(gradient[head][i]/float64(len(samples))+options.L2*float64(value))
				if math.IsNaN(updated) || math.IsInf(updated, 0) || math.Abs(updated) > math.MaxFloat32 {
					return nil, history, errors.New("nonfinite typed source update")
				}
				m.weights[head][i] = float32(updated)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, history, err
	}
	if err := m.bindFingerprint(); err != nil {
		return nil, history, err
	}
	return m, history, nil
}
