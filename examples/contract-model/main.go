// contract-model trains and runs the bounded contract decision model locally.
// Input files are Gooo-derived typed path documents, never model-authored code.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func read(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errors.New("input exceeds byte budget")
	}
	return raw, nil
}

func document(path string) (pathplan.Document, *pathplan.PreparedPlan, error) {
	raw, err := read(path, 128<<10)
	if err != nil {
		return pathplan.Document{}, nil, err
	}
	d, err := pathplan.DecodeDocument(raw)
	if err != nil {
		return d, nil, err
	}
	if d.Seed != "" {
		return d, nil, errors.New("contract model currently uses deterministic score ordering; seed must be empty")
	}
	p, err := d.Prepare()
	return d, p, err
}

// Enumerate complete training candidates with the ordinary finite validator.
// The 64-candidate training cap is explicit; no prefix sampling drops labels.
func sample(ctx context.Context, d pathplan.Document, p *pathplan.PreparedPlan) (contractdecision.Sample, error) {
	return sampleFor(ctx, d, p, decision.DeclaredCaseFeatureVersion)
}

func sampleFor(ctx context.Context, d pathplan.Document, p *pathplan.PreparedPlan, caseVersion string) (contractdecision.Sample, error) {
	if len(d.Plan.Decisions) > 6 {
		return contractdecision.Sample{}, errors.New("training labels require at most six binary choices")
	}
	input, err := p.InitialContractInputFor(d.TestCases, caseVersion)
	if err != nil {
		return contractdecision.Sample{}, err
	}
	s := contractdecision.Sample{Inputs: make([][384]float32, len(d.Plan.Decisions)), Cases: input, Masks: make([]uint16, 1<<len(d.Plan.Decisions))}
	for i, c := range d.Plan.Decisions {
		if err := input.RelationalSourceFeaturesInto(c.ID, &s.Inputs[i]); err != nil {
			return s, err
		}
	}
	// Asking for an impossible extra case would change the source contract.
	// Instead evaluate each supplied complete selection directly.
	for mask := range s.Masks {
		if err := ctx.Err(); err != nil {
			return s, err
		}
		s.Masks[mask] = uint16(mask)
		choices := make(map[string]string, len(d.Plan.Decisions))
		for i, c := range d.Plan.Decisions {
			choices[c.ID] = c.Options[mask>>i&1].Label
		}
		body, err := p.Compile(choices)
		if err != nil {
			continue
		}
		observation, err := p.ObserveExecutionInput(ctx, choices, d.TestCases)
		if err != nil {
			return s, err
		}
		_, outputFailed := observation.OutputFailure()
		_, conditionFailed := observation.Failure()
		if body != nil && !outputFailed && !conditionFailed {
			s.Acceptable |= 1 << mask
		}
	}
	if s.Acceptable == 0 {
		return s, errors.New("training document has no complete source-valid candidate")
	}
	return s, nil
}

