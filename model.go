// Package decision implements the bounded Go-only tiny IR classifier.
package decision

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
	"unsafe"
)

const (
	MetadataSchema = "gooo/tiny-ir-decision-model/v1"
	FeatureDim     = 256
	HiddenDim      = 48
	LabelCount     = 8
	InputMaxBytes  = 512

	DefaultConfidenceThreshold float32 = 0.5
)

var operationLabels = [LabelCount]string{
	"add", "subtract", "multiply", "less_than", "less_equal", "equal", "and", "or",
}

type TensorMetadata struct {
	Name     string  `json:"name"`
	Count    int     `json:"count"`
	Rows     int     `json:"rows"`
	Cols     int     `json:"cols"`
	Encoding string  `json:"encoding"`
	Offset   int64   `json:"offset"`
	Bytes    int64   `json:"bytes"`
	Scale    float64 `json:"scale"`
}

// Metadata is the deliberately closed export format. Unknown keys are rejected
// by Load so a newer, unreviewed contract cannot silently change runtime rules.
type Metadata struct {
	Schema              string           `json:"schema"`
	Variant             string           `json:"variant"`
	FeatureDim          int              `json:"feature_dim"`
	HiddenDim           int              `json:"hidden_dim"`
	MaxBytes            int              `json:"max_bytes"`
	Labels              []string         `json:"labels"`
	Temperature         float64          `json:"temperature"`
	WeightsFile         string           `json:"weights_file"`
	WeightsSHA256       string           `json:"weights_sha256"`
	Tensors             []TensorMetadata `json:"tensors"`
	ConfidenceThreshold *float64         `json:"confidence_threshold,omitempty"`
}

type Model struct {
	metadata        Metadata
	metadataSHA256  string
	floatWeights    []float32
	ternaryWeights  []int8
	biases          []float32
	w1Scale         float32
	w2Scale         float32
	temperature     float32
	threshold       float32
	weightFileBytes int
}

