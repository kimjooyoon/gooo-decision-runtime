package jointdecision

const (
	SharedThreeSchema = "gooo/tiny-shared-three-choice-path-model/v1"
	SharedHiddenDim   = 8
	sharedFeatures    = 256
	sharedW1          = SharedHiddenDim * sharedFeatures
	sharedW2          = 2 * SharedHiddenDim
)

// Each input part reuses the same judge without retaining expanded zero blocks.
func (m *ThreeModel) sharedFirst(x *[ThreeFeatureDim]float32, y *[HiddenDim]float32) {
	for part := range 3 {
		for row := range SharedHiddenDim {
			var sum float32
			for col := range sharedFeatures {
				v := x[part*sharedFeatures+col]
				if len(m.inner.codes) == 0 {
					sum += v * m.inner.floatWeights[row*sharedFeatures+col]
				} else {
					sum += v * float32(m.inner.codes[row*sharedFeatures+col])
				}
			}
			if len(m.inner.codes) == 0 {
				sum += m.inner.floatWeights[sharedW1+row]
			} else {
				sum = sum*m.inner.w1Scale + m.inner.biases[row]
			}
			y[part*SharedHiddenDim+row] = max(sum, 0)
		}
	}
}

// Preserve the expanded model's single 24-term sum. Summing three precomputed
// local scores would regroup floating-point additions and change its decisions.
func (m *ThreeModel) sharedLast(x *[HiddenDim]float32, y *[ThreeLabelCount]float32) {
	for mask := range ThreeLabelCount {
		var sum float32
		for col, v := range x {
			bit := (mask >> (col / SharedHiddenDim)) & 1
			local := bit*SharedHiddenDim + col%SharedHiddenDim
			if len(m.inner.codes) == 0 {
				sum += v * m.inner.floatWeights[sharedW1+SharedHiddenDim+local]
			} else {
				sum += v * float32(m.inner.codes[sharedW1+local])
			}
		}
		if len(m.inner.codes) != 0 {
			sum *= m.inner.w2Scale
		}
		// The removed output bias is canonical positive zero, including its
		// effect on a negative zero produced by underflow.
		y[mask] = sum + float32(0)
	}
}
