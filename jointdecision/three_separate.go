package jointdecision

import "errors"

// SeparateArithmeticVersion rounds each product before the following addition.
// Empty metadata retains the original Go arithmetic, including permitted fusion.
// Softmax still uses the Go math package and the original float64 normalization.
const SeparateArithmeticVersion = "float32_separate_v1"

func validateArithmetic(version string, three bool) error {
	if version != "" && (!three || version != SeparateArithmeticVersion) {
		return errors.New("unsupported joint arithmetic version")
	}
	return nil
}

// Explicit conversions are semantic rounding barriers, including when inlined.
// An intermediate variable alone would still permit fusion across statements.
// https://go.dev/ref/spec#Floating_point_operators
func addProduct32(sum, x, weight float32) float32 {
	return float32(sum + float32(x*weight))
}

func separateDot(x, weights []float32) float32 {
	var sum float32
	for i, value := range x {
		sum = addProduct32(sum, value, weights[i])
	}
	return sum
}

func separateTritDot(x []float32, weights []int8) float32 {
	var sum float32
	for i, value := range x {
		sum = addProduct32(sum, value, float32(weights[i]))
	}
	return sum
}

func (m *ThreeModel) separateFirst(x *[ThreeFeatureDim]float32, y *[HiddenDim]float32) {
	width, biasStart := ThreeFeatureDim, ThreeFeatureDim*HiddenDim
	if m.shared {
		width, biasStart = sharedFeatures, sharedW1
	}
	for row := range y {
		local, start := row, 0
		if m.shared {
			local, start = row%SharedHiddenDim, row/SharedHiddenDim*sharedFeatures
		}
		part, at := x[start:start+width], local*width
		var sum float32
		if len(m.inner.codes) == 0 {
			sum = separateDot(part, m.inner.floatWeights[at:at+width])
			sum = float32(sum + m.inner.floatWeights[biasStart+local])
		} else {
			sum = separateTritDot(part, m.inner.codes[at:at+width])
			sum = addProduct32(m.inner.biases[local], sum, m.inner.w1Scale)
		}
		y[row] = max(sum, 0)
	}
}

func (m *ThreeModel) separateLast(x *[HiddenDim]float32, y *[ThreeLabelCount]float32) {
	if m.shared {
		m.separateSharedLast(x, y)
		return
	}
	for row := range y {
		var sum float32
		if len(m.inner.codes) == 0 {
			start := ThreeFeatureDim*HiddenDim + HiddenDim + row*HiddenDim
			sum = separateDot(x[:], m.inner.floatWeights[start:start+HiddenDim])
			sum = float32(sum + m.inner.floatWeights[ThreeFeatureDim*HiddenDim+HiddenDim+HiddenDim*ThreeLabelCount+row])
		} else {
			start := ThreeFeatureDim*HiddenDim + row*HiddenDim
			sum = separateTritDot(x[:], m.inner.codes[start:start+HiddenDim])
			sum = addProduct32(m.inner.biases[HiddenDim+row], sum, m.inner.w2Scale)
		}
		y[row] = sum
	}
}

func (m *ThreeModel) separateSharedLast(x *[HiddenDim]float32, y *[ThreeLabelCount]float32) {
	for mask := range y {
		var sum float32
		for col, value := range x {
			bit := (mask >> (col / SharedHiddenDim)) & 1
			local := bit*SharedHiddenDim + col%SharedHiddenDim
			var weight float32
			if len(m.inner.codes) == 0 {
				weight = m.inner.floatWeights[sharedW1+SharedHiddenDim+local]
			} else {
				weight = float32(m.inner.codes[sharedW1+local])
			}
			sum = addProduct32(sum, value, weight)
		}
		if len(m.inner.codes) != 0 {
			sum = float32(sum * m.inner.w2Scale)
		}
		y[mask] = float32(sum + float32(0))
	}
}
