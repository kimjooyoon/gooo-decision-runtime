package contractdecision

import (
	"context"
	"errors"
	"math"
)

// Acceptable names complete compiled candidates by index in Masks. The owning
// compiler supplies labels from actual case/condition evaluation. Fit receives
// training samples only; evaluation contracts must stay outside this API.
type Sample struct {
	Inputs     [][FeatureDim]float32
	Cases      CaseSource
	Masks      []uint16
	Acceptable uint64
}

type FitOptions struct {
	Epochs       int     `json:"epochs"`
	LearningRate float64 `json:"learning_rate"`
	L2           float64 `json:"l2"`
	Seed         uint64  `json:"seed"`
}

type Epoch struct {
	Number int     `json:"epoch"`
	Loss   float64 `json:"pre_update_mean_loss"`
}

func initial(seed uint64) [ParameterCount]float32 {
	var w [ParameterCount]float32
	state := seed
	if state == 0 {
		state = 1
	}
	for i := range w {
		if i >= caseBias && i < sourceWeights || i >= hiddenBias && i < outputWeights || i >= outputBias {
			continue
		}
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		w[i] = (float32(state>>40)/float32(1<<24) - 0.5) * 0.5
	}
	return w
}

func validOptions(o FitOptions) bool {
	return o.Epochs > 0 && o.Epochs <= 10000 && o.LearningRate > 0 && !math.IsInf(o.LearningRate, 0) && !math.IsNaN(o.LearningRate) && o.L2 >= 0 && !math.IsInf(o.L2, 0) && !math.IsNaN(o.L2)
}

// Fit trains the case encoder and source/pooled-case network together on CPU.
// Full-batch updates are deterministic for immutable inputs on the same runtime.
// One 16KiB per-sample scratch snapshot binds forward/backward to identical case
// rows. Training never retains a whole-dataset case tensor or calls a compiler.
func Fit(ctx context.Context, samples []Sample, options FitOptions) (*Model, []Epoch, error) {
	if ctx == nil || len(samples) < 1 || len(samples) > 4096 || !validOptions(options) {
		return nil, nil, errors.New("bounded contract training inputs required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	for _, s := range samples {
		if err := validate(s.Inputs, s.Cases, s.Masks); err != nil {
			return nil, nil, err
		}
		if s.Acceptable == 0 || s.Acceptable & ^allCandidates(len(s.Masks)) != 0 {
			return nil, nil, errors.New("acceptable set must name complete contract candidates")
		}
	}
	m, _ := New(initial(options.Seed))
	history := make([]Epoch, 0, options.Epochs)
	var frozen frozenCases
	for epoch := range options.Epochs {
		var gradient [ParameterCount]float64
		loss := 0.0
		for _, s := range samples {
			if err := ctx.Err(); err != nil {
				return nil, history, err
			}
			if err := frozen.capture(s.Cases); err != nil {
				return nil, history, err
			}
			var w Workspace
			if err := m.forward(s.Inputs, &frozen, s.Masks, &w); err != nil {
				return nil, history, err
			}
			all, a := distribution(w.scores[:len(s.Masks)], allCandidates(len(s.Masks)))
			good, b := distribution(w.scores[:len(s.Masks)], s.Acceptable)
			loss += a - b
			var residual [MaxChoices][2]float64
			for i, mask := range s.Masks {
				for c := range s.Inputs {
					residual[c][mask>>c&1] += all[i] - good[i]
				}
			}
			m.backward(s.Inputs, &frozen, &w, &residual, &gradient)
		}
		if err := ctx.Err(); err != nil {
			return nil, history, err
		}
		history = append(history, Epoch{epoch + 1, loss / float64(len(samples))})
		for i, v := range m.weights {
			u := float64(v) - options.LearningRate*(gradient[i]/float64(len(samples))+options.L2*float64(v))
			if math.IsNaN(u) || math.IsInf(u, 0) || math.Abs(u) > math.MaxFloat32 {
				return nil, history, errors.New("nonfinite contract training update")
			}
			m.weights[i] = float32(u)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, history, err
	}
	return m, history, nil
}

type frozenCases struct {
	rows  [MaxCases][CaseDim]float32
	count int
}

func (f *frozenCases) CaseCount() int { return f.count }
func (f *frozenCases) CaseFeatures(i int) ([CaseDim]float32, error) {
	return f.rows[i], nil
}
func (f *frozenCases) capture(cases CaseSource) error {
	f.count = cases.CaseCount()
	if f.count < 1 || f.count > MaxCases {
		return errors.New("declared case count changed outside bounds")
	}
	for i := range f.count {
		row, err := cases.CaseFeatures(i)
		if err != nil {
			return err
		}
		f.rows[i] = row
		if !finite(f.rows[i][:]) {
			return errors.New("nonfinite training case")
		}
	}
	return nil
}

func (m *Model) backward(inputs [][FeatureDim]float32, cases *frozenCases, w *Workspace, residual *[MaxChoices][2]float64, g *[ParameterCount]float64) {
	var poolGradient [PoolDim]float64
	for c, input := range inputs {
		for option := range 2 {
			delta := residual[c][option]
			g[outputBias+option] += delta
			for h, x := range w.hidden[c] {
				g[outputWeights+option*HiddenDim+h] += delta * float64(x)
			}
		}
		for h, x := range w.hidden[c] {
			delta := 0.0
			for option := range 2 {
				delta += residual[c][option] * float64(m.weights[outputWeights+option*HiddenDim+h])
			}
			if x <= 0 {
				delta *= float64(negativeSlope)
			}
			g[hiddenBias+h] += delta
			start := sourceWeights + h*jointDim
			for j, v := range input {
				g[start+j] += delta * float64(v)
			}
			for j, v := range w.pool {
				g[start+FeatureDim+j] += delta * float64(v)
				poolGradient[j] += delta * float64(m.weights[start+FeatureDim+j])
			}
		}
	}
	for _, row := range cases.rows[:cases.count] {
		hidden := m.caseHidden(&row)
		for h, x := range hidden {
			delta := poolGradient[h] / float64(cases.count)
			if x <= 0 {
				delta *= float64(negativeSlope)
			}
			g[caseBias+h] += delta
			for j, v := range row {
				g[h*CaseDim+j] += delta * float64(v)
			}
		}
	}
}
