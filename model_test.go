package decision

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadVariantsAndPredictIntoDoesNotAllocate(t *testing.T) {
	for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
		t.Run(variant, func(t *testing.T) {
			metadataPath, packedBytes := writeFixtureModel(t, variant, 0.5)
			model, err := Load(metadataPath)
			if err != nil {
				t.Fatal(err)
			}
			if model.Variant() != variant {
				t.Fatalf("variant = %q, want %q", model.Variant(), variant)
			}
			wantResident, wantMatrix, wantBias := 50_912, 50_688, 224
			wantScaleBytes := 0
			if variant != "fp32" {
				wantResident, wantMatrix = 12_896, 12_672
				wantScaleBytes = 8
			}
			if model.PackedFileBytes() != packedBytes || model.ResidentTensorBytes() != wantResident || model.MatrixTensorBytes() != wantMatrix || model.BiasTensorBytes() != wantBias || model.MatrixScaleBytes() != wantScaleBytes {
				t.Fatalf("packed/resident/matrix/bias/scales bytes = %d/%d/%d/%d/%d; want packed=%d resident=%d matrix=%d bias=%d scales=%d", model.PackedFileBytes(), model.ResidentTensorBytes(), model.MatrixTensorBytes(), model.BiasTensorBytes(), model.MatrixScaleBytes(), packedBytes, wantResident, wantMatrix, wantBias, wantScaleBytes)
			}
			var workspace Workspace
			var prediction Prediction
			if err := model.PredictInto("ADD the value to the balance", &workspace, &prediction); err != nil {
				t.Fatal(err)
			}
			if WorkspaceBytes() != 1248 || PredictionValueArrayBytes() != 64 || PredictionBytes() != 80 {
				t.Fatalf("unexpected workspace/prediction bytes = %d/%d/%d", WorkspaceBytes(), PredictionValueArrayBytes(), PredictionBytes())
			}
			if got := model.PredictLabel(&prediction); got != "add" || prediction.Abstained {
				t.Fatalf("prediction = %q, abstained=%t confidence=%f", got, prediction.Abstained, prediction.Confidence)
			}
			if !closeTo(float64(probabilitySum(prediction.Probabilities)), 1, 1e-6) {
				t.Fatalf("probabilities do not sum to one: %.9f", probabilitySum(prediction.Probabilities))
			}
			allocs := testing.AllocsPerRun(1000, func() {
				if err := model.PredictInto("ADD the value to the balance", &workspace, &prediction); err != nil {
					panic(err)
				}
			})
			if allocs != 0 {
				t.Fatalf("valid PredictInto hot path allocated %.2f objects per call", allocs)
			}
		})
	}
}

