package jointdecision

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/internal/modelfile"
)

type Metadata struct {
	Schema      string                    `json:"schema"`
	Feature     string                    `json:"feature_version"`
	Arithmetic  string                    `json:"arithmetic_version,omitempty"`
	Variant     string                    `json:"variant"`
	FeatureDim  int                       `json:"feature_dim"`
	HiddenDim   int                       `json:"hidden_dim"`
	MaxBytes    int                       `json:"max_bytes"`
	Labels      []string                  `json:"labels"`
	Temperature float64                   `json:"temperature"`
	WeightsFile string                    `json:"weights_file"`
	WeightsSHA  string                    `json:"weights_sha256"`
	Tensors     []decision.TensorMetadata `json:"tensors"`
}

func digest(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func boundedFile(name string, max int64) ([]byte, error) {
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > max {
		return nil, errors.New("bounded regular joint model file required")
	}
	raw, err := modelfile.ReadChecked(name, max, info)
	if err != nil {
		return nil, fmt.Errorf("joint %w", err)
	}
	return raw, nil
}

type modelContract struct {
	schema, feature              string
	features, labels, inputBytes int
	weightBytes                  int64
}

var twoContract = modelContract{Schema, FeatureVersion, FeatureDim, LabelCount, InputMaxBytes, 64 << 10}

func validate(meta Metadata) error { return validateContract(meta, twoContract) }
func validateContract(meta Metadata, contract modelContract) error {
	if err := validateArithmetic(meta.Arithmetic, contract.schema == ThreeSchema); err != nil {
		return err
	}
	if meta.Schema != contract.schema || meta.Feature != contract.feature || meta.FeatureDim != contract.features || meta.HiddenDim != HiddenDim || meta.MaxBytes != contract.inputBytes ||
		len(meta.Labels) != contract.labels || meta.WeightsFile != "weights.bin" || len(meta.Tensors) != 4 {
		return errors.New("closed joint model dimensions/schema differ")
	}
	for i, label := range meta.Labels {
		if label != fmt.Sprintf("mask_%d", i) {
			return errors.New("canonical joint mask labels required")
		}
	}
	if meta.Variant != "fp32" && meta.Variant != "ptq_ternary" && meta.Variant != "qat_ternary" {
		return errors.New("unknown joint model variant")
	}
	t := float32(meta.Temperature)
	if t <= 0 || meta.Temperature > 100 || math.IsNaN(float64(t)) || math.IsInf(float64(t), 0) {
		return errors.New("finite positive joint temperature required")
	}
	return nil
}
func decodeFloat(raw []byte, values []float32) error {
	if len(raw) != 4*len(values) {
		return errors.New("float extent differs")
	}
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
		if !finite(values[i : i+1]) {
			return errors.New("nonfinite joint weight")
		}
	}
	return nil
}
func decodeTrits(raw []byte, values []int8) error {
	if len(raw) != (len(values)+4)/5 {
		return errors.New("trit extent differs")
	}
	at := 0
	for _, packed := range raw {
		if packed >= 243 {
			return errors.New("invalid five-trit byte")
		}
		for range 5 {
			v := packed % 3
			packed /= 3
			if at < len(values) {
				values[at] = int8(v) - 1
				at++
			} else if v != 1 {
				return errors.New("padding trits must encode zero")
			}
		}
	}
	return nil
}
func layout(meta Metadata, raw []byte, m *Model) error {
	return layoutContract(meta, raw, m, FeatureDim, LabelCount)
}
func layoutContract(meta Metadata, raw []byte, m *Model, features, labels int) error {
	names := [4]string{"w1", "b1", "w2", "b2"}
	rows := [4]int{HiddenDim, 1, labels, 1}
	cols := [4]int{features, HiddenDim, HiddenDim, labels}
	at, floatAt, matrixAt, biasAt := 0, 0, 0, 0
	for i, t := range meta.Tensors {
		matrix := i == 0 || i == 2
		encoding, count := "float32_le", rows[i]*cols[i]
		if matrix && meta.Variant != "fp32" {
			encoding = "ternary_base3_5"
		}
		size := count * 4
		if encoding == "ternary_base3_5" {
			size = (count + 4) / 5
		}
		if t.Name != names[i] || t.Rows != rows[i] || t.Cols != cols[i] || t.Count != count || t.Encoding != encoding || t.Offset != int64(at) || t.Bytes != int64(size) || at+size > len(raw) {
			return errors.New("canonical joint tensor layout differs")
		}
		if encoding == "float32_le" && t.Scale != 1 {
			return errors.New("float tensor scale must be one")
		}
		scale := float32(t.Scale)
		if encoding == "ternary_base3_5" && (scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0)) {
			return errors.New("finite positive trit scale required")
		}
		var err error
		switch {
		case meta.Variant == "fp32":
			err = decodeFloat(raw[at:at+size], m.floatWeights[floatAt:floatAt+count])
			floatAt += count
		case matrix:
			err = decodeTrits(raw[at:at+size], m.codes[matrixAt:matrixAt+count])
			matrixAt += count
			if i == 0 {
				m.w1Scale = scale
			} else {
				m.w2Scale = scale
			}
		default:
			err = decodeFloat(raw[at:at+size], m.biases[biasAt:biasAt+count])
			biasAt += count
		}
		if err != nil {
			return err
		}
		at += size
	}
	if at != len(raw) {
		return errors.New("unreferenced joint weight bytes")
	}
	return nil
}

func Load(name string) (*Model, error) {
	return loadContract(name, twoContract)
}
func loadContract(name string, contract modelContract) (*Model, error) {
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
	if err = validateContract(meta, contract); err != nil {
		return nil, err
	}
	weights, err := boundedFile(filepath.Join(filepath.Dir(name), meta.WeightsFile), contract.weightBytes)
	if err != nil {
		return nil, err
	}
	if digest(weights) != meta.WeightsSHA {
		return nil, errors.New("joint weights digest differs")
	}
	m := &Model{variant: meta.Variant, metadataSHA: digest(raw), weightsSHA: meta.WeightsSHA, arithmetic: meta.Arithmetic, temperature: float32(meta.Temperature), packed: len(weights)}
	if meta.Variant == "fp32" {
		m.floatWeights = make([]float32, contract.features*HiddenDim+HiddenDim+HiddenDim*contract.labels+contract.labels)
	} else {
		m.codes = make([]int8, contract.features*HiddenDim+HiddenDim*contract.labels)
		m.biases = make([]float32, HiddenDim+contract.labels)
	}
	if err = layoutContract(meta, weights, m, contract.features, contract.labels); err != nil {
		return nil, err
	}
	return m, nil
}
