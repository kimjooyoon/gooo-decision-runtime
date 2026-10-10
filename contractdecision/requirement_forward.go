package contractdecision

import (
	"errors"
	"math"
)

func requirementEncoder(channel int) (start, bias, width int) {
	if channel == 1 {
		return requirementConditionStart, requirementConditionBias, ConditionDim + 1
	}
	return 0, requirementOutputBias, CaseDim
}

func (m *RequirementModel) requirementCaseHidden(channel, choice int, row *[CaseDim]float32, source *[PoolDim]float32) [PoolDim]float32 {
	start, bias, width := requirementEncoder(channel)
	var result [PoolDim]float32
	for h := range PoolDim {
		sum := m.weights[bias+h] + source[h]
		for j, value := range row {
			sum += value * m.weights[start+h*width+j]
		}
		if channel == 1 {
			// The original 32 cells remain present. One extra query-relative
			// cell says whether this condition targets the choice being ranked.
			sum += row[16+choice] * m.weights[start+h*width+ConditionDim]
		}
		result[h] = activate(sum)
	}
	return result
}

func (m *RequirementModel) requirementPool(channel, choice int, w *RequirementWorkspace) error {
	var sums [PoolDim]float64
	rows := &w.cases[channel]
	for i, row := range rows.rows[:rows.count] {
		hidden := m.requirementCaseHidden(channel, choice, &row, &w.context[choice])
		if !finite(hidden[:]) {
			return errors.New("nonfinite requirement case activation")
		}
		for h, value := range hidden {
			if m.extreme {
				old, magnitude := w.pool[choice][channel][h], math.Abs(float64(value))
				if i == 0 || magnitude > math.Abs(float64(old)) || magnitude == math.Abs(float64(old)) && value > old {
					w.pool[choice][channel][h], w.winner[choice][channel][h] = value, i
				}
			} else {
				sums[h] += float64(value)
			}
		}
	}
	if !m.extreme && rows.count > 0 {
		for h, sum := range sums {
			w.pool[choice][channel][h] = float32(sum / float64(rows.count))
		}
	}
	return nil
}

func (m *RequirementModel) forwardRequirements(inputs [][FeatureDim]float32, masks []uint16, w *RequirementWorkspace) error {
	for choice, input := range inputs {
		for h := range PoolDim {
			for j, value := range input {
				w.context[choice][h] += value * m.weights[requirementContextStart+h*FeatureDim+j]
			}
		}
		for channel := range 2 {
			if err := m.requirementPool(channel, choice, w); err != nil {
				return err
			}
		}
		for h := range HiddenDim {
			sum := m.weights[requirementHiddenBias+h]
			start := requirementHiddenStart + h*requirementJointDim
			for j, value := range input {
				sum += value * m.weights[start+j]
			}
			for channel := range 2 {
				for j, value := range w.pool[choice][channel] {
					sum += value * m.weights[start+FeatureDim+channel*PoolDim+j]
				}
			}
			w.hidden[choice][h] = activate(sum)
		}
		if !finite(w.hidden[choice][:]) {
			return errors.New("nonfinite joint requirement activation")
		}
		for option := range 2 {
			sum := m.weights[requirementScoreBias+option]
			for h, value := range w.hidden[choice] {
				sum += value * m.weights[requirementScoreStart+option*HiddenDim+h]
			}
			w.logits[choice][option] = sum
		}
		if !finite(w.logits[choice][:]) {
			return errors.New("nonfinite requirement score")
		}
	}
	for i, mask := range masks {
		for choice := range inputs {
			w.scores[i] += float64(w.logits[choice][mask>>choice&1])
		}
	}
	return nil
}
