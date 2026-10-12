package contractdecision

func (m *typedRelativeHead) backward(choice int, input [OrderedFeatureDim]float32, cases *[2]frozenCases,
	work *typedRelativeChoiceWorkspace, residual [2]float64, gradient *[TypedSourceRelativeHeadParameterCount]float64) {
	for option := range 2 {
		delta := residual[option]
		gradient[typedScoreBias+option] += delta
		for h, value := range work.hidden {
			gradient[typedScoreStart+option*typedHiddenDim+h] += delta * float64(value)
		}
	}
	var termGradient [interactionTermCount][typedPoolDim]float64
	for h, value := range work.hidden {
		delta := 0.0
		for option := range 2 {
			delta += residual[option] * float64(m[typedScoreStart+option*typedHiddenDim+h])
		}
		if value <= 0 {
			delta *= float64(negativeSlope)
		}
		gradient[typedHiddenBias+h] += delta
		start := typedHiddenStart + h*typedJointDim
		for j, value := range input {
			gradient[start+j] += delta * float64(value)
		}
		for j := range typedPoolDim {
			terms := interactionTerms(work.context[j], work.pool[0][j], work.pool[1][j])
			for term, value := range terms {
				index := start + OrderedFeatureDim + term*typedPoolDim + j
				gradient[index] += delta * float64(value)
				termGradient[term][j] += delta * float64(m[index])
			}
		}
	}
	var pooledGradient [2][typedPoolDim]float64
	for h := range typedPoolDim {
		var terms [interactionTermCount]float64
		for term := range terms {
			terms[term] = termGradient[term][h]
		}
		source := float64(work.context[h])
		back := interactionTermGradient(source, float64(work.pool[0][h]), float64(work.pool[1][h]), terms)
		pooledGradient[0][h], pooledGradient[1][h] = back[1], back[2]
		delta := back[0] * (1 - source*source)
		gradient[typedContextBias+h] += delta
		for j, value := range input {
			gradient[typedContextStart+h*OrderedFeatureDim+j] += delta * float64(value*interactionInputScale)
		}
	}
	for channel := range 2 {
		start, bias, width := typedRelativeEncoder(channel)
		rows := &cases[channel]
		for _, row := range rows.rows[:rows.count] {
			hidden := m.caseHidden(channel, choice, &row)
			fields := interactionCase(&row, channel)
			for h, value := range hidden {
				delta := pooledGradient[channel][h] / float64(rows.count) * (1 - float64(value)*float64(value))
				gradient[bias+h] += delta
				for j, x := range fields {
					gradient[start+h*width+j] += delta * float64(x)
				}
				if channel == 1 {
					gradient[start+h*width+interactionCaseDim] += delta * float64(row[16+choice]*interactionInputScale)
				}
			}
		}
	}
}
