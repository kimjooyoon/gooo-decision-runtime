// Package flowdecision learns finite candidate rankings from source-bound
// condition, output and static value-flow inputs. A shared small network scores each choice; full candidate
// masks retain the combinations against which learning and selection are scored.
package flowdecision

import (
	"errors"
	"math"
	"slices"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const (
	FeatureDim     = decision.ExecutionFlowFeatureDim
	HiddenDim      = 24
	MaxChoices     = 16
	MaxCandidates  = 64
	ParameterCount = FeatureDim*HiddenDim + HiddenDim + HiddenDim*2 + 2
	w2Start        = FeatureDim*HiddenDim + HiddenDim
	b2Start        = w2Start + HiddenDim*2
)

// Model is immutable after construction. It owns 9,290 FP32 values (37,160B).
// These figures cover weights, not total process RAM or training storage.
type Model struct {
	weights        [ParameterCount]float32
	featureVersion string
}

type Workspace struct {
	hidden [MaxChoices][HiddenDim]float32
	logits [MaxChoices][2]float32
	scores [MaxCandidates]float64
}

type Prediction struct {
	Count         int                    `json:"candidate_count"`
	Selected      uint16                 `json:"selected_mask"`
	Probabilities [MaxCandidates]float32 `json:"candidate_probabilities"`
}

func New(weights [ParameterCount]float32) (*Model, error) {
	return NewForFeatures(weights, decision.ExecutionFlowFeatureVersion)
}

// NewForFeatures binds immutable weights to an explicit input representation.
// Merely relabelling previously trained weights does not train the new channels.
func NewForFeatures(weights [ParameterCount]float32, version string) (*Model, error) {
	if !supportedFeatures(version) {
		return nil, errors.New("unsupported flow feature version")
	}
	if !finite(weights[:]) {
		return nil, errors.New("finite flow model weights required")
	}
	return &Model{weights: weights, featureVersion: version}, nil
}

func supportedFeatures(version string) bool {
	return version == decision.ExecutionFlowFeatureVersion || version == decision.SemanticFlowFeatureVersion ||
		version == decision.RelationalFlowFeatureVersion
}

// FeatureVersion identifies the 384-cell value-flow representation.
func (m *Model) FeatureVersion() string {
	if m == nil {
		return ""
	}
	if m.featureVersion == "" {
		return decision.ExecutionFlowFeatureVersion
	}
	return m.featureVersion
}

func (m *Model) Weights() [ParameterCount]float32 { return m.weights }

func finite(values []float32) bool {
	for _, x := range values {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return false
		}
	}
	return true
}

func validate(inputs [][FeatureDim]float32, masks []uint16) error {
	if len(inputs) == 0 || len(inputs) > MaxChoices || len(masks) == 0 || len(masks) > MaxCandidates {
		return errors.New("flow ranking needs 1..16 choices and 1..64 candidates")
	}
	for _, input := range inputs {
		if !finite(input[:]) {
			return errors.New("nonfinite flow model input")
		}
	}
	for i, mask := range masks {
		if uint32(mask)>>len(inputs) != 0 {
			return errors.New("candidate names an undeclared choice")
		}
		if slices.Contains(masks[:i], mask) {
			return errors.New("duplicate candidate mask")
		}
	}
	return nil
}

func (m *Model) forward(inputs [][FeatureDim]float32, masks []uint16, w *Workspace) error {
	for choice, input := range inputs {
		for h := range HiddenDim {
			sum := m.weights[FeatureDim*HiddenDim+h]
			for j, x := range input {
				sum += x * m.weights[h*FeatureDim+j]
			}
			w.hidden[choice][h] = max(sum, 0)
		}
		if !finite(w.hidden[choice][:]) {
			return errors.New("nonfinite hidden activation")
		}
		for option := range 2 {
			sum := m.weights[b2Start+option]
			for h, x := range w.hidden[choice] {
				sum += x * m.weights[w2Start+option*HiddenDim+h]
			}
			w.logits[choice][option] = sum
		}
		if !finite(w.logits[choice][:]) {
			return errors.New("nonfinite choice score")
		}
	}
	for i, mask := range masks {
		var score float64
		for choice := range inputs {
			score += float64(w.logits[choice][mask>>choice&1])
		}
		w.scores[i] = score
	}
	return nil
}

// distribution uses an independently shifted exponential for each allowed set.
// A very low-scoring correct set therefore does not disappear by underflow.
func distribution(scores []float64, allowed uint64) ([MaxCandidates]float64, float64) {
	var p [MaxCandidates]float64
	maximum := math.Inf(-1)
	for i, score := range scores {
		if allowed>>i&1 != 0 {
			maximum = math.Max(maximum, score)
		}
	}
	total := 0.0
	for i, score := range scores {
		if allowed>>i&1 != 0 {
			p[i] = math.Exp(score - maximum)
			total += p[i]
		}
	}
	for i := range scores {
		p[i] /= total
	}
	return p, maximum + math.Log(total)
}

func allCandidates(n int) uint64 { return ^uint64(0) >> (64 - n) }

// PredictInto ranks only the supplied complete masks. Scores are relative to
// that finite pool and are not calibrated correctness probabilities. Ties use
// the smaller mask. Separate caller workspaces allow concurrent, allocation-free
// inference. Invalid input leaves workspace and output unchanged.
func (m *Model) PredictInto(inputs [][FeatureDim]float32, masks []uint16, workspace *Workspace, output *Prediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("flow model, workspace and output required")
	}
	if err := validate(inputs, masks); err != nil {
		return err
	}
	var candidate Workspace
	if err := m.forward(inputs, masks, &candidate); err != nil {
		return err
	}
	probabilities, _ := distribution(candidate.scores[:len(masks)], allCandidates(len(masks)))
	prediction := Prediction{Count: len(masks), Selected: masks[0]}
	best := 0
	for i, p := range probabilities[:len(masks)] {
		prediction.Probabilities[i] = float32(p)
		if candidate.scores[i] > candidate.scores[best] || (candidate.scores[i] == candidate.scores[best] && masks[i] < masks[best]) {
			best = i
		}
	}
	prediction.Selected = masks[best]
	*workspace, *output = candidate, prediction
	return nil
}
