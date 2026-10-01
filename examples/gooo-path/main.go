// gooo-path demonstrates local own-model ranking over a verified typed Gooo plan.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

// Encoding occurs once per prepared decision, outside the inference hot path.
func contextIntent(plan pathplan.Plan, choice pathplan.Choice) (string, error) {
	if strings.Contains(choice.Intent, "intent: ") {
		return "", errors.New("intent contains the reserved feature separator")
	}
	var buffer [decision.InputMaxBytes]byte
	at := 0
	appendPart := func(part string) bool {
		if len(part) > len(buffer)-at {
			return false
		}
		at += copy(buffer[at:], part)
		return true
	}
	for _, part := range []string{"gooo; activity=", plan.Base.Name, "; result=", string(plan.Base.ResultType),
		"; kind=", choice.Kind, "; target=", strconv.Itoa(choice.Target), "; legal="} {
		if !appendPart(part) {
			return "", errors.New("typed context exceeds model input bound")
		}
	}
	for _, option := range choice.Options {
		var indices [32]byte
		order := indices[:0]
		for _, index := range option.Order {
			// Validated root options contain at most 128 indices; preserve all or fail.
			if len(order) > len(indices)-5 {
				return "", errors.New("root order exceeds example context bound")
			}
			order = strconv.AppendInt(order, int64(index), 10)
			order = append(order, ',')
		}
		for _, part := range []string{option.Label, ":", option.Name, ":", strconv.FormatBool(option.Reverse), ":", string(order), "|"} {
			if !appendPart(part) {
				return "", errors.New("typed context exceeds model input bound")
			}
		}
	}
	if !appendPart("; intent: ") || !appendPart(choice.Intent) {
		return "", errors.New("combined context and intent exceed model input bound")
	}
	return string(buffer[:at]), nil
}

func execute(raw []byte, model *decision.Model) (map[string]any, error) {
	document, err := pathplan.DecodeDocument(raw)
	if err != nil {
		return nil, err
	}
	// Validate every option and combined fallback before constructing model context.
	original, err := document.Prepare()
	if err != nil {
		return nil, err
	}
	plan := document.Plan
	contextHashes := make([]string, len(plan.Decisions))
	if model != nil && model.FeatureVersion() == decision.SplitContextIntentFeatureVersion {
		plan.Decisions = append([]pathplan.Choice(nil), plan.Decisions...)
		for i, choice := range plan.Decisions {
			text, err := contextIntent(plan, choice)
			if err != nil {
				return nil, err
			}
			plan.Decisions[i].Intent = text
			sum := sha256.Sum256([]byte(text))
			contextHashes[i] = hex.EncodeToString(sum[:])
		}
	}
	prepared, err := pathplan.Prepare(plan)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	search, body, err := prepared.Search(ctx, model, document.TestCases, document.MaxAttempts, document.Seed)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, errors.New("search returned no typed body")
	}
	documentSum := sha256.Sum256(raw)
	return map[string]any{"schema": "gooo/own-model-typed-example/v1", "search": search, "gooo_body": body.GoooBody(),
		"input_document_sha256": hex.EncodeToString(documentSum[:]), "original_plan_sha256": original.PlanSHA256(),
		"context_format": "gooo/example-typed-path-context/v1", "context_applied": model != nil && model.FeatureVersion() == decision.SplitContextIntentFeatureVersion,
		"context_input_sha256": contextHashes, "native_compiler_calls": 0, "emitted_go_processes": 0,
		"scope": "Verified caller-supplied typed plan, finite cases and optional local model. Experimental context serialization is not the training text distribution. No source-file binding, native compiler execution or arbitrary body synthesis."}, nil
}

func main() {
	planFile := flag.String("plan", "", "bounded typed path document")
	modelFile := flag.String("model", "", "optional local structural model metadata")
	flag.Parse()
	info, err := os.Stat(*planFile)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 2<<20 || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "bounded regular typed plan required")
		os.Exit(1)
	}
	raw, err := os.ReadFile(*planFile)
	var model *decision.Model
	if err == nil && *modelFile != "" {
		model, err = decision.LoadPath(*modelFile)
	}
	var report map[string]any
	if err == nil {
		report, err = execute(raw, model)
	}
	if err == nil {
		err = json.NewEncoder(os.Stdout).Encode(report)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
