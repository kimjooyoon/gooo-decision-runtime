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

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

// Repackaging changes artifact metadata only. It makes no model or body calls.
func repackageProseNeutral(ctx context.Context, args []string, out io.Writer) error {
	if ctx == nil {
		return errors.New("repackaging requires context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f := flag.NewFlagSet("repackage-prose-neutral", flag.ContinueOnError)
	input := f.String("input", "", "original V3 artifact trained with intent cells zeroed")
	destination := f.String("out", "", "new explicit prose-neutral artifact; must not exist")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *input == "" || *destination == "" || f.NArg() != 0 {
		return errors.New("repackage-prose-neutral requires -input and -out")
	}
	raw, err := read(*input, 512<<10)
	if err != nil {
		return err
	}
	packed, err := contractdecision.RepackageSourceRelativeV3ProseNeutral(raw)
	if err != nil {
		return err
	}
	model, err := contractdecision.DecodeSourceRelativeV3ProseNeutral(packed)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := os.OpenFile(*destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(packed)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return json.NewEncoder(out).Encode(struct {
		Schema              string `json:"schema"`
		ModelSchema         string `json:"model_schema"`
		ModelSHA            string `json:"model_sha256"`
		SourceV3SHA         string `json:"source_v3_fingerprint"`
		InputSHA            string `json:"input_sha256"`
		IntentPolicy        string `json:"intent_policy"`
		ModelCalls          int    `json:"model_calls"`
		CandidateExecutions int    `json:"candidate_executions"`
		TrainingUpdates     int    `json:"training_updates"`
		Scope               string `json:"scope"`
	}{Schema: "gooo/prose-neutral-repackaging/v1", ModelSchema: model.ArtifactSchema(), ModelSHA: model.Fingerprint(),
		SourceV3SHA: model.SourceV3Fingerprint(), IntentPolicy: contractdecision.SourceRelativeV3ProseNeutralIntentPolicy,
		InputSHA: fmt.Sprintf("%x", sha256.Sum256(raw)),
		Scope:    "weights unchanged; only use weights trained with intent cells cleared; training history is not verified by this command"})
}
