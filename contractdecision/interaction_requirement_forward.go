package contractdecision

import (
	"errors"
	"math"
)

func interactionTanh(value float32) float32 {
	if math.IsInf(float64(value), 0) || math.IsNaN(float64(value)) {
		return value
	}
	return float32(math.Tanh(float64(value)))
}

func interactionRequirementEncoder(channel int) (start, bias, width int) {
	if channel == 1 {
		return interactionRequirementConditionStart, interactionRequirementConditionBias, interactionCaseDim + 1
	}
	return 0, interactionRequirementOutputBias, interactionCaseDim
}

// The original 32 fields remain present. Eleven additional fields bind the
// authored target polarity to input negative/zero/positive flags and eight
// input bytes. Ordinals and choice IDs do not enter these association products.
// The encoder never converts a whole int64 to a floating point number.
func interactionCase(row *[CaseDim]float32, channel int) [interactionCaseDim]float32 {
	polarity := (row[6] - row[4]) * 8
	signs := [3]float32{row[1], row[2], row[3]}
	byteStart := 12
	if channel == 1 {
		polarity = (row[2] - row[1]) * 8
		signs = [3]float32{row[4], row[3], row[0] - row[3] - row[4]}
		byteStart = 5
	}
	var result [interactionCaseDim]float32
	for j, value := range row {
		result[j] = value * interactionInputScale
	}
	for j, value := range signs {
		result[CaseDim+j] = value * interactionInputScale * polarity
	}
	for j := range 8 {
		result[CaseDim+3+j] = row[byteStart+j] * interactionInputScale * polarity
	}
	return result
}

func (m *InteractionRequirementModel) interactionCaseHidden(channel, choice int, row *[CaseDim]float32) [PoolDim]float32 {
	start, bias, width := interactionRequirementEncoder(channel)
	fields := interactionCase(row, channel)
	var result [PoolDim]float32
	for h := range PoolDim {
		sum := m.weights[bias+h]
		for j, value := range fields {
			sum += value * m.weights[start+h*width+j]
		}
		if channel == 1 {
			sum += row[16+choice] * interactionInputScale * m.weights[start+h*width+interactionCaseDim]
		}
		result[h] = interactionTanh(sum)
	}
	return result
}

func (m *InteractionRequirementModel) interactionPool(channel, choice int, w *InteractionRequirementWorkspace) error {
	var sums [PoolDim]float64
	rows := &w.cases[channel]
	for i, row := range rows.rows[:rows.count] {
		hidden := m.interactionCaseHidden(channel, choice, &row)
		if !finite(hidden[:]) {
			return errors.New("nonfinite associated case activation")
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

func interactionTerms(source, output, condition float32) [interactionTermCount]float32 {
	return [interactionTermCount]float32{source, output, condition, source * output,
		source * condition, output * condition, source * output * condition}
}

func (m *InteractionRequirementModel) forwardInteractions(inputs [][OrderedFeatureDim]float32, masks []uint16, w *InteractionRequirementWorkspace) error {
	for choice, input := range inputs {
		for h := range PoolDim {
			sum := m.weights[interactionRequirementContextBias+h]
			for j, value := range input {
				sum += value * interactionInputScale * m.weights[interactionRequirementContextStart+h*OrderedFeatureDim+j]
			}
			w.context[choice][h] = interactionTanh(sum)
		}
		if !finite(w.context[choice][:]) {
			return errors.New("nonfinite source interaction context")
		}
		for channel := range 2 {
			if err := m.interactionPool(channel, choice, w); err != nil {
				return err
			}
		}
		for h := range HiddenDim {
			sum := m.weights[interactionRequirementHiddenBias+h]
			start := interactionRequirementHiddenStart + h*interactionRequirementJointDim
			for j, value := range input {
				sum += value * m.weights[start+j]
			}
			for j := range PoolDim {
				terms := interactionTerms(w.context[choice][j], w.pool[choice][0][j], w.pool[choice][1][j])
				for term, value := range terms {
					sum += value * m.weights[start+OrderedFeatureDim+term*PoolDim+j]
				}
			}
			w.hidden[choice][h] = activate(sum)
		}
		if !finite(w.hidden[choice][:]) {
			return errors.New("nonfinite joint interaction activation")
		}
		for option := range 2 {
			sum := m.weights[interactionRequirementScoreBias+option]
			for h, value := range w.hidden[choice] {
				sum += value * m.weights[interactionRequirementScoreStart+option*HiddenDim+h]
			}
			w.logits[choice][option] = sum
		}
		if !finite(w.logits[choice][:]) {
			return errors.New("nonfinite interaction score")
		}
	}
	for i, mask := range masks {
		for choice := range inputs {
			w.scores[i] += float64(w.logits[choice][mask>>choice&1])
		}
	}
	return nil
}
