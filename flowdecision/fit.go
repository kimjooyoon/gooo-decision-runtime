package flowdecision

import (
	"context"
	"errors"
	"math"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// Acceptable is a bit set of indexes in Masks, not independent option labels.
// A caller derives it by compiling complete candidates and checking source cases.
// Fit receives training samples only; evaluation data must stay outside this API.
type Sample struct {
	Inputs     [][FeatureDim]float32 `json:"inputs"`
	Masks      []uint16              `json:"candidate_masks"`
	Acceptable uint64                `json:"acceptable_candidate_bits"`
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
		if i >= FeatureDim*HiddenDim && i < w2Start || i >= b2Start {
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
	return o.Epochs > 0 && o.Epochs <= 10000 && o.LearningRate > 0 && !math.IsInf(o.LearningRate, 0) &&
		!math.IsNaN(o.LearningRate) && o.L2 >= 0 && !math.IsInf(o.L2, 0) && !math.IsNaN(o.L2)
}

// Fit makes deterministic full-batch CPU updates to a shared 384→24→2 network.
// The loss is minus log probability mass of the complete acceptable mask set.
// Its additive score cannot represent every joint distribution; the compiler
// must still check the proposed complete candidate. Cancellation returns no model.
func Fit(ctx context.Context, samples []Sample, options FitOptions) (*Model, []Epoch, error) {
	return FitForFeatures(ctx, samples, options, decision.ExecutionFlowFeatureVersion)
}

// FitForFeatures requires samples encoded with the named ABI. Raw arrays do not
// identify their own representation. It retains the same bounded CPU training.
func FitForFeatures(ctx context.Context, samples []Sample, options FitOptions, version string) (*Model, []Epoch, error) {
	return FitForActivation(ctx, samples, options, version, ReLUActivation)
}

// FitForActivation preserves the bounded full-batch training schedule while
// explicitly selecting the activation. The leaky slope is fixed at FP32 .01;
// its derivative at zero uses that slope. Existing training defaults to ReLU.
func FitForActivation(ctx context.Context, samples []Sample, options FitOptions, version, activation string) (*Model, []Epoch, error) {
	if !supportedFeatures(version) {
		return nil, nil, errors.New("unsupported flow training feature version")
	}
	if !supportedActivation(activation) {
		return nil, nil, errors.New("unsupported flow training activation")
	}
	if ctx == nil || len(samples) == 0 || len(samples) > 4096 || !validOptions(options) {
		return nil, nil, errors.New("bounded flow training samples and options required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	for _, sample := range samples {
		if err := validate(sample.Inputs, sample.Masks); err != nil {
			return nil, nil, err
		}
		if sample.Acceptable == 0 || sample.Acceptable & ^allCandidates(len(sample.Masks)) != 0 {
			return nil, nil, errors.New("acceptable set must name supplied complete candidates")
		}
	}
	m, _ := NewForActivation(initial(options.Seed), version, activation)
	history := make([]Epoch, 0, options.Epochs)
	for epoch := range options.Epochs {
		var gradient [ParameterCount]float64
		loss := 0.0
		for _, sample := range samples {
			if err := ctx.Err(); err != nil {
				return nil, history, err
			}
			var w Workspace
			if err := m.forward(sample.Inputs, sample.Masks, &w); err != nil {
				return nil, history, err
			}
			all, logAll := distribution(w.scores[:len(sample.Masks)], allCandidates(len(sample.Masks)))
			good, logGood := distribution(w.scores[:len(sample.Masks)], sample.Acceptable)
			loss += logAll - logGood
			var residual [MaxChoices][2]float64
			for i, mask := range sample.Masks {
				for choice := range sample.Inputs {
					residual[choice][mask>>choice&1] += all[i] - good[i]
				}
			}
			m.backward(sample.Inputs, &w, &residual, &gradient)
		}
		if err := ctx.Err(); err != nil {
			return nil, history, err
		}
		history = append(history, Epoch{epoch + 1, loss / float64(len(samples))})
		for i, weight := range m.weights {
			updated := float64(weight) - options.LearningRate*(gradient[i]/float64(len(samples))+options.L2*float64(weight))
			if math.IsNaN(updated) || math.IsInf(updated, 0) || math.Abs(updated) > math.MaxFloat32 {
				return nil, history, errors.New("nonfinite execution training update")
			}
			m.weights[i] = float32(updated)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, history, err
	}
	return m, history, nil
}

func (m *Model) backward(inputs [][FeatureDim]float32, w *Workspace, residual *[MaxChoices][2]float64, gradient *[ParameterCount]float64) {
	for choice, input := range inputs {
		for option := range 2 {
			delta := residual[choice][option]
			gradient[b2Start+option] += delta
			for h, x := range w.hidden[choice] {
				gradient[w2Start+option*HiddenDim+h] += delta * float64(x)
			}
		}
		for h, x := range w.hidden[choice] {
			if x <= 0 && m.Activation() != LeakyReLUActivation {
				continue
			}
			delta := 0.0
			for option := range 2 {
				delta += residual[choice][option] * float64(m.weights[w2Start+option*HiddenDim+h])
			}
			if x <= 0 {
				delta *= float64(negativeSlope)
			}
			gradient[FeatureDim*HiddenDim+h] += delta
			for j, v := range input {
				gradient[h*FeatureDim+j] += delta * float64(v)
			}
		}
	}
}
