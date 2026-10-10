package contractdecision

import (
	"errors"
	"math"
)

// Read the owned case snapshot directly. Sending its address back through the
// public CaseSource interface would make the entire temporary workspace escape
// to the heap on every prediction. Arithmetic order matches Model.forward for
// one choice, and is compared against legacy and independent equations in tests.
func (m *Model) forwardCaptured(input *[FeatureDim]float32, cases *frozenCases, w *Workspace) error {
	return m.forwardCapturedTrace(input, cases, w, nil)
}

func (m *Model) forwardCapturedTrace(input *[FeatureDim]float32, cases *frozenCases, w *Workspace, trace *ChoiceExplanationRow) error {
	var sums [PoolDim]float64
	for i, row := range cases.rows[:cases.count] {
		hidden := m.caseHidden(&row)
		if !finite(hidden[:]) {
			return errors.New("nonfinite conditioned case activation")
		}
		for h, value := range hidden {
			if m.extreme {
				old, magnitude := w.pool[h], math.Abs(float64(value))
				if i == 0 || magnitude > math.Abs(float64(old)) || magnitude == math.Abs(float64(old)) && value > old {
					w.pool[h], w.winner[h] = value, i
				}
			} else {
				sums[h] += float64(value)
			}
		}
	}
	if !m.extreme {
		for h, sum := range sums {
			w.pool[h] = float32(sum / float64(cases.count))
		}
	}
	if trace != nil {
		copy(trace.ConditionedBias[:], m.weights[caseBias:sourceWeights])
		trace.Pool = w.pool
		for h := range PoolDim {
			trace.Winner[h] = -1
			if m.extreme {
				trace.Winner[h] = w.winner[h]
			}
		}
	}
	for h := range HiddenDim {
		sum := m.weights[hiddenBias+h]
		start := sourceWeights + h*jointDim
		for j, x := range input {
			sum += x * m.weights[start+j]
		}
		if trace != nil {
			trace.SourcePrefix[h] = sum
		}
		for j, x := range w.pool {
			sum += x * m.weights[start+FeatureDim+j]
		}
		if trace != nil {
			trace.Joint[h] = sum
		}
		w.hidden[0][h] = activate(sum)
	}
	if !finite(w.hidden[0][:]) {
		return errors.New("nonfinite conditioned hidden activation")
	}
	for option := range 2 {
		sum := m.weights[outputBias+option]
		for h, x := range w.hidden[0] {
			sum += x * m.weights[outputWeights+option*HiddenDim+h]
		}
		w.logits[0][option] = sum
	}
	if !finite(w.logits[0][:]) {
		return errors.New("nonfinite conditioned option score")
	}
	if trace != nil {
		trace.Hidden, trace.OptionScores = w.hidden[0], w.logits[0]
	}
	return nil
}
