package main

import (
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type source struct {
	ID, Split, SourceSHA, PlanSHA, CaseSHA string
	Group, Parity                          int
	Gooo                                   string
	Document                               pathplan.Document
	Inputs                                 [][528]float32
	Outputs, Conditions                    [][32]float32
	Acceptable                             uint64
	Candidates                             []candidate
	Probes                                 []pathplan.TestCase
}
type candidate struct {
	Mask         uint16
	Outputs      []pathplan.TestResult
	Conditions   []pathplan.ConditionResult
	ProbeResults []pathplan.TestResult
	Acceptable   bool
}
type counts struct {
	Documents, Complete, OutputMatches, Outputs, ConditionMatches, Conditions, ProbeMatches, Probes int
}
type observation struct {
	ID, Split              string
	FirstMask, CheckedMask uint16
	PredictNS              int64
	Probabilities          [4]float32
	First, Checked         counts
	Attempts               int
}
type splitResult struct {
	First, Checked, FixedOrder counts
	CheckedAttempts            int
}
type report struct {
	Schema           string                      `json:"schema"`
	CorpusSHA        string                      `json:"corpus_sha256"`
	Fingerprint      string                      `json:"model_fingerprint"`
	ModelSchema      string                      `json:"model_schema"`
	FeatureVersion   string                      `json:"feature_version"`
	Options          contractdecision.FitOptions `json:"options"`
	Fits             int                         `json:"fits"`
	Predictions      int                         `json:"model_predictions"`
	NativeExecutions int                         `json:"native_executions"`
	Splits           map[string]splitResult      `json:"splits"`
	Parameters       int                         `json:"parameter_count"`
	TensorBytes      int                         `json:"resident_weight_bytes"`
	ArtifactBytes    int                         `json:"artifact_json_bytes"`
}
