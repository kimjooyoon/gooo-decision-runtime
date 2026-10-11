package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func audit(root string) (map[string]any, error) {
	sources, raw, err := corpus(root)
	if err != nil {
		return nil, err
	}
	if len(sources) != 100 {
		return nil, fmt.Errorf("100 source documents required")
	}
	if digest(raw) != "sha256:1bd6cb3e1dde489e49017097c88aa7a627b80b000b7e478ad0c0d3d626d87f71" {
		return nil, fmt.Errorf("pinned pre-fit corpus bytes differ")
	}
	for i, s := range sources {
		if err = auditSource(s, i); err != nil {
			return nil, fmt.Errorf("%s: %w", s.ID, err)
		}
	}
	var r report
	if err = decode(root, "report.json", &r); err != nil {
		return nil, err
	}
	options := contractdecision.FitOptions{Epochs: 100, LearningRate: .01, L2: .00001, Seed: 20261012}
	if r.Schema != "gooo/return-layout-learning-observation/v1" || r.CorpusSHA != digest(raw) || r.Options != options || r.Fits != 1 || r.Predictions != 100 || r.NativeExecutions != 0 {
		return nil, fmt.Errorf("fit/report identity differs")
	}
	modelBytes, err := read(root, "model.json", 512<<10)
	if err != nil {
		return nil, err
	}
	if digest(modelBytes) != "sha256:8570dd8c6a2aef2676dfc027740d869797a8a11733bd7e07cdd501dc21e4a0bc" {
		return nil, fmt.Errorf("pinned fitted weights differ")
	}
	m, err := contractdecision.DecodeSourceRelativeV2(modelBytes)
	if err != nil {
		return nil, err
	}
	if r.Fingerprint != m.Fingerprint() || digest(modelBytes) != "sha256:"+m.Fingerprint() || r.ModelSchema != m.ArtifactSchema() || r.FeatureVersion != m.SourceFeatureVersion() || r.Parameters != 19034 || r.TensorBytes != 76136 || r.ArtifactBytes != len(modelBytes) {
		return nil, fmt.Errorf("model identity differs")
	}
	var started struct {
		CorpusSHA string                      `json:"corpus_sha256"`
		Documents int                         `json:"training_documents"`
		Options   contractdecision.FitOptions `json:"options"`
	}
	if err = decode(root, "fit-started.json", &started); err != nil {
		return nil, err
	}
	if started.CorpusSHA != r.CorpusSHA || started.Documents != 60 || started.Options != options {
		return nil, fmt.Errorf("fit input identity differs")
	}
	var history []contractdecision.Epoch
	if err = decode(root, "history.json", &history); err != nil {
		return nil, err
	}
	if len(history) != 100 {
		return nil, fmt.Errorf("loss history missing")
	}
	for i, h := range history {
		if h.Number != i+1 || h.Loss < 0 || math.IsNaN(h.Loss) || math.IsInf(h.Loss, 0) {
			return nil, fmt.Errorf("invalid epoch")
		}
	}
	var observations []observation
	if err = decode(root, "observations.json", &observations); err != nil {
		return nil, err
	}
	if err = auditObservations(sources, observations, r.Splits); err != nil {
		return nil, err
	}
	return map[string]any{"schema": "gooo/return-layout-retained-audit/v1", "source_documents": 100, "contract_groups": 25, "original_fits": 1,
		"original_predictions": 100, "new_fits": 0, "new_predictions": 0, "new_candidate_executions": 0, "corpus_sha256": r.CorpusSHA,
		"model_fingerprint": r.Fingerprint, "splits": r.Splits, "scope": "current V2 source/requirement encoders compared with frozen arrays; original numerical predictions and interpreted outcomes read, not re-executed"}, nil
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: audit RESULT_DIRECTORY")
		os.Exit(1)
	}
	r, err := audit(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	if err = e.Encode(r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
