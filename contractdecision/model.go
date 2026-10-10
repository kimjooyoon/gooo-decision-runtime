// Package contractdecision ranks finite Gooo paths from source structure and
// every declared integer case. It learns local decisions without producing text.
package contractdecision

import (
	"errors"
	"math"
	"slices"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const (
	FeatureDim             = decision.ExecutionFlowFeatureDim
	CaseDim                = decision.DeclaredCaseFeatureDim
	PoolDim                = 8
	HiddenDim              = 24
	MaxCases               = 128
	MaxChoices             = 16
	MaxCandidates          = 64
	caseBias               = CaseDim * PoolDim
	sourceWeights          = caseBias + PoolDim
	jointDim               = FeatureDim + PoolDim
	hiddenBias             = sourceWeights + jointDim*HiddenDim
	outputWeights          = hiddenBias + HiddenDim
	outputBias             = outputWeights + HiddenDim*2
	ParameterCount         = outputBias + 2
	negativeSlope  float32 = 0.01
)

// CaseSource supplies the complete declared suite. Implementations must remain
// immutable for a call; pathplan.ContractInput owns and binds the source cases.
type CaseSource interface {
	CaseCount() int
	CaseFeatures(int) ([CaseDim]float32, error)
}

// Model owns 9,746 FP32 weights (38,984 bytes), independent of case count.
// The lossy eight-cell summary is a ranking hint, never a proof of completeness.
type Model struct {
	weights [ParameterCount]float32
	extreme bool
}

type Workspace struct {
	pool   [PoolDim]float32
	winner [PoolDim]int
	hidden [MaxChoices][HiddenDim]float32
	logits [MaxChoices][2]float32
	scores [MaxCandidates]float64
}

type Prediction struct {
	Count         int                    `json:"candidate_count"`
	Selected      uint16                 `json:"selected_mask"`
	Probabilities [MaxCandidates]float32 `json:"candidate_probabilities"`
}

type ChoicePrediction struct {
	Count  int                    `json:"choice_count"`
	Logits [MaxChoices][2]float32 `json:"choice_logits"`
}

func New(weights [ParameterCount]float32) (*Model, error) {
	if !finite(weights[:]) {
		return nil, errors.New("finite contract weights required")
	}
	return &Model{weights: weights}, nil
}

// NewForPooling selects an explicit computation contract. Legacy New and Fit
// retain arithmetic mean. Extreme pooling keeps the signed largest magnitude
// per learned case coordinate; ties of opposite sign choose the positive value.
func NewForPooling(weights [ParameterCount]float32, pooling string) (*Model, error) {
	if pooling != MeanPooling && pooling != ExtremePooling {
		return nil, errors.New("unsupported contract pooling")
	}
	m, err := New(weights)
	if err == nil {
		m.extreme = pooling == ExtremePooling
	}
	return m, err
}

func (m *Model) Pooling() string {
	if m != nil && m.extreme {
		return ExtremePooling
	}
	return MeanPooling
}

func (m *Model) ArtifactSchema() string {
	if m != nil && m.extreme {
		return PoolingSchema
	}
	return Schema
}

func (m *Model) Weights() [ParameterCount]float32 { return m.weights }

func finite(values []float32) bool {
	for _, v := range values {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return false
		}
	}
	return true
}

func activate(v float32) float32 {
	if v <= 0 {
		return v * negativeSlope
	}
	return v
}

func validate(inputs [][FeatureDim]float32, cases CaseSource, masks []uint16) error {
	if len(inputs) < 1 || len(inputs) > MaxChoices || cases == nil || cases.CaseCount() < 1 || cases.CaseCount() > MaxCases || len(masks) < 1 || len(masks) > MaxCandidates {
		return errors.New("contract ranking requires 1..16 choices, 1..128 cases and 1..64 candidates")
	}
	for _, input := range inputs {
		if !finite(input[:]) {
			return errors.New("nonfinite contract source input")
		}
	}
	for i, mask := range masks {
		if uint32(mask)>>len(inputs) != 0 || slices.Contains(masks[:i], mask) {
			return errors.New("duplicate or undeclared contract candidate")
		}
	}
	return nil
}

func (m *Model) caseHidden(row *[CaseDim]float32) [PoolDim]float32 {
	var hidden [PoolDim]float32
	for h := range PoolDim {
		sum := m.weights[caseBias+h]
		for j, x := range row {
			sum += x * m.weights[h*CaseDim+j]
		}
		hidden[h] = activate(sum)
	}
	return hidden
}

func (m *Model) forward(inputs [][FeatureDim]float32, cases CaseSource, masks []uint16, w *Workspace) error {
	return m.forwardTrace(inputs, cases, masks, w, nil)
}

