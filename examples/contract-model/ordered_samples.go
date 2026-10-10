package main

import (
	"context"
	"errors"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func orderedSample(ctx context.Context, doc pathplan.Document, p *pathplan.PreparedPlan) (contractdecision.OrderedRequirementSample, error) {
	var result contractdecision.OrderedRequirementSample
	if ctx == nil {
		return result, errors.New("ordered sample context required")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	input, err := p.InitialContractInput(doc.TestCases)
	if err != nil {
		return result, err
	}
	result.Inputs = make([][contractdecision.OrderedFeatureDim]float32, len(doc.Plan.Decisions))
	for i, choice := range doc.Plan.Decisions {
		if err := input.OrderedSourceFeaturesInto(choice.ID, &result.Inputs[i]); err != nil {
			return result, err
		}
	}
	// Enumerate labels only after the full source representation is available.
	base, err := sample(ctx, doc, p)
	if err != nil {
		return result, err
	}
	result.Cases, result.Conditions = input, input
	result.Masks, result.Acceptable = base.Masks, base.Acceptable
	return result, nil
}
