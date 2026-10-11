package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func writeJSON(name string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(name, append(raw, '\n'), 0644)
}
func cpuNS(r syscall.Rusage) int64 { return r.Utime.Nano() + r.Stime.Nano() }
func run() error {
	if len(os.Args) != 3 {
		return fmt.Errorf("usage: train CORPUS NEW_OUTPUT_DIRECTORY")
	}
	rows, digest, err := loadCorpus(os.Args[1])
	if err != nil {
		return err
	}
	// Refuse an existing output: repeated fits require an explicit new protocol.
	if err = os.Mkdir(os.Args[2], 0755); err != nil {
		return err
	}
	train := make([]contractdecision.InteractionRequirementSample, 0, 60)
	for _, r := range rows {
		if r.Split == "train" {
			req := requirements{r.Outputs, r.Conditions}
			train = append(train, contractdecision.InteractionRequirementSample{Inputs: r.Inputs, Cases: req, Conditions: req, Masks: []uint16{0, 1, 2, 3}, Acceptable: r.Acceptable})
		}
	}
	options := contractdecision.FitOptions{Seed: 20261012, Epochs: 100, LearningRate: 0.01, L2: 0.00001}
	if err = writeJSON(filepath.Join(os.Args[2], "fit-started.json"), map[string]any{"corpus_sha256": digest, "training_documents": len(train), "options": options}); err != nil {
		return err
	}
	var before, after syscall.Rusage
	if err = syscall.Getrusage(syscall.RUSAGE_SELF, &before); err != nil {
		return err
	}
	start := time.Now()
	model, history, err := contractdecision.FitSourceRelativeV2(context.Background(), train, options)
	fitNS := time.Since(start).Nanoseconds()
	if err != nil {
		return err
	}
	if err = syscall.Getrusage(syscall.RUSAGE_SELF, &after); err != nil {
		return err
	}
	artifact, err := model.Marshal()
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(os.Args[2], "model.json"), artifact, 0644); err != nil {
		return err
	}
	if err = writeJSON(filepath.Join(os.Args[2], "history.json"), history); err != nil {
		return err
	}
	observations, splits, err := evaluate(context.Background(), model, rows)
	if err != nil {
		return err
	}
	if err = writeJSON(filepath.Join(os.Args[2], "observations.json"), observations); err != nil {
		return err
	}
	report := map[string]any{"schema": "gooo/return-layout-learning-observation/v1", "corpus_sha256": digest,
		"model_fingerprint": model.Fingerprint(), "model_schema": model.ArtifactSchema(), "feature_version": model.SourceFeatureVersion(),
		"options": options, "fits": 1, "model_predictions": len(observations), "native_executions": 0, "splits": splits,
		"fit_wall_ns": fitNS, "fit_cpu_ns": cpuNS(after) - cpuNS(before), "fit_cpu_wall_ratio": float64(cpuNS(after)-cpuNS(before)) / float64(fitNS),
		"rusage_peak_rss_native_units": after.Maxrss, "os": runtime.GOOS, "arch": runtime.GOARCH, "go": runtime.Version(),
		"parameter_count": contractdecision.InteractionRequirementParameterCount, "resident_weight_bytes": contractdecision.InteractionRequirementParameterCount * 4,
		"artifact_json_bytes": len(artifact), "scope": "100 fresh source documents in 25 sibling groups; one held-out prediction each; first selection and saved-observation checked completion separate; scores are finite-pool rankings, not correctness probabilities; no general natural-language or native execution claim"}
	return writeJSON(filepath.Join(os.Args[2], "report.json"), report)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
