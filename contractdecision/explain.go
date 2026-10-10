package contractdecision

import "errors"

// Explanation records the actual intermediates of one prediction. SourcePrefix
// includes the hidden bias and source products in their original FP32 order.
// Joint is the value after adding the pooled-case products in the same pass.
// Their difference includes rounding and is not a causal attribution. Winner
// contains case indices for signed pooling and -1 for every mean coordinate.
// Unused choice/candidate cells are zero. The caller owns all storage.
type Explanation struct {
	ChoiceCount     int                            `json:"choice_count"`
	CandidateCount  int                            `json:"candidate_count"`
	CaseCount       int                            `json:"case_count"`
	Pooling         string                         `json:"pooling"`
	Pool            [PoolDim]float32               `json:"case_summary"`
	Winner          [PoolDim]int                   `json:"case_winners"`
	SourcePrefix    [MaxChoices][HiddenDim]float32 `json:"source_prefix"`
	Joint           [MaxChoices][HiddenDim]float32 `json:"joint_preactivation"`
	Hidden          [MaxChoices][HiddenDim]float32 `json:"hidden_activations"`
	OptionScores    [MaxChoices][2]float32         `json:"option_scores"`
	CandidateScores [MaxCandidates]float64         `json:"candidate_scores"`
}

// ExplainInto reads each case once and predicts using the normal computation,
// retaining its intermediates. It performs no fitting or candidate execution.
// Errors preserve every caller destination. Like PredictInto, callers may
// share immutable weights with separate workspaces and outputs.
func (m *Model) ExplainInto(inputs [][FeatureDim]float32, cases CaseSource, masks []uint16,
	workspace *Workspace, output *Prediction, explanation *Explanation) error {
	if explanation == nil {
		return errors.New("contract explanation destination required")
	}
	var trace Explanation
	if err := m.predict(inputs, cases, masks, workspace, output, &trace); err != nil {
		return err
	}
	*explanation = trace
	return nil
}
