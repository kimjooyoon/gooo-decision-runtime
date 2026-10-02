package jointdecision

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/kimjooyoon/gooo-decision-runtime"
)

// CompactThree exports a loaded expanded model only after proving every omitted
// value is a canonical zero or an exact tied copy. It does not train, requantize,
// mutate the model, write files, or claim the expanded artifact's identity.
func CompactThree(m *ThreeModel) (Metadata, []byte, error) {
	if m == nil || m.inner == nil || m.shared {
		return Metadata{}, nil, errors.New("expanded three-choice model required")
	}
	if err := verifySharedTies(m.inner); err != nil {
		return Metadata{}, nil, err
	}
	meta := Metadata{Schema: SharedThreeSchema, Feature: m.FeatureVersion(), Arithmetic: m.ArithmeticVersion(), Variant: m.Variant(),
		FeatureDim: ThreeFeatureDim, HiddenDim: SharedHiddenDim, MaxBytes: ThreeInputMaxBytes,
		Temperature: float64(m.inner.temperature), WeightsFile: "weights.bin"}
	for i := range ThreeLabelCount {
		meta.Labels = append(meta.Labels, fmt.Sprintf("mask_%d", i))
	}
	var raw []byte
	for i, name := range [3]string{"w1", "b1", "w2"} {
		rows, cols := [3]int{8, 1, 2}[i], [3]int{256, 8, 8}[i]
		count, encoding, scale := rows*cols, "float32_le", float64(1)
		var block []byte
		if m.Variant() == "fp32" || i == 1 {
			block = make([]byte, 4*count)
			for j := range count {
				var value float32
				if m.Variant() != "fp32" {
					value = m.inner.biases[j]
				} else {
					at := sharedSourceIndex(i, j)
					value = m.inner.floatWeights[at]
				}
				binary.LittleEndian.PutUint32(block[4*j:], math.Float32bits(value))
			}
		} else {
			encoding = "ternary_base3_5"
			if i == 0 {
				scale = float64(m.inner.w1Scale)
			} else {
				scale = float64(m.inner.w2Scale)
			}
			block = make([]byte, (count+4)/5)
			for packed := range block {
				power := byte(1)
				for digit := range 5 {
					j, value := packed*5+digit, int8(0)
					if j < count {
						at := sharedSourceIndex(i, j)
						if i == 2 {
							at -= HiddenDim
						}
						value = m.inner.codes[at]
					}
					block[packed] += byte(value+1) * power
					power *= 3
				}
			}
		}
		meta.Tensors = append(meta.Tensors, decision.TensorMetadata{Name: name, Rows: rows, Cols: cols, Count: count,
			Encoding: encoding, Offset: int64(len(raw)), Bytes: int64(len(block)), Scale: scale})
		raw = append(raw, block...)
	}
	meta.WeightsSHA = digest(raw)
	return meta, raw, nil
}

func sharedSourceIndex(tensor, local int) int {
	switch tensor {
	case 0:
		return (local/sharedFeatures)*ThreeFeatureDim + local%sharedFeatures
	case 1:
		return ThreeFeatureDim*HiddenDim + local
	default:
		return ThreeFeatureDim*HiddenDim + HiddenDim + (local/SharedHiddenDim)*HiddenDim + local%SharedHiddenDim
	}
}

func verifySharedTies(m *Model) error {
	floating := m.variant == "fp32"
	if floating && len(m.floatWeights) != ThreeFeatureDim*HiddenDim+HiddenDim+HiddenDim*ThreeLabelCount+ThreeLabelCount ||
		!floating && (len(m.codes) != ThreeFeatureDim*HiddenDim+HiddenDim*ThreeLabelCount || len(m.biases) != HiddenDim+ThreeLabelCount) {
		return errors.New("expanded tensor extents differ")
	}
	value := func(at int) uint32 {
		if floating {
			return math.Float32bits(m.floatWeights[at])
		}
		return uint32(int32(m.codes[at]))
	}
	for row := range HiddenDim {
		for col := range ThreeFeatureDim {
			want := uint32(0)
			if col/sharedFeatures == row/SharedHiddenDim {
				want = value((row%SharedHiddenDim)*ThreeFeatureDim + col%sharedFeatures)
			}
			if value(row*ThreeFeatureDim+col) != want {
				return errors.New("input matrix is not exactly shared with positive-zero off-blocks")
			}
		}
		var got, want float32
		if floating {
			got, want = m.floatWeights[ThreeFeatureDim*HiddenDim+row], m.floatWeights[ThreeFeatureDim*HiddenDim+row%SharedHiddenDim]
		} else {
			got, want = m.biases[row], m.biases[row%SharedHiddenDim]
		}
		if math.Float32bits(got) != math.Float32bits(want) {
			return errors.New("hidden bias is not exactly shared")
		}
	}
	start := ThreeFeatureDim * HiddenDim
	if floating {
		start += HiddenDim
	}
	for mask := range ThreeLabelCount {
		for col := range HiddenDim {
			bit := (mask >> (col / SharedHiddenDim)) & 1
			if value(start+mask*HiddenDim+col) != value(start+bit*HiddenDim+col%SharedHiddenDim) {
				return errors.New("output bit rows are not exactly shared")
			}
		}
		var bias float32
		if floating {
			bias = m.floatWeights[start+HiddenDim*ThreeLabelCount+mask]
		} else {
			bias = m.biases[HiddenDim+mask]
		}
		if math.Float32bits(bias) != 0 {
			return errors.New("removed output bias must be positive zero")
		}
	}
	return nil
}
