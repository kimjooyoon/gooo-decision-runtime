package decision

import (
	"errors"
	"fmt"
)

const (
	DecisionRequestSchema  = "gooo/tiny-ir-decision-request/v1"
	DecisionResponseSchema = "gooo/tiny-ir-decision-response/v1"
)

type DecisionRequest struct {
	Schema string     `json:"schema"`
	Text   string     `json:"text"`
	Left   Identifier `json:"left"`
	Right  Identifier `json:"right"`
}

type LabelProbability struct {
	Label       string  `json:"label"`
	Probability float32 `json:"probability"`
}

type DecisionResponse struct {
	Schema               string             `json:"schema"`
	Status               string             `json:"status"`
	ModelVariant         string             `json:"model_variant"`
	WeightsSHA256        string             `json:"weights_sha256"`
	Probabilities        []LabelProbability `json:"probabilities"`
	BestLabel            string             `json:"best_label"`
	Confidence           float32            `json:"confidence"`
	Threshold            float32            `json:"confidence_threshold"`
	AbstainReason        string             `json:"abstain_reason,omitempty"`
	TypedBinaryIR        *TypedBinaryIR     `json:"typed_binary_ir,omitempty"`
	PackedFileBytes      int                `json:"packed_weights_bytes"`
	DecodedBytes         int                `json:"resident_tensor_bytes"`
	MatrixBytes          int                `json:"matrix_tensor_bytes"`
	BiasBytes            int                `json:"bias_tensor_bytes"`
	MatrixScaleBytes     int                `json:"matrix_scale_storage_bytes"`
	WorkspaceBytes       int                `json:"workspace_scratch_bytes"`
	PredictionBytes      int                `json:"prediction_output_bytes"`
	PredictionValueBytes int                `json:"prediction_value_arrays_bytes"`
}

// Decide maps one closed operation label to typed binary IR. The model only
// supplies an enum; source code and arbitrary expressions are never accepted.
func (m *Model) Decide(request DecisionRequest, workspace *Workspace) (DecisionResponse, error) {
	if m == nil {
		return DecisionResponse{}, errors.New("model is nil")
	}
	if request.Schema != DecisionRequestSchema {
		return DecisionResponse{}, fmt.Errorf("request schema must be %q", DecisionRequestSchema)
	}
	if _, err := BuildTypedBinary("equal", request.Left, request.Right); err != nil {
		return DecisionResponse{}, fmt.Errorf("invalid binary operands: %w", err)
	}
	var prediction Prediction
	if err := m.PredictInto(request.Text, workspace, &prediction); err != nil {
		return DecisionResponse{}, err
	}
	bestLabel := m.PredictLabel(&prediction)
	response := DecisionResponse{
		Schema:          DecisionResponseSchema,
		Status:          "decision",
		ModelVariant:    m.Variant(),
		WeightsSHA256:   m.WeightsSHA256(),
		Probabilities:   make([]LabelProbability, LabelCount),
		BestLabel:       bestLabel,
		Confidence:      prediction.Confidence,
		Threshold:       m.ConfidenceThreshold(),
		PackedFileBytes: m.PackedFileBytes(),
		DecodedBytes:    m.ResidentTensorBytes(),
		MatrixBytes:     m.MatrixTensorBytes(), BiasBytes: m.BiasTensorBytes(), MatrixScaleBytes: m.MatrixScaleBytes(),
		WorkspaceBytes: WorkspaceBytes(), PredictionBytes: PredictionBytes(), PredictionValueBytes: PredictionValueArrayBytes(),
	}
	labels := Labels()
	for i, label := range labels {
		response.Probabilities[i] = LabelProbability{Label: label, Probability: prediction.Probabilities[i]}
	}
	if prediction.Abstained {
		response.Status = "abstained"
		response.AbstainReason = "LOW_CONFIDENCE"
		return response, nil
	}
	ir, err := BuildTypedBinary(bestLabel, request.Left, request.Right)
	if err != nil {
		response.Status = "abstained"
		response.AbstainReason = "TYPE_MISMATCH"
		return response, nil
	}
	response.TypedBinaryIR = &ir
	return response, nil
}
