package decision_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime"
)

func TestPublicConsumerLoadsSyntheticBundlesAndBuildsTypedIR(t *testing.T) {
	for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
		t.Run(variant, func(t *testing.T) {
			modelPath := writeSyntheticBundle(t, variant)
			model, err := decision.Load(modelPath)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if model.Variant() != variant {
				t.Fatalf("Variant() = %q, want %q", model.Variant(), variant)
			}
			packedBytes, residentBytes, matrixBytes, scaleBytes := 50_912, 50_912, 50_688, 0
			if variant != "fp32" {
				packedBytes, residentBytes, matrixBytes, scaleBytes = 2_759, 12_896, 12_672, 8
			}
			if model.PackedFileBytes() != packedBytes || model.ResidentTensorBytes() != residentBytes || model.MatrixTensorBytes() != matrixBytes || model.MatrixScaleBytes() != scaleBytes {
				t.Fatalf("packed/resident/matrix/scale bytes=%d/%d/%d/%d, want %d/%d/%d/%d", model.PackedFileBytes(), model.ResidentTensorBytes(), model.MatrixTensorBytes(), model.MatrixScaleBytes(), packedBytes, residentBytes, matrixBytes, scaleBytes)
			}

			var workspace decision.Workspace
			var prediction decision.Prediction
			if err := model.PredictInto("Add this amount to the balance", &workspace, &prediction); err != nil {
				t.Fatalf("PredictInto: %v", err)
			}
			if label := model.PredictLabel(&prediction); label != "add" || prediction.Abstained {
				t.Fatalf("label=%q abstained=%t, want add and accepted", label, prediction.Abstained)
			}

			request := decision.DecisionRequest{
				Schema: decision.DecisionRequestSchema,
				Text:   "Add this amount to the balance",
				Left:   decision.Identifier{Name: "amount", Type: decision.TypeInt},
				Right:  decision.Identifier{Name: "balance", Type: decision.TypeInt},
			}
			response, err := model.Decide(request, &workspace)
			if err != nil {
				t.Fatalf("Decide: %v", err)
			}
			if response.Status != "decision" || response.BestLabel != "add" || response.TypedBinaryIR == nil {
				t.Fatalf("unexpected decision response: %+v", response)
			}
			if response.TypedBinaryIR.Operation != "add" || response.TypedBinaryIR.ResultType != decision.TypeInt {
				t.Fatalf("unexpected typed operation: %+v", response.TypedBinaryIR)
			}
			if decision.WorkspaceBytes() != 1248 || decision.PredictionBytes() != 80 {
				t.Fatalf("workspace/prediction bytes=%d/%d, want 1248/80", decision.WorkspaceBytes(), decision.PredictionBytes())
			}

			allocations := testing.AllocsPerRun(100, func() {
				if err := model.PredictInto("Add this amount to the balance", &workspace, &prediction); err != nil {
					panic(err)
				}
			})
			if allocations != 0 {
				t.Fatalf("PredictInto hot path allocations=%g, want zero", allocations)
			}
		})
	}
}

func TestTypedBinaryBuilderRejectsOpenEndedOrIllTypedOperations(t *testing.T) {
	left := decision.Identifier{Name: "left", Type: decision.TypeInt}
	right := decision.Identifier{Name: "right", Type: decision.TypeInt}
	ir, err := decision.BuildTypedBinary("less_equal", left, right)
	if err != nil {
		t.Fatalf("BuildTypedBinary: %v", err)
	}
	expression, err := decision.AssembleGoExpression(ir)
	if err != nil || expression != "left <= right" {
		t.Fatalf("expression=%q err=%v, want typed comparison", expression, err)
	}
	if _, err := decision.BuildTypedBinary("arbitrary_source", left, right); err == nil {
		t.Fatal("unsupported operation was accepted")
	}
	if _, err := decision.BuildTypedBinary("and", left, right); err == nil {
		t.Fatal("ill-typed Boolean operation was accepted")
	}
}

