package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type explainedChoice struct {
	ID          string                                `json:"id"`
	Kind        string                                `json:"kind"`
	Options     [2]string                             `json:"options"`
	Features    [contractdecision.FeatureDim]float32  `json:"source_features"`
	Computation contractdecision.ChoiceExplanationRow `json:"computation"`
}

type choiceExplanationReport struct {
	Schema              string              `json:"schema"`
	ModelSHA256         string              `json:"model_sha256"`
	ModelFingerprint    string              `json:"model_fingerprint"`
	PlanSHA256          string              `json:"plan_sha256"`
	CaseSHA256          string              `json:"case_sha256"`
	SourceFeatureFormat string              `json:"source_feature_version"`
	CaseFeatureFormat   string              `json:"case_feature_version"`
	Pooling             string              `json:"pooling"`
	DeclaredCases       []pathplan.TestCase `json:"declared_cases"`
	Choices             []explainedChoice   `json:"choices"`
	ModelCalls          int                 `json:"model_calls"`
	CandidateExecutions int                 `json:"candidate_executions"`
	EnumeratedMasks     int                 `json:"enumerated_candidate_masks"`
	TrainingUpdates     int                 `json:"training_updates"`
	PredictNS           int64               `json:"predict_ns"`
	Scope               string              `json:"scope"`
}

func explainChoice(ctx context.Context, args []string, out io.Writer) error {
	if ctx == nil {
		return errors.New("explain requires context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f := flag.NewFlagSet("explain", flag.ContinueOnError)
	name := f.String("model", "", "choice-conditioned model JSON")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *name == "" || f.NArg() != 1 {
		return errors.New("explain requires -model and one Gooo-derived document")
	}
	raw, err := read(*name, 1<<20)
	if err != nil {
		return err
	}
	m, err := contractdecision.DecodeChoiceConditioned(raw)
	if err != nil {
		return fmt.Errorf("explain requires a choice-conditioned contract artifact: %w", err)
	}
	doc, prepared, err := document(f.Arg(0))
	if err != nil {
		return err
	}
	input, err := prepared.InitialContractInput(doc.TestCases)
	if err != nil {
		return err
	}
	features := make([][contractdecision.FeatureDim]float32, len(doc.Plan.Decisions))
	for i, choice := range doc.Plan.Decisions {
		if err := input.RelationalSourceFeaturesInto(choice.ID, &features[i]); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var workspace contractdecision.ChoiceWorkspace
	var prediction contractdecision.ChoicePrediction
	var trace contractdecision.ChoiceExplanation
	start := time.Now()
	if err := m.ExplainChoicesInto(features, input, &workspace, &prediction, &trace); err != nil {
		return err
	}
	elapsed := time.Since(start).Nanoseconds()
	report := choiceExplanationReport{Schema: "gooo/choice-contract-explanation/v1",
		ModelSHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(raw)), ModelFingerprint: m.Fingerprint(),
		PlanSHA256: input.PlanSHA256(), CaseSHA256: input.CaseSHA256(), Pooling: trace.Pooling,
		SourceFeatureFormat: decision.RelationalFlowFeatureVersion, CaseFeatureFormat: m.CaseFeatureVersion(),
		DeclaredCases: doc.TestCases, ModelCalls: 1, PredictNS: elapsed,
		Scope: "one numerical decision pass over authored goals; Gooo output and condition checks remain required"}
	for i, choice := range doc.Plan.Decisions {
		report.Choices = append(report.Choices, explainedChoice{ID: choice.ID, Kind: string(choice.Kind),
			Options: [2]string{choice.Options[0].Label, choice.Options[1].Label}, Features: features[i], Computation: trace.Choices[i]})
	}
	return json.NewEncoder(out).Encode(report)
}
