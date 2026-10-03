package orderjudge

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/kimjooyoon/gooo-decision-runtime/internal/strictjson"
)

type Metadata struct {
	Schema        string `json:"schema"`
	Feature       string `json:"feature_version"`
	Arithmetic    string `json:"arithmetic_version"`
	Parameters    int    `json:"parameters"`
	IntentDim     int    `json:"intent_dim"`
	SourceDim     int    `json:"source_dim"`
	MaxInputBytes int    `json:"max_input_bytes"`
	WeightsBytes  int    `json:"weights_bytes"`
	WeightsSHA    string `json:"weights_sha256"`
	WeightsFile   string `json:"weights_file"`
}

func (m *Model) Marshal() ([]byte, []byte, error) {
	if m == nil {
		return nil, nil, errors.New("model required")
	}
	raw := make([]byte, ParameterCount*4)
	for i, w := range m.weights {
		binary.LittleEndian.PutUint32(raw[4*i:], math.Float32bits(w))
	}
	meta := Metadata{Schema, FeatureVersion, ArithmeticVersion, ParameterCount, IntentDim, SourceDim, MaxInputBytes, len(raw), digest(raw), "weights.bin"}
	encoded, err := json.MarshalIndent(meta, "", "  ")
	return append(encoded, '\n'), raw, err
}

// Load rejects metadata drift, altered tensors, trailing input and nonfinite
// values. File reads and their byte bounds remain the caller's responsibility.
func Load(meta, raw []byte) (*Model, error) {
	if len(meta) > 4096 || len(raw) != ParameterCount*4 {
		return nil, errors.New("bounded model artifact required")
	}
	var m Metadata
	if err := strictjson.Decode(meta, &m); err != nil {
		return nil, err
	}
	if m.Schema != Schema || m.Feature != FeatureVersion || m.Arithmetic != ArithmeticVersion || m.Parameters != ParameterCount ||
		m.IntentDim != IntentDim || m.SourceDim != SourceDim || m.MaxInputBytes != MaxInputBytes || m.WeightsBytes != len(raw) ||
		m.WeightsSHA != digest(raw) || m.WeightsFile != "weights.bin" {
		return nil, errors.New("model metadata or digest differs")
	}
	var weights [ParameterCount]float32
	for i := range weights {
		weights[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	return New(weights)
}

func digest(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