func fit(ctx context.Context, args []string, out io.Writer) error {
	f := flag.NewFlagSet("fit", flag.ContinueOnError)
	modelPath := f.String("out", "", "new model JSON path (must not exist)")
	epochs := f.Int("epochs", 400, "CPU full-batch epochs")
	pooling := f.String("pooling", contractdecision.MeanPooling, "arithmetic_mean or signed_max_abs")
	pairedGoals := f.Bool("goal-pairs", false, "pair consecutive training documents with opposite goals (weight 0.5, margin 2)")
	caseVersion := f.String("case-features", decision.DeclaredCaseFeatureVersion, "declared_integer_case_v1 or source_literal_integer_case_v1")
	choiceContext := f.Bool("choice-context", false, "condition the case encoder on each source choice before pooling")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *modelPath == "" || f.NArg() < 1 || f.NArg() > 256 {
		return errors.New("fit requires -out and 1..256 explicit training documents")
	}
	if *epochs < 1 || *epochs > 10000 {
		return errors.New("fit requires 1..10000 epochs")
	}
	if *pairedGoals && f.NArg()%2 != 0 {
		return errors.New("goal-pairs requires an even number of consecutive training documents")
	}
	if *caseVersion != decision.DeclaredCaseFeatureVersion && *caseVersion != decision.SourceLiteralCaseFeatureVersion {
		return errors.New("fit requires a supported case feature version")
	}
	if *pairedGoals && *caseVersion != decision.DeclaredCaseFeatureVersion {
		return errors.New("goal-pairs currently requires declared_integer_case_v1")
	}
	if *choiceContext && (*pairedGoals || *caseVersion != decision.DeclaredCaseFeatureVersion) {
		return errors.New("choice-context requires original declared cases and no goal-pairs")
	}
	if *pooling != contractdecision.MeanPooling && *pooling != contractdecision.ExtremePooling {
		return errors.New("fit pooling must be arithmetic_mean or signed_max_abs")
	}
	if _, err := os.Stat(*modelPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		return errors.New("model output must be a new file")
	}
	options := contractdecision.FitOptions{Epochs: *epochs, LearningRate: 0.3, L2: 0.0001, Seed: 17}
	samples := make([]contractdecision.Sample, 0, f.NArg())
	for _, path := range f.Args() {
		d, p, err := document(path)
		if err != nil {
			return err
		}
		s, err := sampleFor(ctx, d, p, *caseVersion)
		if err != nil {
			return fmt.Errorf("training document %s: %w", path, err)
		}
		samples = append(samples, s)
	}
	start := time.Now()
	model, history, pairInfo, err := fitSamples(ctx, samples, options, *pooling, *pairedGoals, *choiceContext, *caseVersion)
	elapsed := time.Since(start).Nanoseconds()
	if err != nil {
		return err
	}
	raw, err := model.Marshal()
	if err != nil {
		return err
	}
	file, err := os.OpenFile(*modelPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(raw)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	schema := "gooo/contract-local-fit/v1"
	if pairInfo != nil {
		schema = "gooo/contract-local-fit/v2"
	}
	explicitCaseVersion := ""
	if *choiceContext {
		schema, explicitCaseVersion = "gooo/choice-conditioned-local-fit/v1", decision.DeclaredCaseFeatureVersion
	}
	if *caseVersion != decision.DeclaredCaseFeatureVersion {
		schema, explicitCaseVersion = "gooo/contract-local-fit/v3", *caseVersion
	}
	return json.NewEncoder(out).Encode(struct {
		Schema            string                      `json:"schema"`
		TrainingDocuments int                         `json:"training_documents"`
		Options           contractdecision.FitOptions `json:"fit_options"`
		TrainingNS        int64                       `json:"training_ns"`
		ModelSHA          string                      `json:"model_sha256"`
		Fingerprint       string                      `json:"model_fingerprint"`
		History           []contractdecision.Epoch    `json:"history"`
		GoalPairs         *goalPairFit                `json:"goal_pairs,omitempty"`
		CaseFeatures      string                      `json:"case_feature_version,omitempty"`
	}{schema, len(samples), options, elapsed, fmt.Sprintf("%x", sha256.Sum256(raw)), model.Fingerprint(), history, pairInfo, explicitCaseVersion})
}

func search(ctx context.Context, args []string, out io.Writer) error {
	f := flag.NewFlagSet("search", flag.ContinueOnError)
	modelPath := f.String("model", "", "optional contract model JSON")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 1 {
		return errors.New("search requires one typed path document")
	}
	d, p, err := document(f.Arg(0))
	if err != nil {
		return err
	}
	var raw []byte
	if *modelPath != "" {
		raw, err = read(*modelPath, 512<<10)
		if err != nil {
			return err
		}
	}
	s, err := sourceContractSession(ctx, p, d.TestCases, raw)
	if err != nil {
		return err
	}
	initial, err := s.Observe()
	if err != nil {
		return err
	}
	progress, body, advanceErr := s.Advance(ctx, d.MaxAttempts)
	if advanceErr == nil && progress.Status != "TRAINING_COMPLETE" {
		advanceErr = errors.New("finite search budget ended before every declared requirement passed")
	}
	result := struct {
		Schema   string                    `json:"schema"`
		Ranking  pathplan.ContractRanking  `json:"ranking"`
		Initial  pathplan.ContractProgress `json:"initial"`
		Result   pathplan.ContractProgress `json:"result"`
		GoSource string                    `json:"selected_go_body,omitempty"`
		Error    string                    `json:"error,omitempty"`
	}{Schema: "gooo/contract-local-search/v1", Ranking: s.Ranking(), Initial: initial, Result: progress}
	if body != nil {
		result.GoSource = body.GoSource()
	}
	if advanceErr != nil {
		result.Error = advanceErr.Error()
	}
	if err := json.NewEncoder(out).Encode(result); err != nil {
		return err
	}
	return advanceErr
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("use fit or search")
	}
	switch args[0] {
	case "fit":
		return fit(ctx, args[1:], out)
	case "search":
		return search(ctx, args[1:], out)
	}
	return errors.New("unknown command; use fit or search")
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
