package contractdecision

import (
	"context"
	"errors"
	"math"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func initialChoiceContext(seed uint64) [ChoiceContextParameterCount]float32 {
	var weights [ChoiceContextParameterCount]float32
	state := seed ^ 0x9e3779b97f4a7c15
	if state == 0 {
		state = 1
	}
	for i := range weights {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		weights[i] = (float32(state>>40)/float32(1<<24) - .5) * .5
	}
	return weights
}

// FitChoiceConditioned trains source-to-case interactions and all existing
// parameters together. Candidate NLL and bounded CPU full-batch updates are
// unchanged. Each sample is captured once per epoch, then reused for every
// choice. Gradient calculation recomputes each choice's hidden state once.
func FitChoiceConditioned(ctx context.Context, samples []Sample, options FitOptions, pooling string) (*ChoiceModel, []Epoch, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 || !validOptions(options) {
		return nil, nil, errors.New("bounded choice-conditioned training inputs required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	for _, s := range samples {
		if err := validateCaseVersion(s.Cases, decision.DeclaredCaseFeatureVersion); err != nil {
			return nil, nil, err
		}
		if err := validate(s.Inputs, s.Cases, s.Masks); err != nil {
			return nil, nil, err
		}
		if s.Acceptable == 0 || s.Acceptable & ^allCandidates(len(s.Masks)) != 0 {
			return nil, nil, errors.New("acceptable set must name complete candidates")
		}
	}
	model, err := NewChoiceConditioned(initial(options.Seed), initialChoiceContext(options.Seed), pooling)
	if err != nil {
		return nil, nil, err
	}
	history := make([]Epoch, 0, options.Epochs)
	var work ChoiceWorkspace
	for epoch := range options.Epochs {
		var gradient [ChoiceParameterCount]float64
		loss := 0.0
		for _, sample := range samples {
			if err := ctx.Err(); err != nil {
				return nil, history, err
			}
			work = ChoiceWorkspace{}
			if err := work.cases.capture(sample.Cases); err != nil {
				return nil, history, err
			}
			if err := model.forward(sample.Inputs, sample.Masks, &work); err != nil {
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
			if err := model.backward(sample.Inputs, &work, &residual, &gradient); err != nil {
				return nil, history, err
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, history, err
		}
		history = append(history, Epoch{epoch + 1, loss / float64(len(samples))})
		for i, g := range gradient {
			var weight *float32
			if i < ParameterCount {
				weight = &model.base.weights[i]
			} else {
				weight = &model.context[i-ParameterCount]
			}
			value := float64(*weight) - options.LearningRate*(g/float64(len(samples))+options.L2*float64(*weight))
			if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > math.MaxFloat32 {
				return nil, history, errors.New("nonfinite choice-conditioned update")
			}
			*weight = float32(value)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, history, err
	}
	return model, history, nil
}

func (m *ChoiceModel) backward(inputs [][FeatureDim]float32, work *ChoiceWorkspace, residual *[MaxChoices][2]float64, gradient *[ChoiceParameterCount]float64) error {
	for choice := range inputs {
		if err := m.localize(&inputs[choice], &work.local); err != nil {
			return err
		}
		work.work = Workspace{}
		if err := work.local.forwardCaptured(&inputs[choice], &work.cases, &work.work); err != nil {
			return err
		}
		var localGradient [ParameterCount]float64
		var localResidual [MaxChoices][2]float64
		localResidual[0] = residual[choice]
		work.local.backward(inputs[choice:choice+1], &work.cases, &work.work, &localResidual, &localGradient)
		for i, value := range localGradient {
			gradient[i] += value
		}
		for h := range PoolDim {
			for j, value := range inputs[choice] {
				gradient[ParameterCount+h*FeatureDim+j] += localGradient[caseBias+h] * float64(value)
			}
		}
	}
	return nil
}
