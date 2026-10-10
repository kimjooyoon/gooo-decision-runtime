package pathplan

import (
	"context"
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/executiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
)

// NewExecutionSession uses the v3 output-feedback model and the same finite
// compiler/evaluator/frontier as NewConditionSession. Nil uses deterministic
// fallback. Only completed attempts can supply feedback; no model is retained.
func (p *PreparedPlan) NewExecutionSession(ctx context.Context, model *executiondecision.Model, cases []TestCase, seed string) (*ConditionSession, error) {
	return p.newConditionSession(ctx, executionRanker{execution: model}, cases, seed)
}

func (s *ConditionSession) ReconsiderExecution(ctx context.Context, model *executiondecision.Model) (ConditionRanking, error) {
	return s.reconsider(ctx, executionRanker{execution: model})
}

func (p *PreparedPlan) SearchExecutionBatches(ctx context.Context, model *executiondecision.Model, cases []TestCase, total, step int, seed string, rounds int) (SearchResult, *bodyplan.Program, []ConditionProgress, []ConditionRanking, error) {
	return p.searchConditionBatches(ctx, executionRanker{execution: model}, cases, total, step, seed, rounds)
}

// ExecutionInput returns the latest owned committed observation without body
// execution. This supports training-data collection using the real search path.
// It uses the same prompt-busy contract as other session operations.
func (s *ConditionSession) ExecutionInput() (*ConditionInput, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("execution session required")
	}
	if !s.lock.TryLock() {
		return nil, ErrSessionBusy
	}
	defer s.lock.Unlock()
	if !s.hasOutputChannel() {
		return nil, errors.New("execution model session required")
	}
	input := s.input
	return &input, nil
}

// The input ABIs share search behavior while retaining distinct model shapes,
// artifact schemas and fingerprints. This transient adapter is never retained.
type executionRanker struct {
	condition *conditiondecision.Model
	execution *executiondecision.Model
	flow      *flowdecision.Model
}

func (m executionRanker) present() bool {
	return m.condition != nil || m.execution != nil || m.flow != nil
}
func (m executionRanker) FeatureVersion() string {
	if m.flow != nil {
		return m.flow.FeatureVersion()
	}
	if m.execution != nil {
		return m.execution.FeatureVersion()
	}
	return m.condition.FeatureVersion()
}
func (m executionRanker) Fingerprint() string {
	if m.flow != nil {
		return m.flow.Fingerprint()
	}
	if m.execution != nil {
		return m.execution.Fingerprint()
	}
	return m.condition.Fingerprint()
}
func (m executionRanker) schema() string {
	if m.flow != nil {
		return m.flow.ArtifactSchema()
	}
	if m.execution != nil {
		return executiondecision.Schema
	}
	return conditiondecision.Schema
}
func (m executionRanker) variant() string {
	if m.flow != nil {
		return "flow_fp32"
	}
	if m.execution != nil {
		return "execution_fp32"
	}
	return "condition_fp32"
}
func (m executionRanker) predict(inputs [][decision.ExecutionFlowFeatureDim]float32, output *[16][2]float32) error {
	if m.flow != nil {
		var workspace flowdecision.Workspace
		var prediction flowdecision.ChoicePrediction
		if err := m.flow.PredictChoicesInto(inputs, &workspace, &prediction); err != nil {
			return err
		}
		*output = prediction.Logits
		return nil
	}
	if m.execution != nil {
		var prefix [16][decision.ExecutionFeatureDim]float32
		for i := range inputs {
			copy(prefix[i][:], inputs[i][:decision.ExecutionFeatureDim])
		}
		var workspace executiondecision.Workspace
		var prediction executiondecision.ChoicePrediction
		if err := m.execution.PredictChoicesInto(prefix[:len(inputs)], &workspace, &prediction); err != nil {
			return err
		}
		*output = prediction.Logits
		return nil
	}
	var prefix [16][decision.FeatureDim]float32
	for i := range inputs {
		copy(prefix[i][:], inputs[i][:decision.FeatureDim])
	}
	var workspace conditiondecision.Workspace
	var prediction conditiondecision.ChoicePrediction
	if err := m.condition.PredictChoicesInto(prefix[:len(inputs)], &workspace, &prediction); err != nil {
		return err
	}
	*output = prediction.Logits
	return nil
}

func ownedExecutionRanking(r ConditionRanking) ConditionRanking {
	if r.OutputFailure != nil {
		failure := *r.OutputFailure
		r.OutputFailure = &failure
	}
	return r
}
