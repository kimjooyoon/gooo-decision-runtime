package contractdecision

import (
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const (
	interactionCaseDim                           = CaseDim + 11
	interactionRequirementOutputBias             = interactionCaseDim * PoolDim
	interactionRequirementConditionStart         = interactionRequirementOutputBias + PoolDim
	interactionRequirementConditionBias          = interactionRequirementConditionStart + (interactionCaseDim+1)*PoolDim
	interactionRequirementContextStart           = interactionRequirementConditionBias + PoolDim
	interactionRequirementContextBias            = interactionRequirementContextStart + OrderedFeatureDim*PoolDim
	interactionRequirementHiddenStart            = interactionRequirementContextBias + PoolDim
	interactionTermCount                         = 7
	interactionRequirementJointDim               = OrderedFeatureDim + interactionTermCount*PoolDim
	interactionRequirementHiddenBias             = interactionRequirementHiddenStart + interactionRequirementJointDim*HiddenDim
	interactionRequirementScoreStart             = interactionRequirementHiddenBias + HiddenDim
	interactionRequirementScoreBias              = interactionRequirementScoreStart + 2*HiddenDim
	InteractionRequirementParameterCount         = interactionRequirementScoreBias + 2
	interactionInputScale                float32 = 8
)

// InteractionRequirementModel joins source, output goals and Boolean condition goals in
// one learned hidden layer. It binds each case to its authored target polarity,
// then learns bounded source, output and condition interactions.
// Its 19,034 FP32 weights occupy 76,136 bytes.
type InteractionRequirementModel struct {
	weights [InteractionRequirementParameterCount]float32
	extreme bool
}

// InteractionRequirementWorkspace snapshots both bounded streams once per prediction.
// Choice-specific pooling reuses them; no dataset-sized tensor is retained.
type InteractionRequirementWorkspace struct {
	cases   [2]frozenCases
	context [MaxChoices][PoolDim]float32
	pool    [MaxChoices][2][PoolDim]float32
	winner  [MaxChoices][2][PoolDim]int
	hidden  [MaxChoices][HiddenDim]float32
	logits  [MaxChoices][2]float32
	scores  [MaxCandidates]float64
}

func NewInteractionRequirementConditioned(weights [InteractionRequirementParameterCount]float32, pooling string) (*InteractionRequirementModel, error) {
	if !finite(weights[:]) || pooling != MeanPooling && pooling != ExtremePooling {
		return nil, errors.New("finite requirement weights and supported pooling required")
	}
	return &InteractionRequirementModel{weights: weights, extreme: pooling == ExtremePooling}, nil
}

func (m *InteractionRequirementModel) Weights() [InteractionRequirementParameterCount]float32 {
	return m.weights
}
func (m *InteractionRequirementModel) SourceFeatureVersion() string {
	return OrderedSourceFeatureVersion
}
func (m *InteractionRequirementModel) CaseFeatureVersion() string {
	return decision.DeclaredCaseFeatureVersion
}
func (m *InteractionRequirementModel) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}
func (m *InteractionRequirementModel) Pooling() string {
	if m != nil && m.extreme {
		return ExtremePooling
	}
	return MeanPooling
}

func (w *InteractionRequirementWorkspace) capture(cases CaseSource, conditions ConditionSource) error {
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
func (m *InteractionRequirementModel) PredictInto(inputs [][OrderedFeatureDim]float32, cases CaseSource, conditions ConditionSource,
	masks []uint16, workspace *InteractionRequirementWorkspace, output *Prediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("requirement model and destinations required")
	}
	if err := validateOrderedRequirements(inputs, cases, conditions, masks); err != nil {
		return err
	}
	var next InteractionRequirementWorkspace
	if err := next.capture(cases, conditions); err != nil {
		return err
	}
	if err := m.forwardInteractions(inputs, masks, &next); err != nil {
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

func (m *InteractionRequirementModel) PredictChoicesInto(inputs [][OrderedFeatureDim]float32, cases CaseSource, conditions ConditionSource,
	workspace *InteractionRequirementWorkspace, output *ChoicePrediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("requirement model and destinations required")
	}
	if err := validateOrderedRequirements(inputs, cases, conditions, []uint16{0}); err != nil {
		return err
	}
	var next InteractionRequirementWorkspace
	if err := next.capture(cases, conditions); err != nil {
		return err
	}
	if err := m.forwardInteractions(inputs, nil, &next); err != nil {
		return err
	}
	*workspace, *output = next, ChoicePrediction{Count: len(inputs), Logits: next.logits}
	return nil
}
