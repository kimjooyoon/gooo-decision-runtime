package contractdecision

import "errors"

func (m *SourceRelativeV2Model) PredictChoicesInto(inputs [][528]float32, cases CaseSource, conditions ConditionSource,
	work *InteractionRequirementWorkspace, out *ChoicePrediction) error {
	if m == nil || m.inner == nil {
		return errors.New("source-relative V2 model required")
	}
	if err := validateSourceRelativeV2(inputs, conditions); err != nil {
		return err
	}
	return m.inner.PredictChoicesInto(inputs, cases, conditions, work, out)
}

func (m *SourceRelativeV2Model) PredictInto(inputs [][528]float32, cases CaseSource, conditions ConditionSource,
	masks []uint16, work *InteractionRequirementWorkspace, out *Prediction) error {
	if m == nil || m.inner == nil {
		return errors.New("source-relative V2 model required")
	}
	if err := validateSourceRelativeV2(inputs, conditions); err != nil {
		return err
	}
	return m.inner.PredictInto(inputs, cases, conditions, masks, work, out)
}
