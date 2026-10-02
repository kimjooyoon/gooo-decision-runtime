package jointdecision

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"

	"github.com/kimjooyoon/gooo-decision-runtime"
)

// LoadSharedThree loads an explicit tied-weight ABI. LoadThree continues to
// require the expanded ABI; format selection is never inferred from byte length.
func LoadSharedThree(name string) (*ThreeModel, error) {
	raw, err := boundedFile(name, 64<<10)
	if err != nil {
		return nil, err
	}
	if err = decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	var meta Metadata
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&meta); err != nil {
		return nil, err
	}
	if err = validateShared(meta); err != nil {
		return nil, err
	}
	weights, err := boundedFile(filepath.Join(filepath.Dir(name), meta.WeightsFile), 16<<10)
	if err != nil {
		return nil, err
	}
	if digest(weights) != meta.WeightsSHA {
		return nil, errors.New("shared weights digest differs")
	}
	m := &Model{variant: meta.Variant, metadataSHA: digest(raw), weightsSHA: meta.WeightsSHA, arithmetic: meta.Arithmetic,
		temperature: float32(meta.Temperature), packed: len(weights)}
	if meta.Variant == "fp32" {
		m.floatWeights = make([]float32, sharedW1+SharedHiddenDim+sharedW2)
	} else {
		m.codes = make([]int8, sharedW1+sharedW2)
		m.biases = make([]float32, SharedHiddenDim)
	}
	if err = sharedLayout(meta, weights, m); err != nil {
		return nil, err
	}
	return &ThreeModel{inner: m, shared: true, feature: meta.Feature}, nil
}

func validateShared(meta Metadata) error {
	if err := validateArithmetic(meta.Arithmetic, true); err != nil {
		return err
	}
	if meta.Schema != SharedThreeSchema || (meta.Feature != ThreeFeatureVersion && meta.Feature != ThreeBagFeatureVersion) || meta.FeatureDim != ThreeFeatureDim ||
		meta.HiddenDim != SharedHiddenDim || meta.MaxBytes != ThreeInputMaxBytes || len(meta.Labels) != ThreeLabelCount ||
		meta.WeightsFile != "weights.bin" || len(meta.Tensors) != 3 {
		return errors.New("closed shared three-choice dimensions/schema differ")
	}
	for i, label := range meta.Labels {
		if label != fmt.Sprintf("mask_%d", i) {
			return errors.New("canonical shared labels required")
		}
	}
	if meta.Variant != "fp32" && meta.Variant != "ptq_ternary" && meta.Variant != "qat_ternary" {
		return errors.New("unknown shared variant")
	}
	t := float32(meta.Temperature)
	if t <= 0 || meta.Temperature > 100 || !finite([]float32{t}) {
		return errors.New("finite positive shared temperature required")
	}
	return nil
}

func sharedLayout(meta Metadata, raw []byte, m *Model) error {
	at, floatAt, matrixAt := 0, 0, 0
	for i, t := range meta.Tensors {
		rows, cols := [3]int{8, 1, 2}[i], [3]int{256, 8, 8}[i]
		count, encoding := rows*cols, "float32_le"
		if i != 1 && meta.Variant != "fp32" {
			encoding = "ternary_base3_5"
		}
		size := count * 4
		if encoding == "ternary_base3_5" {
			size = (count + 4) / 5
		}
		if t.Name != [3]string{"w1", "b1", "w2"}[i] || t.Rows != rows || t.Cols != cols || t.Count != count ||
			t.Encoding != encoding || t.Offset != int64(at) || t.Bytes != int64(size) || at+size > len(raw) {
			return errors.New("canonical shared tensor layout differs")
		}
		if encoding == "float32_le" && t.Scale != 1 {
			return errors.New("shared float scale must be one")
		}
		scale := float32(t.Scale)
		if encoding == "ternary_base3_5" && (scale <= 0 || math.IsInf(float64(scale), 0) || math.IsNaN(float64(scale))) {
			return errors.New("finite positive shared trit scale required")
		}
		var err error
		switch {
		case meta.Variant == "fp32":
			err = decodeFloat(raw[at:at+size], m.floatWeights[floatAt:floatAt+count])
			floatAt += count
		case i != 1:
			err = decodeTrits(raw[at:at+size], m.codes[matrixAt:matrixAt+count])
			matrixAt += count
			if i == 0 {
				m.w1Scale = scale
			} else {
				m.w2Scale = scale
			}
		default:
			err = decodeFloat(raw[at:at+size], m.biases)
		}
		if err != nil {
			return err
		}
		at += size
	}
	if at != len(raw) {
		return errors.New("unreferenced shared weight bytes")
	}
	return nil
}
