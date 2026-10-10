package main

import (
	"errors"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func requirementSamples(samples []contractdecision.Sample) ([]contractdecision.RequirementSample, error) {
	result := make([]contractdecision.RequirementSample, len(samples))
	for i, sample := range samples {
		conditions, ok := sample.Cases.(contractdecision.ConditionSource)
		if !ok {
			return nil, errors.New("requirement fit needs source-bound declared condition readers")
		}
		result[i] = contractdecision.RequirementSample{Sample: sample, Conditions: conditions}
	}
	return result, nil
}
