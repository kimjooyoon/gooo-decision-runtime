package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
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
	ID                 string                                         `json:"id"`
	Options            [2]string                                      `json:"options"`
	Features           [decision.ExecutionFlowFeatureDim]float32      `json:"features"`
	OrderedContext     *pathplan.OrderedBranchContext                 `json:"ordered_context,omitempty"`
	OrderedFeatures    *[decision.OrderedExpressionFeatureDim]float32 `json:"ordered_features,omitempty"`
	CanonicalContext   *pathplan.CanonicalOrderedBranchContext        `json:"canonical_context,omitempty"`
	CanonicalFeatures  *[contractdecision.OrderedFeatureDim]float32   `json:"canonical_features,omitempty"`
	ConditionRelations *pathplan.SourceRelativeConditionContext       `json:"condition_relations,omitempty"`
	RelativeFeatures   *[contractdecision.OrderedFeatureDim]float32   `json:"source_relative_features,omitempty"`
}

type contractInputInspection struct {
	Schema                 string                  `json:"schema"`
	PlanSHA256             string                  `json:"plan_sha256"`
	OutputCasesSHA256      string                  `json:"output_cases_sha256"`
	SourceFeatureFormat    string                  `json:"source_feature_version"`
	OutputFeatureFormat    string                  `json:"output_feature_version"`
	ConditionFeatureFormat string                  `json:"condition_feature_version"`
	OrderedFeatureFormat   string                  `json:"ordered_feature_version,omitempty"`
	CanonicalFeatureFormat string                  `json:"canonical_feature_version,omitempty"`
	RelativeFeatureFormat  string                  `json:"source_relative_feature_version,omitempty"`
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
	canonical := f.Bool("canonical-source", false, "include the complete canonical source input; no prediction or body execution")
	relations := f.Bool("condition-relations", false, "include source-relative declared condition counts; no model or candidate execution")
	relative := f.Bool("source-relative-source", false, "include the full source-relative model input and condition relations; no prediction or body execution")
	relativeV2 := f.Bool("source-relative-v2-source", false, "inspect the explicit V2 source input, including integer division; no prediction or execution")
	relativeV3 := f.Bool("source-relative-v3-source", false, "inspect explicit V3 input and declared output relations; no prediction or execution")
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
	if *relativeV3 {
		if *ordered || *canonical || *relations || *relative || *relativeV2 {
			return errors.New("source-relative-v3-source selects its own input ABI; use this format alone")
		}
		return inspectRelativeSource(ctx, doc, prepared, out, true)
	}
	if *relativeV2 {
		if *ordered || *canonical || *relations || *relative {
			return errors.New("source-relative-v2-source selects its own input ABI; use this format alone")
		}
		return inspectRelativeV2(ctx, doc, prepared, out)
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
	if *canonical {
		report.Schema = "gooo/contract-input-inspection/v3"
		report.CanonicalFeatureFormat = contractdecision.CanonicalOrderedSourceFeatureVersion
		report.Scope += "; canonical source input requires its own explicitly trained model"
	}
	if *relations {
		report.Schema = "gooo/contract-input-inspection/v4"
		report.Scope += "; source-relative condition features are versioned separately from canonical source arrays"
	}
	if *relative {
		report.Schema = "gooo/contract-input-inspection/v5"
		report.RelativeFeatureFormat = contractdecision.SourceRelativeOrderedSourceFeatureVersion
		report.Scope += "; source-relative input requires its matching research artifact schema"
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
		if *canonical {
			view, err := prepared.CanonicalOrderedBranchContext(choice.ID)
			if err != nil {
				return err
			}
			report.Choices[i].CanonicalContext = &view
			if view.Available {
				features := new([contractdecision.OrderedFeatureDim]float32)
				if err := input.CanonicalOrderedSourceFeaturesInto(choice.ID, features); err != nil {
					return err
				}
				report.Choices[i].CanonicalFeatures = features
			}
		}
		if *relations || *relative {
			view, err := input.SourceRelativeConditions(choice.ID)
			if err != nil {
				return err
			}
			report.Choices[i].ConditionRelations = &view
			if *relative && view.Available {
				features := new([contractdecision.OrderedFeatureDim]float32)
				if err := input.SourceRelativeOrderedSourceFeaturesInto(choice.ID, features); err != nil {
					return err
				}
				report.Choices[i].RelativeFeatures = features
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
