package criteriadecision

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"

	"github.com/kimjooyoon/gooo-decision-runtime/criteriainput"
)

func digest(data []byte) string {
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}

// Load owns a copy of finite little-endian float32 parameters. Metadata binds
// the exact feature schema, architecture and weights. It establishes artifact
// identity; training and model quality require their own observations.
func Load(metadata, weights []byte) (*Model, error) {
	if len(metadata) == 0 || len(metadata) > 8192 || len(weights) != WeightBytes {
		return nil, errors.New("complete bounded scalar criteria model artifact required")
	}
	var info Metadata
	decoder := json.NewDecoder(bytes.NewReader(metadata))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&info); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("model metadata contains trailing data")
	}
	if info.Schema != Schema || info.FeatureSchema != criteriainput.FeatureSchema ||
		info.Architecture != Architecture || info.Variant != "fp32" || info.HiddenDim != HiddenDim ||
		info.Parameters != ParameterCount || info.WeightsSHA256 != digest(weights) {
		return nil, errors.New("model feature, parameter or weight identity differs")
	}
	m := &Model{metadata: info, metadataSHA256: digest(metadata)}
	for i := range m.weights {
		value := math.Float32frombits(binary.LittleEndian.Uint32(weights[i*4 : i*4+4]))
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, errors.New("finite model parameters required")
		}
		m.weights[i] = value
	}
	return m, nil
}

// EncodeArtifact binds caller-supplied parameters to this runtime contract. It
// does not fit parameters or label the resulting artifact as a trained model.
func EncodeArtifact(parameters *[ParameterCount]float32) ([]byte, []byte, error) {
	if parameters == nil {
		return nil, nil, errors.New("complete model parameters required")
	}
	weights := make([]byte, WeightBytes)
	for i, value := range parameters {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, nil, errors.New("finite model parameters required")
		}
		binary.LittleEndian.PutUint32(weights[i*4:i*4+4], math.Float32bits(value))
	}
	metadata, err := json.Marshal(Metadata{Schema: Schema, FeatureSchema: criteriainput.FeatureSchema,
		Architecture: Architecture, Variant: "fp32", HiddenDim: HiddenDim,
		Parameters: ParameterCount, WeightsSHA256: digest(weights)})
	return metadata, weights, err
}
