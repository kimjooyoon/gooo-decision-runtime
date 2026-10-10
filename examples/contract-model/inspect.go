package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type inspectedOutput struct {
	Case     pathplan.TestCase                        `json:"case"`
	Features [decision.DeclaredCaseFeatureDim]float32 `json:"features"`
}

type inspectedCondition struct {
	Case     pathplan.ConditionCase                        `json:"case"`
	Features [decision.DeclaredConditionFeatureDim]float32 `json:"features"`
}

type inspectedSourceChoice struct {
	ID              string                                         `json:"id"`
	Options         [2]string                                      `json:"options"`
	Features        [decision.ExecutionFlowFeatureDim]float32      `json:"features"`
	OrderedContext  *pathplan.OrderedBranchContext                 `json:"ordered_context,omitempty"`
	OrderedFeatures *[decision.OrderedExpressionFeatureDim]float32 `json:"ordered_features,omitempty"`
}

type contractInputInspection struct {
	Schema                 string                  `json:"schema"`
	PlanSHA256             string                  `json:"plan_sha256"`
	OutputCasesSHA256      string                  `json:"output_cases_sha256"`
	SourceFeatureFormat    string                  `json:"source_feature_version"`
	OutputFeatureFormat    string                  `json:"output_feature_version"`
	ConditionFeatureFormat string                  `json:"condition_feature_version"`
	OrderedFeatureFormat   string                  `json:"ordered_feature_version,omitempty"`
	Choices                []inspectedSourceChoice `json:"source_choices"`
	Outputs                []inspectedOutput       `json:"declared_outputs"`
	Conditions             []inspectedCondition    `json:"declared_conditions"`
	ModelCalls             int                     `json:"model_calls"`
	CandidateExecutions    int                     `json:"candidate_executions"`
	TrainingUpdates        int                     `json:"training_updates"`
	Scope                  string                  `json:"scope"`
}

func inspectInput(ctx context.Context, args []string, out io.Writer) error {
	if ctx == nil {
		return errors.New("inspect requires context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f := flag.NewFlagSet("inspect", flag.ContinueOnError)
	ordered := f.Bool("ordered-source", false, "include ordered predicate and return expressions for input development")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 1 {
		return errors.New("inspect requires one Gooo-derived document; no model file is needed")
	}
	doc, prepared, err := document(f.Arg(0))
	if err != nil {
		return err
	}
	input, err := prepared.InitialContractInput(doc.TestCases)
	if err != nil {
		return err
	}
	report := contractInputInspection{
		Schema: "gooo/contract-input-inspection/v1", PlanSHA256: input.PlanSHA256(), OutputCasesSHA256: input.CaseSHA256(),
		SourceFeatureFormat: decision.RelationalFlowFeatureVersion, OutputFeatureFormat: decision.DeclaredCaseFeatureVersion,
		ConditionFeatureFormat: input.ConditionFeatureVersion(),
		Choices:                make([]inspectedSourceChoice, len(doc.Plan.Decisions)), Outputs: make([]inspectedOutput, input.CaseCount()),
		Conditions: make([]inspectedCondition, input.ConditionCount()),
		Scope:      "source-bound inputs only; requirement-conditioned artifacts consume all three channels; prediction requires an explicit compatible model",
	}
	if *ordered {
		report.Schema = "gooo/contract-input-inspection/v2"
		report.OrderedFeatureFormat = decision.OrderedExpressionFeatureVersion
		report.Scope += "; ordered source adjunct is available for a future separately trained consumer"
	}
	for i, choice := range doc.Plan.Decisions {
		report.Choices[i].ID, report.Choices[i].Options = choice.ID, [2]string{choice.Options[0].Label, choice.Options[1].Label}
		if err := input.RelationalSourceFeaturesInto(choice.ID, &report.Choices[i].Features); err != nil {
			return err
		}
		if *ordered {
			view, err := prepared.OrderedBranchContext(choice.ID)
			if err != nil {
				return err
			}
			report.Choices[i].OrderedContext = &view
			if view.Available {
				features := new([decision.OrderedExpressionFeatureDim]float32)
				if err := decision.OrderedExpressionFeaturesInto(view.Expressions, features); err != nil {
					return err
				}
				report.Choices[i].OrderedFeatures = features
			}
		}
	}
	for i, row := range doc.TestCases {
		report.Outputs[i].Case = row
		if err := input.CaseFeaturesInto(i, &report.Outputs[i].Features); err != nil {
			return err
		}
	}
	for i := range report.Conditions {
		if report.Conditions[i].Case, err = input.ConditionCase(i); err != nil {
			return err
		}
		if err := input.ConditionFeaturesInto(i, &report.Conditions[i].Features); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(report)
}
