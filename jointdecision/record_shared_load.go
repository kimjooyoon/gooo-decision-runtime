package jointdecision

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// RecordSharedFeatureVersion keeps the complete record feature projection while
// using a shared 256/8/2 judge for the three ordered choices. It needs weights
// trained for record expressions; frozen integer shared contracts stay separate.
const RecordSharedFeatureVersion = "triple_record_field_context_v1_shared_v1"

func LoadRecordSharedThree(name string) (*ThreeModel, error) {
	return loadRecordSharedContract(name, RecordSharedFeatureVersion, ThreeInputMaxBytes)
}

// LoadRecordOriginSharedThree requires weights trained for the distinct source
// origin projection. Shape equality does not make older weights compatible.
func LoadRecordOriginSharedThree(name string) (*ThreeModel, error) {
	return loadRecordSharedContract(name, RecordOriginSharedFeatureVersion, RecordOriginInputMaxBytes)
}

func loadRecordSharedContract(name, feature string, maxBytes int) (*ThreeModel, error) {
	raw, err := boundedFile(name, 64<<10)
	if err != nil {
		return nil, err
	}
	if err = decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	var meta Metadata
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&meta); err != nil {
		return nil, err
	}
	if meta.Feature != feature || meta.MaxBytes != maxBytes || meta.Arithmetic != SeparateArithmeticVersion {
		return nil, errors.New("explicit shared record feature and arithmetic contract required")
	}
	// Dimensions/layout are the existing compact shared ABI. Validate a local
	// descriptor against that ABI while retaining the real immutable metadata.
	shape := meta
	shape.Feature, shape.MaxBytes = ThreeFeatureVersion, ThreeInputMaxBytes
	if err = validateShared(shape); err != nil {
		return nil, err
	}
	weights, err := boundedFile(filepath.Join(filepath.Dir(name), meta.WeightsFile), 16<<10)
	if err != nil {
		return nil, err
	}
	if digest(weights) != meta.WeightsSHA {
		return nil, errors.New("shared record weights digest differs")
	}
	m := &Model{variant: meta.Variant, metadataSHA: digest(raw), weightsSHA: meta.WeightsSHA,
		arithmetic: meta.Arithmetic, temperature: float32(meta.Temperature), packed: len(weights)}
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
