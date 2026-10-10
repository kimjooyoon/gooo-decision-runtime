package flowdecision

import "errors"

// Explanation exposes the actual hidden activations and scores of one bounded
// forward pass. These are model observations, not semantic proof or causal
// attribution. Scores use the supplied candidate order; unused array cells are
// zero. The caller owns all storage and can retain it with the prediction.
type Explanation struct {
	ChoiceCount     int                            `json:"choice_count"`
	CandidateCount  int                            `json:"candidate_count"`
	Hidden          [MaxChoices][HiddenDim]float32 `json:"hidden_activations"`
	OptionScores    [MaxChoices][2]float32         `json:"option_scores"`
	CandidateScores [MaxCandidates]float64         `json:"candidate_scores"`
}

// ExplainInto predicts once using the same path as PredictInto and returns its
// internal observations. It does not fit, modify weights, assemble or execute a
// body. Errors preserve all caller outputs, including the explanation.
func (m *Model) ExplainInto(inputs [][FeatureDim]float32, masks []uint16,
	workspace *Workspace, output *Prediction, explanation *Explanation) error {
	if workspace == nil || output == nil || explanation == nil {
		return errors.New("flow workspace, prediction and explanation required")
	}
	var next Workspace
	var prediction Prediction
	if err := m.PredictInto(inputs, masks, &next, &prediction); err != nil {
		return err
	}
	observations := Explanation{ChoiceCount: len(inputs), CandidateCount: len(masks),
		Hidden: next.hidden, OptionScores: next.logits, CandidateScores: next.scores}
	*workspace, *output, *explanation = next, prediction, observations
	return nil
}
