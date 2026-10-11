package contractdecision

import (
	"context"
	"errors"
)

// The existing Go learner is reused with the explicit V2 input/artifact contract.
// Callers pass only training documents; no compiler or probe is invoked here.
func FitSourceRelativeV2(ctx context.Context, samples []InteractionRequirementSample,
	options FitOptions) (*SourceRelativeV2Model, []Epoch, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 || !validOptions(options) {
		return nil, nil, errors.New("bounded source-relative V2 training inputs required")
	}
	for _, sample := range samples {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if err := validateSourceRelativeV2(sample.Inputs, sample.Conditions); err != nil {
			return nil, nil, err
		}
	}
	model, history, err := FitInteractionRequirementConditioned(ctx, samples, options, MeanPooling)
	if err != nil {
		return nil, history, err
	}
	result, err := NewSourceRelativeV2(model.Weights())
	return result, history, err
}