func TestLoadCapturesSHA256OfExactMetadataSnapshot(t *testing.T) {
	metadataPath, _ := writeFixtureModel(t, "fp32", 0.5)
	raw, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n', ' ', '\n')
	if err := os.WriteFile(metadataPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	model, err := Load(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	want := shaHex(raw)
	if got := model.MetadataSHA256(); got != want {
		t.Fatalf("MetadataSHA256() = %q, want exact loaded-byte digest %q", got, want)
	}
	if got := (*Model)(nil).MetadataSHA256(); got != "" {
		t.Fatalf("nil MetadataSHA256() = %q, want empty string", got)
	}

	if err := os.WriteFile(metadataPath, append(raw, ' '), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := model.MetadataSHA256(); got != want {
		t.Fatalf("loaded snapshot digest changed after file rewrite: got %q, want %q", got, want)
	}
}

func TestFeatureHashingLowercasesASCIIAndNormalizes(t *testing.T) {
	var upper, lower [FeatureDim]float32
	buildFeatures("Add Two", &upper)
	buildFeatures("aDD tWO", &lower)
	if upper != lower {
		t.Fatal("ASCII case folding produced different feature vectors")
	}
	var norm float64
	for _, value := range upper {
		norm += float64(value * value)
	}
	if !closeTo(norm, 1, 1e-6) {
		t.Fatalf("L2 norm squared = %.9f, want 1", norm)
	}
	var short [FeatureDim]float32
	buildFeatures("A", &short)
	for _, value := range short {
		if value != 0 {
			t.Fatal("one-byte input should have no byte bigram/trigram features")
		}
	}
}

func TestPredictRejectsOversizeAndInvalidUTF8(t *testing.T) {
	metadataPath, _ := writeFixtureModel(t, "fp32", 0.5)
	model, err := Load(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var workspace Workspace
	var prediction Prediction
	if err := model.PredictInto(string(make([]byte, InputMaxBytes+1)), &workspace, &prediction); err == nil {
		t.Fatal("oversized input accepted")
	}
	if err := model.PredictInto(string([]byte{0xff, 'a'}), &workspace, &prediction); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestConfidenceThresholdAbstains(t *testing.T) {
	metadataPath, _ := writeFixtureModel(t, "fp32", 0.99)
	model, err := Load(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var workspace Workspace
	var prediction Prediction
	if err := model.PredictInto("add two values", &workspace, &prediction); err != nil {
		t.Fatal(err)
	}
	if !prediction.Abstained || prediction.Confidence >= 0.99 {
		t.Fatalf("expected low-confidence abstention, confidence=%f abstained=%t", prediction.Confidence, prediction.Abstained)
	}
}

func TestDecodeTernaryBase3FiveAndPadding(t *testing.T) {
	// Five trits encode -1, 0, 1, -1, 0 as 0 + 3 + 18 + 0 + 81 = 102.
	// The second byte encodes 1, 0, -1 plus two required zero-padding trits.
	dst := make([]int8, 8)
	if err := decodeTernaryTensor([]byte{102, 113}, dst); err != nil {
		t.Fatal(err)
	}
	want := []int8{-1, 0, 1, -1, 0, 1, 0, -1}
	for i := range want {
		if dst[i] != want[i] {
			t.Fatalf("decoded[%d] = %d, want %d", i, dst[i], want[i])
		}
	}
	if err := decodeTernaryTensor([]byte{243, 0}, dst); err == nil {
		t.Fatal("out-of-range base-3 byte accepted")
	}
	badPadding := make([]int8, 6)
	if err := decodeTernaryTensor([]byte{121, 0}, badPadding); err == nil {
		t.Fatal("nonzero unused padding trit accepted")
	}
}

func TestTernaryPredictUsesInt8FlatMatricesAndPerMatrixScales(t *testing.T) {
	model := &Model{
		metadata:       Metadata{MaxBytes: InputMaxBytes},
		ternaryWeights: make([]int8, FeatureDim*HiddenDim+HiddenDim*LabelCount),
		biases:         make([]float32, HiddenDim+LabelCount),
		w1Scale:        0.25,
		w2Scale:        0.5,
		temperature:    1,
		threshold:      0,
	}
	var features [FeatureDim]float32
	buildFeatures("aa", &features)
	featureIndex := hashNgram("aa", 0, 2)
	model.ternaryWeights[featureIndex] = 1
	model.ternaryWeights[HiddenDim*FeatureDim] = 1
	var workspace Workspace
	var prediction Prediction
	if err := model.PredictInto("aa", &workspace, &prediction); err != nil {
		t.Fatal(err)
	}
	wantLogit := features[featureIndex] * 0.25 * 0.5
	wantProbability := float32(math.Exp(float64(wantLogit)) / (math.Exp(float64(wantLogit)) + LabelCount - 1))
	if !closeTo(float64(prediction.Probabilities[0]), float64(wantProbability), 1e-6) {
		t.Fatalf("first-label probability = %.8f, want %.8f", prediction.Probabilities[0], wantProbability)
	}
	if model.MatrixTensorBytes() != FeatureDim*HiddenDim+HiddenDim*LabelCount || model.BiasTensorBytes() != (HiddenDim+LabelCount)*4 {
		t.Fatalf("unexpected ternary tensor storage: matrix=%d bias=%d", model.MatrixTensorBytes(), model.BiasTensorBytes())
	}
}

func TestLoadRejectsDigestAndUnknownMetadataKeys(t *testing.T) {
	metadataPath, _ := writeFixtureModel(t, "fp32", 0.5)
	metadataRaw, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	weightsPath := filepath.Join(filepath.Dir(metadataPath), "weights.bin")
	weightsRaw, err := os.ReadFile(weightsPath)
	if err != nil {
		t.Fatal(err)
	}
	originalWeights := append([]byte(nil), weightsRaw...)
	weightsRaw[len(weightsRaw)-1] ^= 1
	if err := os.WriteFile(weightsPath, weightsRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(metadataPath); err == nil {
		t.Fatal("weights digest mismatch accepted")
	}
	if err := os.WriteFile(weightsPath, originalWeights, 0o600); err != nil {
		t.Fatal(err)
	}
	metadataRaw = bytes.TrimSpace(metadataRaw)
	metadataRaw = append(metadataRaw[:len(metadataRaw)-1], []byte(`,"unexpected":true}`)...)
	if err := os.WriteFile(metadataPath, metadataRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(metadataPath); err == nil {
		t.Fatal("unknown metadata field accepted")
	}
}

func TestLoadRejectsDuplicateMetadataKeysAndInvalidFloat32Ranges(t *testing.T) {
	metadataPath, _ := writeFixtureModel(t, "fp32", 0.5)
	metadataRaw, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	metadataRaw = bytes.Replace(metadataRaw, []byte(`"variant":"fp32"`), []byte(`"variant":"fp32","variant":"fp32"`), 1)
	if err := os.WriteFile(metadataPath, metadataRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(metadataPath); err == nil {
		t.Fatal("duplicate metadata member was accepted")
	}
	if err := RejectDuplicateJSONKeys([]byte(`{"outer":{"value":1,"value":2}}`)); err == nil {
		t.Fatal("nested duplicate JSON key was accepted")
	}

	labels := Labels()
	if err := validateMetadata(Metadata{Schema: MetadataSchema, Variant: "fp32", FeatureDim: FeatureDim,
		HiddenDim: HiddenDim, MaxBytes: InputMaxBytes, Labels: labels[:], Temperature: math.SmallestNonzeroFloat64}); err == nil {
		t.Fatal("temperature that underflows float32 was accepted")
	}
	if err := validateEncoding("qat_ternary", TensorMetadata{Name: "w1", Count: FeatureDim * HiddenDim,
		Encoding: "ternary_base3_5", Bytes: 2458, Scale: math.MaxFloat64}); err == nil {
		t.Fatal("ternary scale that overflows float32 was accepted")
	}
}

func TestJSONValidationAndLoadRejectRawInvalidUTF8(t *testing.T) {
	invalidJSON := []byte{'{', '"', 'v', '"', ':', '"', 0xff, '"', '}'}
	if err := RejectDuplicateJSONKeys(invalidJSON); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("invalid UTF-8 JSON was not rejected centrally: %v", err)
	}

	metadataPath, _ := writeFixtureModel(t, "fp32", 0.5)
	metadataRaw, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	variantOffset := bytes.Index(metadataRaw, []byte(`"variant":"fp32"`))
	if variantOffset < 0 {
		t.Fatal("fixture metadata variant not found")
	}
	metadataRaw[variantOffset+len(`"variant":"`)] = 0xff
	if err := os.WriteFile(metadataPath, metadataRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(metadataPath); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("model loader accepted raw invalid UTF-8: %v", err)
	}
}

func TestPredictFailsClosedOnFiniteWeightOverflow(t *testing.T) {
	max := float32(math.MaxFloat32)
	large := float32(1e20)
	tinyTemperature := float64(1e-30)
	cases := []struct {
		name        string
		values      map[string]map[int]float32
		temperature *float64
		wantError   string
	}{
		{
			name: "hidden activation overflow",
			values: map[string]map[int]float32{
				"w1": {hashNgram("aa", 0, 2): max},
				"b1": {0: max},
			},
			wantError: "hidden activation is not finite",
		},
		{
			name: "negative hidden activation overflow",
			values: map[string]map[int]float32{
				"w1": {hashNgram("aa", 0, 2): -max},
				"b1": {0: -max},
			},
			wantError: "hidden activation is not finite",
		},
		{
			name: "output logit overflow",
			values: map[string]map[int]float32{
				"b1": {0: large},
				"w2": {0: large},
			},
			wantError: "output logit is not finite",
		},
		{
			name: "calibrated logit overflow",
			values: map[string]map[int]float32{
				"b2": {0: large},
			},
			temperature: &tinyTemperature,
			wantError:   "calibrated logit is not finite",
		},
		{
			name: "late calibrated logit overflow is atomic",
			values: map[string]map[int]float32{
				"b2": {0: 1, LabelCount - 1: large},
			},
			temperature: &tinyTemperature,
			wantError:   "calibrated logit is not finite",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			metadataPath, _ := writeFixtureModel(t, "fp32", 0)
			updateFP32Fixture(t, metadataPath, test.values, test.temperature)
			model, err := Load(metadataPath)
			if err != nil {
				t.Fatalf("finite overflow fixture should load: %v", err)
			}
			var workspace Workspace
			prediction := Prediction{TopIndex: 3, Confidence: 0.75, Abstained: true}
			err = model.PredictInto("aa", &workspace, &prediction)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("PredictInto error = %v; want %q", err, test.wantError)
			}
			if prediction != (Prediction{}) {
				t.Fatalf("failed prediction exposed partial output: %+v", prediction)
			}
		})
	}
}

func updateFP32Fixture(t *testing.T, metadataPath string, values map[string]map[int]float32, temperature *float64) {
	t.Helper()
	metadataRaw, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var metadata Metadata
	if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
		t.Fatal(err)
	}
	weightsPath := filepath.Join(filepath.Dir(metadataPath), metadata.WeightsFile)
	weightsRaw, err := os.ReadFile(weightsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, tensor := range metadata.Tensors {
		for index, value := range values[tensor.Name] {
			if index < 0 || index >= tensor.Count {
				t.Fatalf("fixture tensor index %s[%d] is out of range", tensor.Name, index)
			}
			offset := int(tensor.Offset) + index*4
			binary.LittleEndian.PutUint32(weightsRaw[offset:offset+4], math.Float32bits(value))
		}
	}
	if temperature != nil {
		metadata.Temperature = *temperature
	}
	metadata.WeightsSHA256 = shaHex(weightsRaw)
	metadataRaw, err = json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(weightsPath, weightsRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadataPath, metadataRaw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTensorByteCounts(t *testing.T) {
	if got, err := TensorByteCount(12288, "ternary_base3_5"); err != nil || got != 2458 {
		t.Fatalf("ternary tensor bytes = %d, err=%v", got, err)
	}
	if got, err := TensorByteCount(48, "float32_le"); err != nil || got != 192 {
		t.Fatalf("float tensor bytes = %d, err=%v", got, err)
	}
}

func BenchmarkPredictInto(b *testing.B) {
	for _, variant := range []string{"fp32", "qat_ternary"} {
		b.Run(variant, func(b *testing.B) {
			metadataPath, _ := writeFixtureModel(b, variant, 0.5)
			model, err := Load(metadataPath)
			if err != nil {
				b.Fatal(err)
			}
			var workspace Workspace
			var prediction Prediction
			text := "Multiply the quantity by the unit price"
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := model.PredictInto(text, &workspace, &prediction); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(model.PackedFileBytes()), "packed-B")
			b.ReportMetric(float64(model.MatrixTensorBytes()), "matrix-B")
			b.ReportMetric(float64(model.BiasTensorBytes()), "bias-B")
			b.ReportMetric(float64(model.MatrixScaleBytes()), "scale-B")
			b.ReportMetric(float64(model.ResidentTensorBytes()), "tensor-B")
			b.ReportMetric(float64(WorkspaceBytes()), "workspace-B")
			b.ReportMetric(float64(PredictionValueArrayBytes()), "prediction-arrays-B")
			b.ReportMetric(float64(PredictionBytes()), "prediction-B")
		})
	}
}

type testingTB interface {
	Helper()
	Fatalf(string, ...any)
	TempDir() string
}

func writeFixtureModel(t testingTB, variant string, threshold float64) (string, int) {
	t.Helper()
	dir := t.TempDir()
	segments := tensorSegments()
	labels := Labels()
	metadata := Metadata{
		Schema: MetadataSchema, Variant: variant,
		FeatureDim: FeatureDim, HiddenDim: HiddenDim, MaxBytes: InputMaxBytes,
		Labels: labels[:], Temperature: 1, WeightsFile: "weights.bin",
		Tensors: make([]TensorMetadata, 0, 4), ConfidenceThreshold: &threshold,
	}
	// Populate the biases so the synthetic bundle predicts "add" without text input.
	total := FeatureDim*HiddenDim + HiddenDim + HiddenDim*LabelCount + LabelCount
	decoded := make([]float32, total)
	decoded[total-LabelCount] = 2
	var packed []byte
	for _, name := range []string{"w1", "b1", "w2", "b2"} {
		segment := segments[name]
		encoding := "float32_le"
		if (name == "w1" || name == "w2") && variant != "fp32" {
			encoding = "ternary_base3_5"
		}
		tensor := TensorMetadata{Name: name, Count: segment.count, Rows: segment.rows, Cols: segment.cols,
			Encoding: encoding, Offset: int64(len(packed)), Scale: 1}
		if encoding == "ternary_base3_5" {
			tensor.Scale = 0.25
			n := (segment.count + 4) / 5
			for range n {
				packed = append(packed, 121) // five zero weights, including required padding trits
			}
		} else {
			for i := 0; i < segment.count; i++ {
				packed = binary.LittleEndian.AppendUint32(packed, math.Float32bits(decoded[segment.floatOffset+i]))
			}
		}
		tensor.Bytes = int64(len(packed)) - tensor.Offset
		metadata.Tensors = append(metadata.Tensors, tensor)
	}
	metadata.WeightsSHA256 = shaHex(packed)
	metadataRaw, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal test metadata: %v", err)
	}
	weightsPath := filepath.Join(dir, metadata.WeightsFile)
	if err := os.WriteFile(weightsPath, packed, 0o600); err != nil {
		t.Fatalf("write test weights: %v", err)
	}
	metadataPath := filepath.Join(dir, "model.json")
	if err := os.WriteFile(metadataPath, metadataRaw, 0o600); err != nil {
		t.Fatalf("write test metadata: %v", err)
	}
	return metadataPath, len(packed)
}

func shaHex(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func probabilitySum(values [LabelCount]float32) float32 {
	var sum float32
	for _, value := range values {
		sum += value
	}
	return sum
}

func closeTo(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}
