package contractdecision

func interactionTermGradient(s, o, c float64, gradient [interactionTermCount]float64) [3]float64 {
	return [3]float64{
		gradient[0] + gradient[3]*o + gradient[4]*c + gradient[6]*o*c,
		gradient[1] + gradient[3]*s + gradient[5]*c + gradient[6]*s*c,
		gradient[2] + gradient[4]*s + gradient[5]*o + gradient[6]*s*o,
	}
}

func (m *InteractionRequirementModel) backwardInteractions(inputs [][OrderedFeatureDim]float32, work *InteractionRequirementWorkspace,
	residual *[MaxChoices][2]float64, gradient *[InteractionRequirementParameterCount]float64) {
	for choice, input := range inputs {
		for option := range 2 {
			delta := residual[choice][option]
			gradient[interactionRequirementScoreBias+option] += delta
			for h, value := range work.hidden[choice] {
				gradient[interactionRequirementScoreStart+option*HiddenDim+h] += delta * float64(value)
			}
		}
		var termGradient [interactionTermCount][PoolDim]float64
		for h, value := range work.hidden[choice] {
			delta := 0.0
			for option := range 2 {
				delta += residual[choice][option] * float64(m.weights[interactionRequirementScoreStart+option*HiddenDim+h])
			}
			if value <= 0 {
				delta *= float64(negativeSlope)
			}
			gradient[interactionRequirementHiddenBias+h] += delta
			start := interactionRequirementHiddenStart + h*interactionRequirementJointDim
			for j, value := range input {
				gradient[start+j] += delta * float64(value)
			}
			for j := range PoolDim {
				terms := interactionTerms(work.context[choice][j], work.pool[choice][0][j], work.pool[choice][1][j])
				for term, value := range terms {
					index := start + OrderedFeatureDim + term*PoolDim + j
					gradient[index] += delta * float64(value)
					termGradient[term][j] += delta * float64(m.weights[index])
				}
			}
		}
		var pooledGradient [2][PoolDim]float64
		for h := range PoolDim {
			var terms [interactionTermCount]float64
			for term := range terms {
				terms[term] = termGradient[term][h]
			}
			source := float64(work.context[choice][h])
			back := interactionTermGradient(source, float64(work.pool[choice][0][h]), float64(work.pool[choice][1][h]), terms)
			pooledGradient[0][h], pooledGradient[1][h] = back[1], back[2]
			delta := back[0] * (1 - source*source)
			gradient[interactionRequirementContextBias+h] += delta
			for j, value := range input {
				gradient[interactionRequirementContextStart+h*OrderedFeatureDim+j] += delta * float64(value*interactionInputScale)
			}
		}
		for channel := range 2 {
			start, bias, width := interactionRequirementEncoder(channel)
			rows := &work.cases[channel]
			for i, row := range rows.rows[:rows.count] {
				hidden := m.interactionCaseHidden(channel, choice, &row)
				fields := interactionCase(&row, channel)
				for h, value := range hidden {
					delta := pooledGradient[channel][h] / float64(rows.count)
					if m.extreme {
						if work.winner[choice][channel][h] != i {
							continue
						}
						delta = pooledGradient[channel][h]
					}
					delta *= 1 - float64(value)*float64(value)
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
}
