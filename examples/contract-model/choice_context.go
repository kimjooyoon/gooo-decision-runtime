package main

import (
	"context"
	"encoding/json"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func sourceContractSession(ctx context.Context, plan *pathplan.PreparedPlan, cases []pathplan.TestCase, raw []byte) (*pathplan.ContractSession, error) {
	if raw == nil {
		return plan.NewContractSession(ctx, nil, cases)
	}
	var header struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, err
	}
	if header.Schema == contractdecision.InteractionRequirementSchema {
		model, err := contractdecision.DecodeInteractionRequirementConditioned(raw)
		if err != nil {
			return nil, err
		}
		return plan.NewInteractionRequirementContractSession(ctx, model, cases)
	}
	if header.Schema == contractdecision.OrderedRequirementSchema {
		model, err := contractdecision.DecodeOrderedRequirementConditioned(raw)
		if err != nil {
			return nil, err
		}
		return plan.NewOrderedRequirementContractSession(ctx, model, cases)
	}
	if header.Schema == contractdecision.RequirementSchema {
		model, err := contractdecision.DecodeRequirementConditioned(raw)
		if err != nil {
			return nil, err
		}
		return plan.NewRequirementContractSession(ctx, model, cases)
	}
	if header.Schema == contractdecision.ChoiceSchema {
		model, err := contractdecision.DecodeChoiceConditioned(raw)
		if err != nil {
			return nil, err
		}
		return plan.NewChoiceContractSession(ctx, model, cases)
	}
	model, err := contractdecision.Decode(raw)
	if err != nil {
		return nil, err
	}
	return plan.NewContractSession(ctx, model, cases)
}