// RejectDuplicateJSONKeys validates that raw is one complete JSON value and
// contains no repeated object member names. encoding/json otherwise accepts a
// duplicate and silently keeps the last value.
func RejectDuplicateJSONKeys(raw []byte) error {
	if !utf8.Valid(raw) {
		return errors.New("JSON input is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return errors.New("trailing JSON content")
		}
		return err
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object member name is not a string")
			}
			if _, exists := keys[key]; exists {
				return fmt.Errorf("duplicate JSON object key %q", key)
			}
			keys[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim('}') {
			return errors.New("unterminated JSON object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim(']') {
			return errors.New("unterminated JSON array")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
	return nil
}

// Workspace is caller-owned scratch space. Keep one per concurrent worker;
// sharing a workspace across simultaneous PredictInto calls is not safe.
type Workspace struct {
	features [FeatureDim]float32
	hidden   [HiddenDim]float32
	logits   [LabelCount]float32
}

type Prediction struct {
	Probabilities [LabelCount]float32
	Logits        [LabelCount]float32
	TopIndex      int
	Confidence    float32
	Abstained     bool
}

func Labels() [LabelCount]string { return operationLabels }

func (m *Model) Variant() string { return m.metadata.Variant }

func (m *Model) ConfidenceThreshold() float32 { return m.threshold }

func (m *Model) ResidentTensorBytes() int {
	if m == nil {
		return 0
	}
	return len(m.floatWeights)*4 + len(m.ternaryWeights) + len(m.biases)*4
}

// DecodedTensorBytes is kept as a compatibility alias; it reports actual
// resident matrix and bias tensor storage, which is int8 for ternary matrices.
func (m *Model) DecodedTensorBytes() int { return m.ResidentTensorBytes() }

func (m *Model) MatrixTensorBytes() int {
	if m == nil {
		return 0
	}
	if len(m.floatWeights) != 0 {
		return (FeatureDim*HiddenDim + HiddenDim*LabelCount) * 4
	}
	return len(m.ternaryWeights)
}

func (m *Model) BiasTensorBytes() int {
	if m == nil {
		return 0
	}
	if len(m.floatWeights) != 0 {
		return (HiddenDim + LabelCount) * 4
	}
	return len(m.biases) * 4
}

func (m *Model) MatrixScaleBytes() int {
	if m == nil || len(m.ternaryWeights) == 0 {
		return 0
	}
	return 2 * 4
}

// WorkspaceBytes reports the fixed per-worker arrays used by PredictInto.
func WorkspaceBytes() int {
	return (FeatureDim + HiddenDim + LabelCount) * 4
}

// PredictionValueArrayBytes reports the fixed logits and probability arrays.
func PredictionValueArrayBytes() int { return LabelCount * 8 }

// PredictionBytes reports the complete caller-owned Prediction struct size.
func PredictionBytes() int { return int(unsafe.Sizeof(Prediction{})) }

func (m *Model) PackedFileBytes() int {
	if m == nil {
		return 0
	}
	return m.weightFileBytes
}

func (m *Model) WeightsSHA256() string {
	if m == nil {
		return ""
	}
	return m.metadata.WeightsSHA256
}

// MetadataSHA256 reports the digest of the exact metadata bytes read by Load.
func (m *Model) MetadataSHA256() string {
	if m == nil {
		return ""
	}
	return m.metadataSHA256
}

func metadataDigestHex(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// Load reads strict JSON metadata and a sibling packed tensor file, verifies
// the file digest and every tensor extent, then decodes weights once. FP32
// bundles use one flat float32 tensor slice; ternary bundles use one flat int8
// matrix slice plus float32 biases.
func Load(metadataPath string) (*Model, error) {
	metadataAbs, err := filepath.Abs(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("resolve model metadata path: %w", err)
	}
	metadataInfo, err := os.Lstat(metadataAbs)
	if err != nil {
		return nil, fmt.Errorf("stat model metadata: %w", err)
	}
	if !metadataInfo.Mode().IsRegular() {
		return nil, errors.New("model metadata must be a regular file")
	}
	if metadataInfo.Size() <= 0 || metadataInfo.Size() > 64*1024 {
		return nil, errors.New("model metadata file size is outside the allowed range")
	}
	metadataRaw, err := os.ReadFile(metadataAbs)
	if err != nil {
		return nil, fmt.Errorf("read model metadata: %w", err)
	}
	if err := RejectDuplicateJSONKeys(metadataRaw); err != nil {
		return nil, fmt.Errorf("model metadata: %w", err)
	}
	var metadata Metadata
	decoder := json.NewDecoder(bytes.NewReader(metadataRaw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decode model metadata: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err == nil {
		return nil, errors.New("model metadata contains trailing JSON content")
	} else if err != io.EOF {
		return nil, fmt.Errorf("model metadata has trailing content: %w", err)
	}
	if err := validateMetadata(metadata); err != nil {
		return nil, err
	}

	metadataDir := filepath.Dir(metadataAbs)
	if filepath.Base(metadata.WeightsFile) != metadata.WeightsFile || metadata.WeightsFile == "." || metadata.WeightsFile == ".." {
		return nil, errors.New("weights_file must be a sibling filename")
	}
	weightsPath := filepath.Join(metadataDir, metadata.WeightsFile)
	weightsInfo, err := os.Lstat(weightsPath)
	if err != nil {
		return nil, fmt.Errorf("stat weights file: %w", err)
	}
	if !weightsInfo.Mode().IsRegular() || weightsInfo.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("weights file must be a regular non-symlink file")
	}
	if weightsInfo.Size() <= 0 || weightsInfo.Size() > 1024*1024 {
		return nil, errors.New("weights file size is outside the allowed range")
	}
	weightsRaw, err := os.ReadFile(weightsPath)
	if err != nil {
		return nil, fmt.Errorf("read weights file: %w", err)
	}
	digest := sha256.Sum256(weightsRaw)
	if hex.EncodeToString(digest[:]) != metadata.WeightsSHA256 {
		return nil, errors.New("weights_sha256 does not match weights_file bytes")
	}

	const totalValues = FeatureDim*HiddenDim + HiddenDim + HiddenDim*LabelCount + LabelCount
	const matrixValues = FeatureDim*HiddenDim + HiddenDim*LabelCount
	var floatWeights []float32
	var ternaryWeights []int8
	var biases []float32
	if metadata.Variant == "fp32" {
		floatWeights = make([]float32, totalValues)
	} else {
		ternaryWeights = make([]int8, matrixValues)
		biases = make([]float32, HiddenDim+LabelCount)
	}
	segments := tensorSegments()
	seen := [4]bool{}
	covered := make([]bool, len(weightsRaw))
	for _, tensor := range metadata.Tensors {
		segment, ok := segments[tensor.Name]
		if !ok {
			return nil, fmt.Errorf("unknown tensor name %q", tensor.Name)
		}
		if seen[segment.index] {
			return nil, fmt.Errorf("duplicate tensor %q", tensor.Name)
		}
		seen[segment.index] = true
		if tensor.Count != segment.count || tensor.Rows != segment.rows || tensor.Cols != segment.cols {
			return nil, fmt.Errorf("tensor %s shape/count mismatch", tensor.Name)
		}
		if tensor.Offset < 0 || tensor.Bytes < 0 || tensor.Offset > int64(len(weightsRaw)) || tensor.Bytes > int64(len(weightsRaw))-tensor.Offset {
			return nil, fmt.Errorf("tensor %s byte range is outside weights file", tensor.Name)
		}
		start, end := int(tensor.Offset), int(tensor.Offset+tensor.Bytes)
		for _, alreadyCovered := range covered[start:end] {
			if alreadyCovered {
				return nil, fmt.Errorf("tensor %s overlaps another tensor", tensor.Name)
			}
		}
		for i := start; i < end; i++ {
			covered[i] = true
		}
		if err := validateEncoding(metadata.Variant, tensor); err != nil {
			return nil, err
		}
		switch {
		case metadata.Variant == "fp32":
			if err := decodeFloat32Tensor(weightsRaw[start:end], floatWeights[segment.floatOffset:segment.floatOffset+segment.count]); err != nil {
				return nil, fmt.Errorf("decode tensor %s: %w", tensor.Name, err)
			}
		case tensor.Name == "w1" || tensor.Name == "w2":
			if err := decodeTernaryTensor(weightsRaw[start:end], ternaryWeights[segment.matrixOffset:segment.matrixOffset+segment.count]); err != nil {
				return nil, fmt.Errorf("decode tensor %s: %w", tensor.Name, err)
			}
		case tensor.Name == "b1" || tensor.Name == "b2":
			if err := decodeFloat32Tensor(weightsRaw[start:end], biases[segment.biasOffset:segment.biasOffset+segment.count]); err != nil {
				return nil, fmt.Errorf("decode tensor %s: %w", tensor.Name, err)
			}
		}
	}
	for i, present := range seen {
		if !present {
			return nil, fmt.Errorf("required tensor %q is missing", [...]string{"w1", "b1", "w2", "b2"}[i])
		}
	}
	for _, present := range covered {
		if !present {
			return nil, errors.New("weights file contains unreferenced bytes")
		}
	}

	threshold := float32(DefaultConfidenceThreshold)
	if metadata.ConfidenceThreshold != nil {
		threshold = float32(*metadata.ConfidenceThreshold)
	}
	model := &Model{
		metadata: metadata, metadataSHA256: metadataDigestHex(metadataRaw), floatWeights: floatWeights, ternaryWeights: ternaryWeights,
		biases: biases, temperature: float32(metadata.Temperature), threshold: threshold,
		weightFileBytes: len(weightsRaw),
	}
	if metadata.Variant != "fp32" {
		for _, tensor := range metadata.Tensors {
			switch tensor.Name {
			case "w1":
				model.w1Scale = float32(tensor.Scale)
			case "w2":
				model.w2Scale = float32(tensor.Scale)
			}
		}
	}
	return model, nil
}

func validateMetadata(metadata Metadata) error {
	if metadata.Schema != MetadataSchema || metadata.FeatureDim != FeatureDim || metadata.HiddenDim != HiddenDim || metadata.MaxBytes != InputMaxBytes {
		return errors.New("model metadata schema or fixed dimensions do not match the supported contract")
	}
	if metadata.Variant != "fp32" && metadata.Variant != "ptq_ternary" && metadata.Variant != "qat_ternary" {
		return fmt.Errorf("unsupported model variant %q", metadata.Variant)
	}
	if len(metadata.Labels) != LabelCount {
		return errors.New("model label count must be exactly eight")
	}
	for i, label := range operationLabels {
		if metadata.Labels[i] != label {
			return fmt.Errorf("model label %d must be %q", i, label)
		}
	}
	if math.IsNaN(metadata.Temperature) || math.IsInf(metadata.Temperature, 0) || metadata.Temperature <= 0 || metadata.Temperature > 100 {
		return errors.New("temperature must be finite and in (0, 100]")
	}
	if temperature := float32(metadata.Temperature); temperature <= 0 || math.IsNaN(float64(temperature)) || math.IsInf(float64(temperature), 0) {
		return errors.New("temperature must remain positive and finite as float32")
	}
	if metadata.ConfidenceThreshold != nil && (math.IsNaN(*metadata.ConfidenceThreshold) || math.IsInf(*metadata.ConfidenceThreshold, 0) || *metadata.ConfidenceThreshold < 0 || *metadata.ConfidenceThreshold > 1) {
		return errors.New("confidence_threshold must be finite and between 0 and 1")
	}
	if metadata.WeightsFile == "" || len(metadata.WeightsSHA256) != 64 {
		return errors.New("weights_file and 64-character weights_sha256 are required")
	}
	if _, err := hex.DecodeString(metadata.WeightsSHA256); err != nil || strings.ToLower(metadata.WeightsSHA256) != metadata.WeightsSHA256 {
		return errors.New("weights_sha256 must be lowercase hexadecimal")
	}
	if len(metadata.Tensors) != 4 {
		return errors.New("metadata must declare exactly four tensors")
	}
	return nil
}

type tensorSegment struct {
	index        int
	count        int
	rows         int
	cols         int
	floatOffset  int
	matrixOffset int
	biasOffset   int
}

func tensorSegments() map[string]tensorSegment {
	w1Count := FeatureDim * HiddenDim
	b1Offset := w1Count
	w2Offset := b1Offset + HiddenDim
	b2Offset := w2Offset + HiddenDim*LabelCount
	return map[string]tensorSegment{
		"w1": {index: 0, count: w1Count, rows: HiddenDim, cols: FeatureDim, floatOffset: 0, matrixOffset: 0},
		"b1": {index: 1, count: HiddenDim, rows: 1, cols: HiddenDim, floatOffset: b1Offset, biasOffset: 0},
		"w2": {index: 2, count: HiddenDim * LabelCount, rows: LabelCount, cols: HiddenDim, floatOffset: w2Offset, matrixOffset: w1Count},
		"b2": {index: 3, count: LabelCount, rows: 1, cols: LabelCount, floatOffset: b2Offset, biasOffset: HiddenDim},
	}
}

func validateEncoding(variant string, tensor TensorMetadata) error {
	want := "float32_le"
	if tensor.Name == "w1" || tensor.Name == "w2" {
		if variant != "fp32" {
			want = "ternary_base3_5"
		}
	}
	if tensor.Encoding != want {
		return fmt.Errorf("tensor %s encoding %q; want %q for variant %s", tensor.Name, tensor.Encoding, want, variant)
	}
	var wantBytes int64
	if want == "float32_le" {
		wantBytes = int64(tensor.Count) * 4
	} else {
		wantBytes = int64((tensor.Count + 4) / 5)
		scale32 := float32(tensor.Scale)
		if math.IsNaN(tensor.Scale) || math.IsInf(tensor.Scale, 0) || tensor.Scale <= 0 || scale32 <= 0 || math.IsInf(float64(scale32), 0) {
			return fmt.Errorf("tensor %s ternary scale must be positive and finite as float32", tensor.Name)
		}
	}
	if tensor.Bytes != wantBytes {
		return fmt.Errorf("tensor %s byte count %d; want %d", tensor.Name, tensor.Bytes, wantBytes)
	}
	if want == "float32_le" && (math.IsNaN(tensor.Scale) || math.IsInf(tensor.Scale, 0) || tensor.Scale != 1) {
		return fmt.Errorf("tensor %s float32 scale must be exactly 1", tensor.Name)
	}
	return nil
}

func decodeFloat32Tensor(raw []byte, dst []float32) error {
	if len(raw) != len(dst)*4 {
		return errors.New("float32 tensor length mismatch")
	}
	for i := range dst {
		value := math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4 : i*4+4]))
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("weight %d is not finite", i)
		}
		dst[i] = value
	}
	return nil
}

func decodeTernaryTensor(raw []byte, dst []int8) error {
	if len(raw) != (len(dst)+4)/5 {
		return errors.New("ternary tensor length mismatch")
	}
	at := 0
	for byteIndex, packed := range raw {
		if packed >= 243 {
			return fmt.Errorf("packed base-3 byte %d exceeds five-trit range", byteIndex)
		}
		value := int(packed)
		for tritIndex := 0; tritIndex < 5; tritIndex++ {
			trit := value % 3
			value /= 3
			if at < len(dst) {
				dst[at] = int8(trit - 1)
				at++
			} else if trit != 1 {
				return fmt.Errorf("unused padding trit %d must encode zero", tritIndex)
			}
		}
	}
	if at != len(dst) {
		return errors.New("ternary tensor decoded count mismatch")
	}
	return nil
}

// PredictInto hashes lowercased-ASCII byte bigrams/trigrams into the fixed
// feature vector, runs the two dense layers, and writes calibrated probabilities
// into caller-owned arrays. The valid hot path allocates no heap memory.
func (m *Model) PredictInto(text string, workspace *Workspace, output *Prediction) error {
	if m == nil {
		return errors.New("model is nil")
	}
	if workspace == nil || output == nil {
		return errors.New("workspace and prediction output are required")
	}
	destination := output
	*destination = Prediction{}
	if len(m.floatWeights) != 0 {
		if len(m.floatWeights) != FeatureDim*HiddenDim+HiddenDim+HiddenDim*LabelCount+LabelCount {
			return errors.New("model float tensor layout is uninitialized or invalid")
		}
	} else if len(m.ternaryWeights) != FeatureDim*HiddenDim+HiddenDim*LabelCount || len(m.biases) != HiddenDim+LabelCount {
		return errors.New("model ternary tensor layout is uninitialized or invalid")
	}
	var candidate Prediction
	output = &candidate
	if err := FeaturesInto(text, &workspace.features); err != nil {
		return err
	}
	for i := 0; i < HiddenDim; i++ {
		var bias, dot float32
		start := i * FeatureDim
		if len(m.floatWeights) != 0 {
			bias = m.floatWeights[FeatureDim*HiddenDim+i]
			for j := 0; j < FeatureDim; j++ {
				dot += m.floatWeights[start+j] * workspace.features[j]
			}
		} else {
			bias = m.biases[i]
			for j := 0; j < FeatureDim; j++ {
				dot += float32(m.ternaryWeights[start+j]) * workspace.features[j]
			}
			dot *= m.w1Scale
		}
		activation := bias + dot
		if !finiteFloat32(activation) {
			return errors.New("hidden activation is not finite")
		}
		if activation < 0 {
			activation = 0
		}
		workspace.hidden[i] = activation
	}
	for i := 0; i < LabelCount; i++ {
		var logit float32
		if len(m.floatWeights) != 0 {
			biasOffset := FeatureDim*HiddenDim + HiddenDim + HiddenDim*LabelCount + i
			matrixOffset := FeatureDim*HiddenDim + HiddenDim + i*HiddenDim
			var dot float32
			for j := 0; j < HiddenDim; j++ {
				dot += m.floatWeights[matrixOffset+j] * workspace.hidden[j]
			}
			logit = m.floatWeights[biasOffset] + dot
		} else {
			matrixOffset := HiddenDim*FeatureDim + i*HiddenDim
			var dot float32
			for j := 0; j < HiddenDim; j++ {
				dot += float32(m.ternaryWeights[matrixOffset+j]) * workspace.hidden[j]
			}
			logit = m.biases[HiddenDim+i] + dot*m.w2Scale
		}
		if !finiteFloat32(logit) {
			return errors.New("output logit is not finite")
		}
		scaledLogit := logit / m.temperature
		if !finiteFloat32(scaledLogit) {
			return errors.New("calibrated logit is not finite")
		}
		output.Logits[i] = logit
		workspace.logits[i] = scaledLogit
	}
	maxLogit := workspace.logits[0]
	for i := 1; i < LabelCount; i++ {
		if workspace.logits[i] > maxLogit {
			maxLogit = workspace.logits[i]
		}
	}
	var expSum float64
	for i, logit := range workspace.logits {
		value := math.Exp(float64(logit - maxLogit))
		probability := float32(value)
		if math.IsNaN(value) || math.IsInf(value, 0) || !finiteFloat32(probability) {
			return errors.New("softmax produced a nonfinite probability")
		}
		output.Probabilities[i] = probability
		expSum += value
	}
	if expSum == 0 || math.IsNaN(expSum) || math.IsInf(expSum, 0) {
		return errors.New("softmax produced an invalid probability sum")
	}
	output.TopIndex = 0
	for i := range output.Probabilities {
		probability := float32(float64(output.Probabilities[i]) / expSum)
		if !finiteFloat32(probability) {
			return errors.New("normalized probability is not finite")
		}
		output.Probabilities[i] = probability
		if output.Probabilities[i] > output.Probabilities[output.TopIndex] {
			output.TopIndex = i
		}
	}
	output.Confidence = output.Probabilities[output.TopIndex]
	output.Abstained = output.Confidence < m.threshold
	*destination = *output
	return nil
}

func finiteFloat32(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

// FeaturesInto reproduces the pinned byte bigram/trigram feature contract in a
// caller-owned array. It accepts only nonempty, valid UTF-8 input up to 512
// bytes and allocates nothing on the valid path.
func FeaturesInto(text string, output *[FeatureDim]float32) error {
	if output == nil {
		return errors.New("feature output is required")
	}
	if len(text) == 0 {
		return errors.New("instruction text is empty")
	}
	if len(text) > InputMaxBytes {
		return fmt.Errorf("instruction exceeds %d UTF-8 bytes", InputMaxBytes)
	}
	if !validUTF8(text) {
		return errors.New("instruction is not valid UTF-8")
	}
	buildFeatures(text, output)
	return nil
}

func (m *Model) PredictLabel(output *Prediction) string {
	if m == nil || output == nil || output.TopIndex < 0 || output.TopIndex >= LabelCount {
		return ""
	}
	return operationLabels[output.TopIndex]
}

func buildFeatures(text string, features *[FeatureDim]float32) {
	clear(features[:])
	for i := 0; i+1 < len(text); i++ {
		features[hashNgram(text, i, 2)]++
	}
	for i := 0; i+2 < len(text); i++ {
		features[hashNgram(text, i, 3)]++
	}
	var squareSum float64
	for _, count := range features {
		squareSum += float64(count * count)
	}
	if squareSum == 0 {
		return
	}
	inverseNorm := float32(1 / math.Sqrt(squareSum))
	for i := range features {
		features[i] *= inverseNorm
	}
}

func hashNgram(text string, start, count int) int {
	hash := uint32(2166136261)
	for i := 0; i < count; i++ {
		value := text[start+i]
		if value >= 'A' && value <= 'Z' {
			value += 'a' - 'A'
		}
		hash ^= uint32(value)
		hash *= 16777619
	}
	return int(hash & (FeatureDim - 1))
}

func validUTF8(text string) bool {
	return utf8.ValidString(text)
}

func TensorByteCount(count int, encoding string) (int64, error) {
	if count < 0 {
		return 0, errors.New("tensor count cannot be negative")
	}
	switch encoding {
	case "float32_le":
		return int64(count) * 4, nil
	case "ternary_base3_5":
		return int64((count + 4) / 5), nil
	default:
		return 0, fmt.Errorf("unsupported tensor encoding %q", encoding)
	}
}
