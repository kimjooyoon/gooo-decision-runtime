package contractdecision

import (
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const ChoiceContextParameterCount = FeatureDim * PoolDim
const ChoiceParameterCount = ParameterCount + ChoiceContextParameterCount

// ChoiceModel conditions the case encoder on each source choice before pooling.
// All original source/case cells are retained. Additional shared weights connect
// 384 source cells to the eight case neurons: 12,818 FP32 weights in total.
type ChoiceModel struct {
	base    Model
	context [ChoiceContextParameterCount]float32
}

// ChoiceWorkspace owns a complete 16KiB case snapshot and bounded working
// arrays. The local copy folds one source projection into case biases, reusing
// the existing arithmetic/backpropagation without changing legacy workspaces.
type ChoiceWorkspace struct {
	cases  frozenCases
	local  Model
	work   Workspace
	logits [MaxChoices][2]float32
	scores [MaxCandidates]float64
}

func NewChoiceConditioned(weights [ParameterCount]float32, contextWeights [ChoiceContextParameterCount]float32, pooling string) (*ChoiceModel, error) {
	if !finite(contextWeights[:]) {
		return nil, errors.New("finite choice context weights required")
	}
	base, err := NewForPooling(weights, pooling)
	if err != nil {
		return nil, err
	}
	return &ChoiceModel{base: *base, context: contextWeights}, nil
}

func (m *ChoiceModel) CaseFeatureVersion() string { return decision.DeclaredCaseFeatureVersion }
func (m *ChoiceModel) Pooling() string            { return m.base.Pooling() }
func (m *ChoiceModel) Weights() ([ParameterCount]float32, [ChoiceContextParameterCount]float32) {
	return m.base.weights, m.context
}

func (m *ChoiceModel) localize(input *[FeatureDim]float32, local *Model) error {
	*local = m.base
	for h := range PoolDim {
		for j, x := range input {
			local.weights[caseBias+h] += x * m.context[h*FeatureDim+j]
		}
	}
	if !finite(local.weights[caseBias:sourceWeights]) {
		return errors.New("nonfinite source-conditioned case bias")
	}
	return nil
}

func (m *ChoiceModel) forward(inputs [][FeatureDim]float32, masks []uint16, w *ChoiceWorkspace) error {
	for i := range inputs {
		if err := m.localize(&inputs[i], &w.local); err != nil {
			return err
		}
		w.work = Workspace{}
		if err := w.local.forwardCaptured(&inputs[i], &w.cases, &w.work); err != nil {
			return err
		}
		w.logits[i] = w.work.logits[0]
	}
	for i, mask := range masks {
		for c := range inputs {
			w.scores[i] += float64(w.logits[c][mask>>c&1])
		}
	}
	return nil
}

func (m *ChoiceModel) prepare(inputs [][FeatureDim]float32, cases CaseSource, masks []uint16, w *ChoiceWorkspace) error {
	if err := validateCaseVersion(cases, decision.DeclaredCaseFeatureVersion); err != nil {
		return err
	}
	if err := validate(inputs, cases, masks); err != nil {
		return err
	}
	// Read every caller case exactly once, including the last case, before
	// doing the choice-specific passes. No dataset-wide tensor is retained.
	return w.cases.capture(cases)
}

func (m *ChoiceModel) PredictInto(inputs [][FeatureDim]float32, cases CaseSource, masks []uint16, workspace *ChoiceWorkspace, output *Prediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("choice model and destinations required")
	}
	var next ChoiceWorkspace
	if err := m.prepare(inputs, cases, masks, &next); err != nil {
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

func (m *ChoiceModel) PredictChoicesInto(inputs [][FeatureDim]float32, cases CaseSource, workspace *ChoiceWorkspace, output *ChoicePrediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("choice model and destinations required")
	}
	var next ChoiceWorkspace
	if err := m.prepare(inputs, cases, []uint16{0}, &next); err != nil {
		return err
	}
	if err := m.forward(inputs, nil, &next); err != nil {
		return err
	}
	*workspace, *output = next, ChoicePrediction{Count: len(inputs), Logits: next.logits}
	return nil
}