func TestModelCanBeSharedWithOneWorkspacePerWorker(t *testing.T) {
	model, err := decision.Load(writeSyntheticBundle(t, "qat_ternary"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	const workers = 8
	const callsPerWorker = 20
	errorsFound := make(chan error, workers)
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			var workspace decision.Workspace
			var prediction decision.Prediction
			for call := 0; call < callsPerWorker; call++ {
				if err := model.PredictInto("Multiply the item count by the unit price", &workspace, &prediction); err != nil {
					errorsFound <- err
					return
				}
				if model.PredictLabel(&prediction) != "add" {
					errorsFound <- errUnexpectedLabel
					return
				}
			}
		}()
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
}

func TestRejectDuplicateJSONKeysIsPublicAndNested(t *testing.T) {
	if err := decision.RejectDuplicateJSONKeys([]byte(`{"outer":{"label":"add","label":"or"}}`)); err == nil {
		t.Fatal("nested duplicate JSON key was accepted")
	}
	if err := decision.RejectDuplicateJSONKeys([]byte(`{"outer":{"label":"add"}}`)); err != nil {
		t.Fatalf("valid JSON was rejected: %v", err)
	}
}

func TestPublicMetadataSHA256BindsExactLoadedSnapshot(t *testing.T) {
	metadataPath := writeSyntheticBundle(t, "fp32")
	raw, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n', ' ', '\n')
	if err := os.WriteFile(metadataPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	model, err := decision.Load(metadataPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	digest := sha256.Sum256(raw)
	want := hex.EncodeToString(digest[:])
	if got := model.MetadataSHA256(); got != want {
		t.Fatalf("MetadataSHA256() = %q, want exact loaded-byte digest %q", got, want)
	}

	if err := os.WriteFile(metadataPath, append(raw, ' '), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := model.MetadataSHA256(); got != want {
		t.Fatalf("loaded metadata digest changed after file rewrite: got %q, want %q", got, want)
	}
}

var errUnexpectedLabel = errors.New("expected deterministic add label")

func writeSyntheticBundle(t *testing.T, variant string) string {
	t.Helper()
	labels := decision.Labels()
	threshold := 0.1
	metadata := decision.Metadata{
		Schema: decision.MetadataSchema, Variant: variant,
		FeatureDim: decision.FeatureDim, HiddenDim: decision.HiddenDim, MaxBytes: decision.InputMaxBytes,
		Labels: labels[:], Temperature: 1, WeightsFile: "weights.bin", ConfidenceThreshold: &threshold,
	}
	tensors := []struct {
		name  string
		count int
		rows  int
		cols  int
	}{
		{"w1", decision.FeatureDim * decision.HiddenDim, decision.HiddenDim, decision.FeatureDim},
		{"b1", decision.HiddenDim, 1, decision.HiddenDim},
		{"w2", decision.HiddenDim * decision.LabelCount, decision.LabelCount, decision.HiddenDim},
		{"b2", decision.LabelCount, 1, decision.LabelCount},
	}
	var weights []byte
	for _, tensor := range tensors {
		encoding := "float32_le"
		var raw []byte
		if variant != "fp32" && (tensor.name == "w1" || tensor.name == "w2") {
			encoding = "ternary_base3_5"
			raw = bytes.Repeat([]byte{121}, (tensor.count+4)/5)
		} else {
			raw = make([]byte, tensor.count*4)
		}
		scale := 1.0
		if encoding == "ternary_base3_5" {
			scale = 0.25
		}
		metadata.Tensors = append(metadata.Tensors, decision.TensorMetadata{
			Name: tensor.name, Count: tensor.count, Rows: tensor.rows, Cols: tensor.cols,
			Encoding: encoding, Offset: int64(len(weights)), Bytes: int64(len(raw)), Scale: scale,
		})
		weights = append(weights, raw...)
	}
	// A zero-valued bundle gives a deterministic tie whose first closed label is add.
	if variant == "fp32" {
		for i := 0; i < decision.LabelCount; i++ {
			binary.LittleEndian.PutUint32(weights[len(weights)-decision.LabelCount*4+i*4:], 0)
		}
	}
	digest := sha256.Sum256(weights)
	metadata.WeightsSHA256 = hex.EncodeToString(digest[:])
	metadataRaw, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal synthetic metadata: %v", err)
	}
	dir := t.TempDir()
	weightsPath := filepath.Join(dir, metadata.WeightsFile)
	if err := os.WriteFile(weightsPath, weights, 0o600); err != nil {
		t.Fatalf("write synthetic weights: %v", err)
	}
	metadataPath := filepath.Join(dir, "model.json")
	if err := os.WriteFile(metadataPath, metadataRaw, 0o600); err != nil {
		t.Fatalf("write synthetic metadata: %v", err)
	}
	return metadataPath
}