func (m *Model) forwardTrace(inputs [][FeatureDim]float32, cases CaseSource, masks []uint16, w *Workspace, trace *Explanation) error {
	var sums [PoolDim]float64
	count := cases.CaseCount()
	if count < 1 || count > MaxCases {
		return errors.New("declared case count changed outside bounds")
	}
	for i := range count {
		row, err := cases.CaseFeatures(i)
		if err != nil {
			return err
		}
		if !finite(row[:]) {
			return errors.New("nonfinite declared case input")
		}
		hidden := m.caseHidden(&row)
		if !finite(hidden[:]) {
			return errors.New("nonfinite case activation")
		}
		for h, v := range hidden {
			if m.extreme {
				old, magnitude := w.pool[h], math.Abs(float64(v))
				if i == 0 || magnitude > math.Abs(float64(old)) || magnitude == math.Abs(float64(old)) && v > old {
					w.pool[h], w.winner[h] = v, i
				}
			} else {
				sums[h] += float64(v)
			}
		}
	}
	if !m.extreme {
		for h, sum := range sums {
			w.pool[h] = float32(sum / float64(count))
		}
	}
	if trace != nil {
		trace.ChoiceCount, trace.CandidateCount, trace.CaseCount = len(inputs), len(masks), count
		trace.Pooling, trace.Pool = m.Pooling(), w.pool
		for h := range PoolDim {
			trace.Winner[h] = -1
			if m.extreme {
				trace.Winner[h] = w.winner[h]
			}
		}
	}
	for choice, input := range inputs {
		for h := range HiddenDim {
			sum := m.weights[hiddenBias+h]
			start := sourceWeights + h*jointDim
			for j, x := range input {
				sum += x * m.weights[start+j]
			}
			if trace != nil {
				trace.SourcePrefix[choice][h] = sum
			}
			for j, x := range w.pool {
				sum += x * m.weights[start+FeatureDim+j]
			}
			if trace != nil {
				trace.Joint[choice][h] = sum
			}
			w.hidden[choice][h] = activate(sum)
		}
		if !finite(w.hidden[choice][:]) {
			return errors.New("nonfinite contract hidden activation")
		}
		for option := range 2 {
			sum := m.weights[outputBias+option]
			for h, x := range w.hidden[choice] {
				sum += x * m.weights[outputWeights+option*HiddenDim+h]
			}
			w.logits[choice][option] = sum
		}
		if !finite(w.logits[choice][:]) {
			return errors.New("nonfinite contract score")
		}
	}
	for i, mask := range masks {
		for choice := range inputs {
			w.scores[i] += float64(w.logits[choice][mask>>choice&1])
		}
	}
	if trace != nil {
		trace.Hidden, trace.OptionScores, trace.CandidateScores = w.hidden, w.logits, w.scores
	}
	return nil
}

// PredictInto reads all cases once, streaming through shared weights. Scores
// describe the supplied finite candidate pool, not correctness probabilities.
// Ties use the smaller mask. Errors leave both caller destinations unchanged.
func (m *Model) PredictInto(inputs [][FeatureDim]float32, cases CaseSource, masks []uint16, workspace *Workspace, output *Prediction) error {
	return m.predict(inputs, cases, masks, workspace, output, nil)
}

func (m *Model) predict(inputs [][FeatureDim]float32, cases CaseSource, masks []uint16, workspace *Workspace, output *Prediction, trace *Explanation) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("contract model and destinations required")
	}
	if err := validate(inputs, cases, masks); err != nil {
		return err
	}
	var candidate Workspace
	if err := m.forwardTrace(inputs, cases, masks, &candidate, trace); err != nil {
		return err
	}
	p, _ := distribution(candidate.scores[:len(masks)], allCandidates(len(masks)))
	result := Prediction{Count: len(masks)}
	best := 0
	for i := range masks {
		result.Probabilities[i] = float32(p[i])
		if candidate.scores[i] > candidate.scores[best] || (candidate.scores[i] == candidate.scores[best] && masks[i] < masks[best]) {
			best = i
		}
	}
	result.Selected = masks[best]
	*workspace, *output = candidate, result
	return nil
}

// PredictChoicesInto exposes the same additive scores for a bounded frontier.
// Concurrent calls share immutable weights and use separate caller workspaces.
func (m *Model) PredictChoicesInto(inputs [][FeatureDim]float32, cases CaseSource, workspace *Workspace, output *ChoicePrediction) error {
	if m == nil || workspace == nil || output == nil {
		return errors.New("contract model and destinations required")
	}
	if err := validate(inputs, cases, []uint16{0}); err != nil {
		return err
	}
	var candidate Workspace
	if err := m.forward(inputs, cases, nil, &candidate); err != nil {
		return err
	}
	*workspace, *output = candidate, ChoicePrediction{Count: len(inputs), Logits: candidate.logits}
	return nil
}

func allCandidates(n int) uint64 { return ^uint64(0) >> (64 - n) }

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
