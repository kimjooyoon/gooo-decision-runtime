package contractdecision

import "errors"

func typedRelativeEncoder(channel int) (start, bias, width int) {
	if channel == 1 {
		return typedConditionStart, typedConditionBias, interactionCaseDim + 1
	}
	return 0, typedOutputBias, interactionCaseDim
}

func (m *typedRelativeHead) caseHidden(channel, choice int, row *[CaseDim]float32) [typedPoolDim]float32 {
	start, bias, width := typedRelativeEncoder(channel)
	fields := interactionCase(row, channel)
	var result [typedPoolDim]float32
	for h := range typedPoolDim {
		sum := m[bias+h]
		for j, value := range fields {
			sum = interactionAffine(sum, value, m[start+h*width+j])
		}
		if channel == 1 {
			sum = interactionAffine(sum, row[16+choice]*interactionInputScale, m[start+h*width+interactionCaseDim])
		}
		result[h] = interactionTanh(sum)
	}
	return result
}

func (m *typedRelativeHead) forward(choice int, input [OrderedFeatureDim]float32, cases *[2]frozenCases, w *typedRelativeChoiceWorkspace) error {
	for h := range typedPoolDim {
		sum := m[typedContextBias+h]
		for j, value := range input {
			sum = interactionAffine(sum, value*interactionInputScale, m[typedContextStart+h*OrderedFeatureDim+j])
		}
		w.context[h] = interactionTanh(sum)
	}
	if !finite(w.context[:]) {
		return errors.New("nonfinite typed source context")
	}
	for channel := range 2 {
		var sums [typedPoolDim]float64
		rows := &cases[channel]
		for _, row := range rows.rows[:rows.count] {
			hidden := m.caseHidden(channel, choice, &row)
			if !finite(hidden[:]) {
				return errors.New("nonfinite typed case activation")
			}
			for h, value := range hidden {
				sums[h] += float64(value)
			}
		}
		if rows.count > 0 {
			for h, sum := range sums {
				w.pool[channel][h] = float32(sum / float64(rows.count))
			}
		}
	}
	for h := range typedHiddenDim {
		sum := m[typedHiddenBias+h]
		start := typedHiddenStart + h*typedJointDim
		for j, value := range input {
			sum = interactionAffine(sum, value, m[start+j])
		}
		for j := range typedPoolDim {
			terms := interactionTerms(w.context[j], w.pool[0][j], w.pool[1][j])
			for term, value := range terms {
				sum = interactionAffine(sum, value, m[start+OrderedFeatureDim+term*typedPoolDim+j])
			}
		}
		w.hidden[h] = activate(sum)
	}
	if !finite(w.hidden[:]) {
		return errors.New("nonfinite typed interaction activation")
	}
	for option := range 2 {
		sum := m[typedScoreBias+option]
		for h, value := range w.hidden {
			sum = interactionAffine(sum, value, m[typedScoreStart+option*typedHiddenDim+h])
		}
		w.logits[option] = sum
	}
	if !finite(w.logits[:]) {
		return errors.New("nonfinite typed interaction score")
	}
	return nil
}
