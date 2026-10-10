package contractdecision

func (m *OrderedRequirementModel) backwardRequirements(inputs [][OrderedFeatureDim]float32, work *OrderedRequirementWorkspace,
	residual *[MaxChoices][2]float64, gradient *[OrderedRequirementParameterCount]float64) {
	for choice, input := range inputs {
		for option := range 2 {
			delta := residual[choice][option]
			gradient[orderedRequirementScoreBias+option] += delta
			for h, value := range work.hidden[choice] {
				gradient[orderedRequirementScoreStart+option*HiddenDim+h] += delta * float64(value)
			}
		}
		var pooledGradient [2][PoolDim]float64
		for h, value := range work.hidden[choice] {
			delta := 0.0
			for option := range 2 {
				delta += residual[choice][option] * float64(m.weights[orderedRequirementScoreStart+option*HiddenDim+h])
			}
			if value <= 0 {
				delta *= float64(negativeSlope)
			}
			gradient[orderedRequirementHiddenBias+h] += delta
			start := orderedRequirementHiddenStart + h*orderedRequirementJointDim
			for j, value := range input {
				gradient[start+j] += delta * float64(value)
			}
			for channel := range 2 {
				for j, value := range work.pool[choice][channel] {
					index := start + OrderedFeatureDim + channel*PoolDim + j
					gradient[index] += delta * float64(value)
					pooledGradient[channel][j] += delta * float64(m.weights[index])
				}
			}
		}
		var contextGradient [PoolDim]float64
		for channel := range 2 {
			start, bias, width := orderedRequirementEncoder(channel)
			rows := &work.cases[channel]
			for i, row := range rows.rows[:rows.count] {
				hidden := m.orderedRequirementCaseHidden(channel, choice, &row, &work.context[choice])
				for h, value := range hidden {
					delta := pooledGradient[channel][h] / float64(rows.count)
					if m.extreme {
						if work.winner[choice][channel][h] != i {
							continue
						}
						delta = pooledGradient[channel][h]
					}
					if value <= 0 {
						delta *= float64(negativeSlope)
					}
					gradient[bias+h] += delta
					contextGradient[h] += delta
					for j, x := range row {
						gradient[start+h*width+j] += delta * float64(x)
					}
					if channel == 1 {
						gradient[start+h*width+ConditionDim] += delta * float64(row[16+choice])
					}
				}
			}
		}
		for h, delta := range contextGradient {
			for j, value := range input {
				gradient[orderedRequirementContextStart+h*OrderedFeatureDim+j] += delta * float64(value)
			}
		}
	}
}
