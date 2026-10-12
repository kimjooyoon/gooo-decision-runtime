package main

import (
	"context"
	"encoding/json"
	"io"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type relativeV2Choice struct {
	ID        string                                  `json:"id"`
	Options   [2]string                               `json:"options"`
	Features  *[528]float32                           `json:"source_relative_features,omitempty"`
	Source    pathplan.CanonicalOrderedBranchContext  `json:"canonical_context"`
	Relations pathplan.SourceRelativeConditionContext `json:"condition_relations"`
}
type relativeV2Inspection struct {
	Schema              string               `json:"schema"`
	PlanSHA             string               `json:"plan_sha256"`
	CaseSHA             string               `json:"output_cases_sha256"`
	SourceVersion       string               `json:"source_feature_version"`
	CaseVersion         string               `json:"output_feature_version"`
	ConditionVersion    string               `json:"condition_feature_version"`
	Choices             []relativeV2Choice   `json:"source_choices"`
	Outputs             []inspectedOutput    `json:"declared_outputs"`
	Conditions          []inspectedCondition `json:"declared_conditions"`
	ModelCalls          int                  `json:"model_calls"`
	CandidateExecutions int                  `json:"candidate_executions"`
	TrainingUpdates     int                  `json:"training_updates"`
	Scope               string               `json:"scope"`
}

func inspectRelativeV2(ctx context.Context, doc pathplan.Document, prepared *pathplan.PreparedPlan, out io.Writer) error {
	input, err := prepared.InitialContractInput(doc.TestCases)
	if err != nil {
		return err
	}
	r := relativeV2Inspection{Schema: "gooo/contract-input-inspection/v6", PlanSHA: input.PlanSHA256(), CaseSHA: input.CaseSHA256(),
		SourceVersion: contractdecision.SourceRelativeV2FeatureVersion, CaseVersion: decision.DeclaredCaseFeatureVersion,
		ConditionVersion: input.ConditionFeatureVersion(), Scope: "explicit V2 source and all declared output/Boolean rows; no prediction, candidate execution or training",
		Choices: make([]relativeV2Choice, 0, len(doc.Plan.Decisions)), Outputs: make([]inspectedOutput, 0, input.CaseCount()),
		Conditions: make([]inspectedCondition, 0, input.ConditionCount())}
	for _, choice := range doc.Plan.Decisions {
		if err = ctx.Err(); err != nil {
			return err
		}
		row := relativeV2Choice{ID: choice.ID, Options: [2]string{choice.Options[0].Label, choice.Options[1].Label}}
		row.Source, err = prepared.CanonicalOrderedBranchContextV2(choice.ID)
		if err != nil {
			return err
		}
		row.Relations, err = input.SourceRelativeV2Conditions(choice.ID)
		if err != nil {
			return err
		}
		if row.Source.Available && row.Relations.Available {
			row.Features = new([528]float32)
			if err = input.SourceRelativeV2FeaturesInto(choice.ID, row.Features); err != nil {
				return err
			}
		}
		r.Choices = append(r.Choices, row)
	}
	for i, c := range doc.TestCases {
		row := inspectedOutput{Case: c}
		if err = input.CaseFeaturesInto(i, &row.Features); err != nil {
			return err
		}
		r.Outputs = append(r.Outputs, row)
	}
	for i := range input.ConditionCount() {
		row := inspectedCondition{}
		row.Case, err = input.ConditionCase(i)
		if err != nil {
			return err
		}
		if err = input.ConditionFeaturesInto(i, &row.Features); err != nil {
			return err
		}
		r.Conditions = append(r.Conditions, row)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(r)
}
