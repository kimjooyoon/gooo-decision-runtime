package contractdecision

import (
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const (
	OrderedFeatureDim                = FeatureDim + decision.OrderedExpressionFeatureDim
	OrderedSourceFeatureVersion      = decision.RelationalFlowFeatureVersion + "+" + decision.OrderedExpressionFeatureVersion
	orderedRequirementOutputBias     = CaseDim * PoolDim
	orderedRequirementConditionStart = orderedRequirementOutputBias + PoolDim
	orderedRequirementConditionBias  = orderedRequirementConditionStart + (ConditionDim+1)*PoolDim
	orderedRequirementContextStart   = orderedRequirementConditionBias + PoolDim
	orderedRequirementHiddenStart    = orderedRequirementContextStart + OrderedFeatureDim*PoolDim
	orderedRequirementJointDim       = OrderedFeatureDim + 2*PoolDim
	orderedRequirementHiddenBias     = orderedRequirementHiddenStart + orderedRequirementJointDim*HiddenDim
	orderedRequirementScoreStart     = orderedRequirementHiddenBias + HiddenDim
	orderedRequirementScoreBias      = orderedRequirementScoreStart + 2*HiddenDim
	OrderedRequirementParameterCount = orderedRequirementScoreBias + 2
)

// OrderedRequirementModel joins source, output goals and Boolean condition goals in
// one learned hidden layer. Its 17,890 FP32 weights occupy 71,560 bytes. Ordered
// expressions enter both the choice-query projection and the joint hidden layer.
type OrderedRequirementModel struct {
	weights [OrderedRequirementParameterCount]float32
	extreme bool
}

// OrderedRequirementWorkspace snapshots both bounded streams once per prediction.
// Choice-specific pooling reuses them; no dataset-sized tensor is retained.
type OrderedRequirementWorkspace struct {
	cases   [2]frozenCases
	context [MaxChoices][PoolDim]float32
	pool    [MaxChoices][2][PoolDim]float32
	winner  [MaxChoices][2][PoolDim]int
	hidden  [MaxChoices][HiddenDim]float32
	logits  [MaxChoices][2]float32
	scores  [MaxCandidates]float64
}

func NewOrderedRequirementConditioned(weights [OrderedRequirementParameterCount]float32, pooling string) (*OrderedRequirementModel, error) {
	if !finite(weights[:]) || pooling != MeanPooling && pooling != ExtremePooling {
		return nil, errors.New("finite requirement weights and supported pooling required")
	}
	return &OrderedRequirementModel{weights: weights, extreme: pooling == ExtremePooling}, nil
}

func (m *OrderedRequirementModel) Weights() [OrderedRequirementParameterCount]float32 {
	return m.weights
}
func (m *OrderedRequirementModel) SourceFeatureVersion() string {
	return OrderedSourceFeatureVersion
}
func (m *OrderedRequirementModel) CaseFeatureVersion() string {
	return decision.DeclaredCaseFeatureVersion
}
func (m *OrderedRequirementModel) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}
func (m *OrderedRequirementModel) Pooling() string {
	if m != nil && m.extreme {
		return ExtremePooling
	}
	return MeanPooling
}

func validateOrderedRequirements(inputs [][OrderedFeatureDim]float32, cases CaseSource, conditions ConditionSource, masks []uint16) error {
	if len(inputs) < 1 || len(inputs) > MaxChoices {
		return errors.New("ordered requirement input needs 1..16 choices")
	}
	var prefix [MaxChoices][FeatureDim]float32
	for i, input := range inputs {
		if !finite(input[:]) {
			return errors.New("finite ordered source input required")
		}
		copy(prefix[i][:], input[:FeatureDim])
	}
	return validateRequirements(prefix[:len(inputs)], cases, conditions, masks)
}

func (w *OrderedRequirementWorkspace) capture(cases CaseSource, conditions ConditionSource) error {
	if err := w.cases[0].capture(cases); err != nil {
		return err
	}
	w.cases[1].count = conditions.ConditionCount()
	if w.cases[1].count < 0 || w.cases[1].count > MaxConditions {
		return errors.New("declared condition count changed outside bounds")
	}
	for i := range w.cases[1].count {
		row, err := conditions.ConditionFeatures(i)
		if err != nil {
			return err
		}
		if !finite(row[:]) {
			return errors.New("nonfinite declared condition input")
		}
		w.cases[1].rows[i] = row
	}
	return nil
}

// PredictInto scores complete candidate masks. Probabilities describe this
// supplied pool; Gooo still checks the original output and condition cases.
// Errors preserve both caller destinations.
func (m *OrderedRequirementModel) PredictInto(inputs [][OrderedFeatureDim]float32, cases CaseSource, conditions ConditionSource,
	masks []uint16, workspace *OrderedRequirementWorkspace, output *Prediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("requirement model and destinations required")
	}
	if err := validateOrderedRequirements(inputs, cases, conditions, masks); err != nil {
		return err
	}
	var next OrderedRequirementWorkspace
	if err := next.capture(cases, conditions); err != nil {
		return err
	}
	if err := m.forwardRequirements(inputs, masks, &next); err != nil {
		return err
	}
	probabilities, _ := distribution(next.scores[:len(masks)], allCandidates(len(masks)))
	result := Prediction{Count: len(masks)}
	best := 0
	for i, mask := range masks {
		result.Probabilities[i] = float32(probabilities[i])
		if next.scores[i] > next.scores[best] || next.scores[i] == next.scores[best] && mask < masks[best] {
			best = i
		}
	}
	result.Selected = masks[best]
	*workspace, *output = next, result
	return nil
}

func (m *OrderedRequirementModel) PredictChoicesInto(inputs [][OrderedFeatureDim]float32, cases CaseSource, conditions ConditionSource,
	workspace *OrderedRequirementWorkspace, output *ChoicePrediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("requirement model and destinations required")
	}
	if err := validateOrderedRequirements(inputs, cases, conditions, []uint16{0}); err != nil {
		return err
	}
	var next OrderedRequirementWorkspace
	if err := next.capture(cases, conditions); err != nil {
		return err
	}
	if err := m.forwardRequirements(inputs, nil, &next); err != nil {
		return err
	}
	*workspace, *output = next, ChoicePrediction{Count: len(inputs), Logits: next.logits}
	return nil
}
