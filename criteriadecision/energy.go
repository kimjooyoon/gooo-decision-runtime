package criteriadecision

import "math"

func (m *Model) encode(input []float32, weights, bias int, output *[HiddenDim]float32) {
	for i := range HiddenDim {
		sum := float64(m.weights[bias+i])
		for j, value := range input {
			sum += float64(m.weights[weights+i*len(input)+j]) * float64(value)
		}
		output[i] = float32(math.Tanh(sum))
	}
}

func (m *Model) combine(choice, context, output *[HiddenDim]float32) {
	for i := range HiddenDim {
		sum := float64(m.weights[mixBias+i])
		for j := range HiddenDim {
			sum += float64(m.weights[mixWeights+i*2*HiddenDim+j]) * float64(choice[j])
			sum += float64(m.weights[mixWeights+i*2*HiddenDim+HiddenDim+j]) * float64(context[j])
		}
		output[i] = float32(math.Tanh(sum))
	}
}

func (m *Model) energy(mask uint16, choices int, work *Workspace) float64 {
	var score float64
	for i := range choices {
		option := int(mask >> i & 1)
		score += float64(m.weights[unaryBias+option])
		for j, value := range work.combined[i] {
			score += float64(m.weights[unaryWeights+option*HiddenDim+j]) * float64(value)
		}
		for other := range i {
			sign := float64(1)
			if (mask>>i)&1 != (mask>>other)&1 {
				sign = -1
			}
			for j, value := range work.combined[i] {
				score += sign * float64(m.weights[pairWeights+j]) * float64(value) *
					float64(work.combined[other][j]) / math.Sqrt(HiddenDim)
			}
		}
	}
	return score
}
