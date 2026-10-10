package contractdecision

import "errors"

// ChoiceExplanationRow records the actual computation for one source choice.
// ConditionedBias includes the learned case bias and source products in FP32
// order. Winner holds zero-based case indices for signed pooling, or -1 for
// mean pooling. A winning coordinate is an observed intermediate, not a causal
// attribution or a proof that an individual case determined the selected body.
type ChoiceExplanationRow struct {
	ConditionedBias [PoolDim]float32   `json:"source_conditioned_case_bias"`
	Pool            [PoolDim]float32   `json:"case_summary"`
	Winner          [PoolDim]int       `json:"case_winners"`
	SourcePrefix    [HiddenDim]float32 `json:"source_prefix"`
	Joint           [HiddenDim]float32 `json:"joint_preactivation"`
	Hidden          [HiddenDim]float32 `json:"hidden_activations"`
	OptionScores    [2]float32         `json:"option_scores"`
}

// ChoiceExplanation belongs to one prediction with the same captured cases as
// its scores. Unused cells are zero; no candidates are enumerated by the trace.
type ChoiceExplanation struct {
	ChoiceCount     int                              `json:"choice_count"`
	CandidateCount  int                              `json:"candidate_count"`
	CaseCount       int                              `json:"case_count"`
	Pooling         string                           `json:"pooling"`
	Choices         [MaxChoices]ChoiceExplanationRow `json:"choices"`
	CandidateScores [MaxCandidates]float64           `json:"candidate_scores"`
}

// ExplainInto scores the supplied candidate masks and records intermediate
// values in that same pass. It reads each caller case once, with no training or
// body execution. Errors preserve every caller destination. Shared immutable
// models need a separate workspace, output and explanation for each request.
func (m *ChoiceModel) ExplainInto(inputs [][FeatureDim]float32, cases CaseSource, masks []uint16,
	workspace *ChoiceWorkspace, output *Prediction, explanation *ChoiceExplanation) error {
	if explanation == nil {
		return errors.New("choice explanation destination required")
	}
	var trace ChoiceExplanation
	if err := m.predictTrace(inputs, cases, masks, workspace, output, &trace); err != nil {
		return err
	}
	*explanation = trace
	return nil
}

// ExplainChoicesInto records the same pass as PredictChoicesInto, without
// materializing the combinatorial candidate space. CandidateCount and scores
// remain zero; all active choice logits and case summaries are available.
func (m *ChoiceModel) ExplainChoicesInto(inputs [][FeatureDim]float32, cases CaseSource,
	workspace *ChoiceWorkspace, output *ChoicePrediction, explanation *ChoiceExplanation) error {
	if explanation == nil {
		return errors.New("choice explanation destination required")
	}
	var trace ChoiceExplanation
	if err := m.predictChoicesTrace(inputs, cases, workspace, output, &trace); err != nil {
		return err
	}
	*explanation = trace
	return nil
}
