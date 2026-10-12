package contractdecision

import (
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const (
	typedPoolDim                          = 4
	typedHiddenDim                        = 12
	typedOutputBias                       = interactionCaseDim * typedPoolDim
	typedConditionStart                   = typedOutputBias + typedPoolDim
	typedConditionBias                    = typedConditionStart + (interactionCaseDim+1)*typedPoolDim
	typedContextStart                     = typedConditionBias + typedPoolDim
	typedContextBias                      = typedContextStart + OrderedFeatureDim*typedPoolDim
	typedHiddenStart                      = typedContextBias + typedPoolDim
	typedJointDim                         = OrderedFeatureDim + interactionTermCount*typedPoolDim
	typedHiddenBias                       = typedHiddenStart + typedJointDim*typedHiddenDim
	typedScoreStart                       = typedHiddenBias + typedHiddenDim
	typedScoreBias                        = typedScoreStart + 2*typedHiddenDim
	TypedSourceRelativeHeadParameterCount = typedScoreBias + 2
	TypedSourceRelativeParameterCount     = 2 * TypedSourceRelativeHeadParameterCount
)

type typedRelativeHead [TypedSourceRelativeHeadParameterCount]float32

// TypedSourceRelativeModel routes canonical operand-order and branch-layout
// choices to independent small heads. All V4 source/requirement channels remain.
// Two 9,182-parameter heads occupy 73,456 raw FP32 tensor bytes in total.
type TypedSourceRelativeModel struct {
	weights     [2]typedRelativeHead
	fingerprint string
}

type typedRelativeChoiceWorkspace struct {
	context [typedPoolDim]float32
	pool    [2][typedPoolDim]float32
	hidden  [typedHiddenDim]float32
	logits  [2]float32
}

// TypedSourceRelativeWorkspace retains bounded case rows and two choice states.
// Each caller owns its workspace; immutable models can serve concurrent callers.
type TypedSourceRelativeWorkspace struct {
	cases   [2]frozenCases
	choices [2]typedRelativeChoiceWorkspace
	scores  [MaxCandidates]float64
}

func NewTypedSourceRelative(weights [2][TypedSourceRelativeHeadParameterCount]float32) (*TypedSourceRelativeModel, error) {
	m := &TypedSourceRelativeModel{}
	for i, head := range weights {
		if !finite(head[:]) {
			return nil, errors.New("finite typed source weights required")
		}
		m.weights[i] = head
	}
	if err := m.bindFingerprint(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *TypedSourceRelativeModel) Weights() [2][TypedSourceRelativeHeadParameterCount]float32 {
	return [2][TypedSourceRelativeHeadParameterCount]float32{m.weights[0], m.weights[1]}
}
func (m *TypedSourceRelativeModel) ArtifactSchema() string { return TypedSourceRelativeSchema }
func (m *TypedSourceRelativeModel) SourceFeatureVersion() string {
	return SourceRelativeV4FeatureVersion
}
func (m *TypedSourceRelativeModel) CaseFeatureVersion() string {
	return decision.DeclaredCaseFeatureVersion
}
func (m *TypedSourceRelativeModel) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}
func (m *TypedSourceRelativeModel) Pooling() string { return MeanPooling }
func (m *TypedSourceRelativeModel) Fingerprint() string {
	if m == nil {
		return ""
	}
	return m.fingerprint
}

// Canonical source flags are normalized together, so their positive magnitude
// depends on the source. Routing uses the one-hot kind, never a choice ordinal.
func typedRelativeRoute(input [OrderedFeatureDim]float32) (int, error) {
	if input[0] != 0 || input[1] != 0 || input[4] != 0 {
		return 0, errors.New("typed source head requires operand_order or branch_layout")
	}
	if input[2] > 0 && input[3] == 0 {
		return 0, nil
	}
	if input[3] > 0 && input[2] == 0 {
		return 1, nil
	}
	return 0, errors.New("typed source head requires one canonical choice kind")
}

func validateTypedRelative(inputs [][OrderedFeatureDim]float32, cases CaseSource, conditions ConditionSource, masks []uint16) error {
	if err := validateOrderedRequirements(inputs, cases, conditions, masks); err != nil {
		return err
	}
	if err := validateSourceRelativeV4(inputs, cases, conditions); err != nil {
		return err
	}
	for _, row := range inputs {
		route, err := typedRelativeRoute(row)
		if err != nil {
			return err
		}
		if row[196+route] != 1 {
			return errors.New("typed source kind differs from condition relation kind")
		}
	}
	return nil
}

func (w *TypedSourceRelativeWorkspace) capture(cases CaseSource, conditions ConditionSource) error {
	if err := w.cases[0].capture(cases); err != nil {
		return err
	}
	w.cases[1].count = conditions.ConditionCount()
	if w.cases[1].count < 0 || w.cases[1].count > MaxConditions {
		return errors.New("declared condition count changed")
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

func (m *TypedSourceRelativeModel) forward(inputs [][OrderedFeatureDim]float32, masks []uint16, w *TypedSourceRelativeWorkspace) error {
	for choice, row := range inputs {
		route, err := typedRelativeRoute(row)
		if err != nil {
			return err
		}
		if err := m.weights[route].forward(choice, row, &w.cases, &w.choices[choice]); err != nil {
			return err
		}
	}
	for i, mask := range masks {
		for choice := range inputs {
			w.scores[i] += float64(w.choices[choice].logits[mask>>choice&1])
		}
	}
	return nil
}

func (m *TypedSourceRelativeModel) PredictInto(inputs [][OrderedFeatureDim]float32, cases CaseSource, conditions ConditionSource,
	masks []uint16, workspace *TypedSourceRelativeWorkspace, output *Prediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("typed source model and destinations required")
	}
	if err := validateTypedRelative(inputs, cases, conditions, masks); err != nil {
		return err
	}
	var next TypedSourceRelativeWorkspace
	if err := next.capture(cases, conditions); err != nil {
		return err
	}
	if err := m.forward(inputs, masks, &next); err != nil {
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

func (m *TypedSourceRelativeModel) PredictChoicesInto(inputs [][OrderedFeatureDim]float32, cases CaseSource, conditions ConditionSource,
	workspace *TypedSourceRelativeWorkspace, output *ChoicePrediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("typed source model and destinations required")
	}
	if err := validateTypedRelative(inputs, cases, conditions, []uint16{0}); err != nil {
		return err
	}
	var next TypedSourceRelativeWorkspace
	if err := next.capture(cases, conditions); err != nil {
		return err
	}
	if err := m.forward(inputs, nil, &next); err != nil {
		return err
	}
	result := ChoicePrediction{Count: len(inputs)}
	for i := range inputs {
		result.Logits[i] = next.choices[i].logits
	}
	*workspace, *output = next, result
	return nil
}
