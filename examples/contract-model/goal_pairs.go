package main

import (
	"context"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

type goalPairFit struct {
	Pairs   []contractdecision.GoalPair      `json:"pairs"`
	Options contractdecision.GoalPairOptions `json:"options"`
}

func fitSamples(ctx context.Context, samples []contractdecision.Sample, options contractdecision.FitOptions, pooling string, paired bool, caseVersion string) (*contractdecision.Model, []contractdecision.Epoch, *goalPairFit, error) {
	if !paired {
		model, history, err := contractdecision.FitForCaseFeatures(ctx, samples, options, pooling, caseVersion)
		return model, history, nil, err
	}
	info := &goalPairFit{Options: contractdecision.GoalPairOptions{Weight: .5, Margin: 2}}
	for i := 0; i+1 < len(samples); i += 2 {
		info.Pairs = append(info.Pairs, contractdecision.GoalPair{First: i, Second: i + 1})
	}
	model, history, err := contractdecision.FitWithGoalPairs(ctx, samples, options, pooling, info.Pairs, info.Options)
	return model, history, info, err
}
