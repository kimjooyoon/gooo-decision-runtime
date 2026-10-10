package contractdecision

import (
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const (
	ConditionDim              = decision.DeclaredConditionFeatureDim
	MaxConditions             = 128
	requirementOutputBias     = CaseDim * PoolDim
	requirementConditionStart = requirementOutputBias + PoolDim
	requirementConditionBias  = requirementConditionStart + (ConditionDim+1)*PoolDim
	requirementContextStart   = requirementConditionBias + PoolDim
	requirementHiddenStart    = requirementContextStart + FeatureDim*PoolDim
	requirementJointDim       = FeatureDim + 2*PoolDim
	requirementHiddenBias     = requirementHiddenStart + requirementJointDim*HiddenDim
	requirementScoreStart     = requirementHiddenBias + HiddenDim
	requirementScoreBias      = requirementScoreStart + 2*HiddenDim
	RequirementParameterCount = requirementScoreBias + 2
)

// ConditionSource supplies all authored Boolean requirements in source order.
// Readers remain immutable throughout prediction or training. Zero rows means
// no authored conditions; output cases are still required.
type ConditionSource interface {
	ConditionCount() int
	ConditionFeatureVersion() string
	ConditionFeatures(int) ([ConditionDim]float32, error)
}

// RequirementModel joins source, output goals and Boolean condition goals in
// one learned hidden layer. Its 13,282 FP32 weights occupy 53,128 bytes.
type RequirementModel struct {
	weights [RequirementParameterCount]float32
	extreme bool
}

// RequirementWorkspace snapshots both bounded streams once per prediction.
// Choice-specific pooling reuses them; no dataset-sized tensor is retained.
type RequirementWorkspace struct {
	cases   [2]frozenCases
	context [MaxChoices][PoolDim]float32
	pool    [MaxChoices][2][PoolDim]float32
	winner  [MaxChoices][2][PoolDim]int
	hidden  [MaxChoices][HiddenDim]float32
	logits  [MaxChoices][2]float32
	scores  [MaxCandidates]float64
}

func NewRequirementConditioned(weights [RequirementParameterCount]float32, pooling string) (*RequirementModel, error) {
	if !finite(weights[:]) || pooling != MeanPooling && pooling != ExtremePooling {
		return nil, errors.New("finite requirement weights and supported pooling required")
	}
	return &RequirementModel{weights: weights, extreme: pooling == ExtremePooling}, nil
}

func (m *RequirementModel) Weights() [RequirementParameterCount]float32 { return m.weights }
func (m *RequirementModel) CaseFeatureVersion() string                  { return decision.DeclaredCaseFeatureVersion }
func (m *RequirementModel) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}
func (m *RequirementModel) Pooling() string {
	if m != nil && m.extreme {
		return ExtremePooling
	}
	return MeanPooling
}

func validateRequirements(inputs [][FeatureDim]float32, cases CaseSource, conditions ConditionSource, masks []uint16) error {
	if err := validateCaseVersion(cases, decision.DeclaredCaseFeatureVersion); err != nil {
		return err
	}
	if err := validate(inputs, cases, masks); err != nil {
		return err
	}
	if conditions == nil || conditions.ConditionFeatureVersion() != decision.DeclaredConditionFeatureVersion ||
		conditions.ConditionCount() < 0 || conditions.ConditionCount() > MaxConditions {
		return errors.New("requirement model needs an explicit 0..128-row declared condition reader")
	}
	return nil
}

func (w *RequirementWorkspace) capture(cases CaseSource, conditions ConditionSource) error {
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
func (m *RequirementModel) PredictInto(inputs [][FeatureDim]float32, cases CaseSource, conditions ConditionSource,
	masks []uint16, workspace *RequirementWorkspace, output *Prediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("requirement model and destinations required")
	}
	if err := validateRequirements(inputs, cases, conditions, masks); err != nil {
		return err
	}
	var next RequirementWorkspace
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

func (m *RequirementModel) PredictChoicesInto(inputs [][FeatureDim]float32, cases CaseSource, conditions ConditionSource,
	workspace *RequirementWorkspace, output *ChoicePrediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("requirement model and destinations required")
	}
	if err := validateRequirements(inputs, cases, conditions, []uint16{0}); err != nil {
		return err
	}
	var next RequirementWorkspace
	if err := next.capture(cases, conditions); err != nil {
		return err
	}
	if err := m.forwardRequirements(inputs, nil, &next); err != nil {
		return err
	}
	*workspace, *output = next, ChoicePrediction{Count: len(inputs), Logits: next.logits}
	return nil
}
