// Saved-record types mirror the committed producer 2db75199 without executing it.
package main

import (
	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type sourceRecord struct {
	ID       string            `json:"id"`
	Split    string            `json:"split"`
	Source   string            `json:"gooo_source"`
	SHA      string            `json:"source_sha256"`
	Document pathplan.Document `json:"document"`
}
type frozenContext struct {
	ID         string            `json:"id"`
	Split      string            `json:"split"`
	Context    string            `json:"context"`
	Features   [3][][320]float32 `json:"features_v2_off_on"`
	FeatureSHA [3]string         `json:"features_sha256_v2_off_on"`
	Masks      []uint16          `json:"candidate_masks"`
	Acceptable uint64            `json:"acceptable_candidate_bits"`
}
type projectionRow struct {
	ID         string                     `json:"id"`
	Split      string                     `json:"split"`
	SourceSHA  string                     `json:"source_sha256"`
	PlanSHA    string                     `json:"plan_sha256"`
	Decline    string                     `json:"decline"`
	Acceptable uint64                     `json:"acceptable_candidate_bits"`
	AfterSHA   string                     `json:"after_sha256"`
	Facts      []decision.BranchValueFlow `json:"facts"`
	Features   [][384]float32             `json:"features"`
}
type inputRecord struct {
	ID         string         `json:"id"`
	Split      string         `json:"split"`
	Context    string         `json:"context"`
	Features   [][384]float32 `json:"features"`
	SHA        string         `json:"features_sha256"`
	Masks      []uint16       `json:"candidate_masks"`
	Acceptable uint64         `json:"acceptable_candidate_bits"`
}
type preparedSource struct {
	source     sourceRecord
	plan       *pathplan.PreparedPlan
	projection projectionRow
}
type candidate struct {
	Mask            uint16                     `json:"mask"`
	Choices         map[string]string          `json:"choices"`
	TypeError       string                     `json:"type_error,omitempty"`
	GoooBody        string                     `json:"gooo_body"`
	GoSHA           string                     `json:"go_sha256"`
	Cases           []pathplan.TestResult      `json:"cases"`
	Conditions      []pathplan.ConditionResult `json:"conditions"`
	OutputPassed    int                        `json:"output_passed"`
	ConditionPassed int                        `json:"condition_passed"`
	Accepted        bool                       `json:"accepted"`
}
type judgment struct {
	ID         string                  `json:"id"`
	Split      string                  `json:"split"`
	Context    string                  `json:"context"`
	Model      string                  `json:"model"`
	SourceSHA  string                  `json:"source_sha256"`
	FeatureSHA string                  `json:"features_sha256"`
	Acceptable uint64                  `json:"acceptable_candidate_bits"`
	Prediction flowdecision.Prediction `json:"prediction"`
	PredictNS  int64                   `json:"predict_ns"`
	Selected   candidate               `json:"selected"`
}
type searchRow struct {
	ID           string                       `json:"id"`
	Split        string                       `json:"split"`
	Mode         string                       `json:"mode"`
	SearchNS     int64                        `json:"search_ns"`
	Result       pathplan.SearchResult        `json:"result"`
	Progress     []pathplan.ConditionProgress `json:"progress"`
	Feedback     []pathplan.ConditionRanking  `json:"feedback"`
	SelectedBody string                       `json:"selected_body"`
	Error        string                       `json:"error,omitempty"`
}
type group struct {
	Rows            int `json:"rows"`
	Valid           int `json:"valid"`
	OutputPassed    int `json:"output_passed"`
	OutputTotal     int `json:"output_total"`
	ConditionPassed int `json:"condition_passed"`
	ConditionTotal  int `json:"condition_total"`
}
type searchGroup struct {
	Programs      int   `json:"programs"`
	Complete      int   `json:"complete"`
	Attempts      int   `json:"attempts"`
	ModelCalls    int   `json:"model_calls"`
	FeedbackCalls int   `json:"feedback_calls"`
	Reused        int   `json:"reused"`
	SearchNS      int64 `json:"search_ns"`
}
type fitRecord struct {
	ModelSHA           string  `json:"model_sha256"`
	Fingerprint        string  `json:"model_fingerprint"`
	TrainingSHA        string  `json:"training_samples_sha256"`
	ArtifactBytes      int     `json:"artifact_bytes"`
	TrainingNS         int64   `json:"training_ns"`
	FirstLoss          float64 `json:"first_loss"`
	LastLoss           float64 `json:"last_pre_update_loss"`
	PredictionMedianNS int64   `json:"prediction_median_ns"`
	PredictionP95NS    int64   `json:"prediction_p95_ns"`
}
type report struct {
	Schema           string                  `json:"schema"`
	Producer         string                  `json:"producer_revision"`
	Go               string                  `json:"go_version"`
	OS               string                  `json:"os"`
	Arch             string                  `json:"arch"`
	InputDigests     map[string]string       `json:"input_gzip_sha256"`
	Programs         int                     `json:"programs"`
	Contexts         int                     `json:"contexts"`
	TrainingRows     int                     `json:"training_rows_per_fit"`
	TrainingCalls    int                     `json:"training_calls"`
	Parameters       int                     `json:"parameters_per_model"`
	WeightBytes      int                     `json:"weight_bytes_per_model"`
	InputBytes       int                     `json:"input_bytes_per_choice"`
	Judgments        int                     `json:"judgments"`
	Searches         int                     `json:"searches"`
	SearchModelCalls int                     `json:"search_model_calls"`
	NativeExecutions int                     `json:"native_executions"`
	Options          flowdecision.FitOptions `json:"fit_options"`
	Fits             map[string]fitRecord    `json:"fits"`
	Groups           map[string]group        `json:"groups"`
	SearchGroups     map[string]searchGroup  `json:"search_groups"`
	Scope            string                  `json:"scope"`
}
