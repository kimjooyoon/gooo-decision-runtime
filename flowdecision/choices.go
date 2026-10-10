package flowdecision

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
)

// ChoicePrediction exposes the additive factors used by PredictInto. A bounded
// search can score its frontier without enumerating all 65,536 possible masks.
// These are raw scores, not calibrated per-choice correctness probabilities.
type ChoicePrediction struct {
	Count  int                    `json:"choice_count"`
	Logits [MaxChoices][2]float32 `json:"choice_logits"`
}

// PredictChoicesInto performs the same neural forward pass as PredictInto.
// Invalid input preserves both caller-owned arrays; valid inference allocates
// zero heap objects. The model and workspace are never retained by a search.
func (m *Model) PredictChoicesInto(inputs [][FeatureDim]float32, workspace *Workspace, output *ChoicePrediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("flow model, workspace and output required")
	}
	if err := validate(inputs, []uint16{0}); err != nil {
		return err
	}
	var candidate Workspace
	if err := m.forward(inputs, nil, &candidate); err != nil {
		return err
	}
	*workspace, *output = candidate, ChoicePrediction{Count: len(inputs), Logits: candidate.logits}
	return nil
}

// Fingerprint binds the feature ABI, architecture schema and exact FP32 weight
// bits. It is stable across JSON formatting. It is not a raw artifact-file hash;
// a loader should retain that separate hash when identifying a downloaded file.
func (m *Model) Fingerprint() string {
	if m == nil {
		return ""
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte(Schema + "\x00" + m.FeatureVersion() + "\x00"))
	var raw [ParameterCount * 4]byte
	for i, w := range m.weights {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(w))
	}
	_, _ = digest.Write(raw[:])
	return hex.EncodeToString(digest.Sum(nil))
}
