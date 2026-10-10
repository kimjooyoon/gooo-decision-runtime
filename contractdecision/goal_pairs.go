package contractdecision

import (
	"context"
	"errors"
	"math"
	"slices"
)

// GoalPair identifies two training samples with identical source arrays and
// candidate order, but disjoint compiler-validated acceptable candidate sets.
// Indices refer only to the supplied training samples, never evaluation rows.
type GoalPair struct {
	First  int `json:"first"`
	Second int `json:"second"`
}

type GoalPairOptions struct {
	Weight float64 `json:"weight"`
	Margin float64 `json:"margin"`
}

// FitWithGoalPairs adds a paired goal-separation loss to the usual mean candidate
// loss. It preserves the inference architecture, artifact format and parameter
// count. Epoch.Loss reports mean candidate loss plus Weight times mean pair loss
// before each update; the L2 penalty is applied separately as in Fit.
// Each paired sample's cases are read twice per epoch into bounded scratch arrays.
// Inputs and case sources must remain immutable throughout training.
func FitWithGoalPairs(ctx context.Context, samples []Sample, options FitOptions, pooling string, pairs []GoalPair, pairOptions GoalPairOptions) (*Model, []Epoch, error) {
	if len(pairs) == 0 || pairOptions.Weight <= 0 {
		return nil, nil, errors.New("goal-pair training requires pairs and positive weight")
	}
	return fit(ctx, samples, options, pooling, pairs, pairOptions)
}

func validateGoalPairs(samples []Sample, pairs []GoalPair, options GoalPairOptions) error {
	if len(pairs) == 0 && options == (GoalPairOptions{}) {
		return nil
	}
	if len(pairs) < 1 || len(pairs) > len(samples)/2 || options.Weight <= 0 || options.Margin < 0 || math.IsNaN(options.Weight) || math.IsInf(options.Weight, 0) || math.IsNaN(options.Margin) || math.IsInf(options.Margin, 0) {
		return errors.New("bounded pairs and finite positive weight/nonnegative margin required")
	}
	var used [4096]bool
	for _, pair := range pairs {
		if pair.First < 0 || pair.Second < 0 || pair.First >= len(samples) || pair.Second >= len(samples) || pair.First == pair.Second || used[pair.First] || used[pair.Second] {
			return errors.New("goal pairs must use distinct training samples once")
		}
		first, second := samples[pair.First], samples[pair.Second]
		if !slices.Equal(first.Inputs, second.Inputs) || !slices.Equal(first.Masks, second.Masks) || first.Acceptable&second.Acceptable != 0 {
			return errors.New("goal pairs require equal sources/candidates and disjoint acceptable sets")
		}
		used[pair.First], used[pair.Second] = true, true
	}
	return nil
}

// softplusSlope returns softplus(x) and its derivative without exp overflow.
func softplusSlope(x float64) (float64, float64) {
	if x >= 0 {
		e := math.Exp(-x)
		return x + math.Log1p(e), 1 / (1 + e)
	}
	e := math.Exp(x)
	return math.Log1p(e), e / (1 + e)
}

// Each goal must prefer its own acceptable set over the other goal's set.
// For singleton acceptable sets, the gap cancels goal-independent candidate
// biases. For larger sets it compares their log-sum-exp scores. The ordinary
// per-sample loss remains necessary: a large paired gap alone need not put both
// individual margins above zero or select a complete candidate.
func goalPairResidual(first, second *Workspace, masks []uint16, choices int, firstGood, secondGood uint64, margin, scale float64) (float64, [2][MaxChoices][2]float64) {
	aOwn, a := distribution(first.scores[:len(masks)], firstGood)
	aOther, b := distribution(first.scores[:len(masks)], secondGood)
	bOwn, c := distribution(second.scores[:len(masks)], secondGood)
	bOther, d := distribution(second.scores[:len(masks)], firstGood)
	loss, slope := softplusSlope(margin - ((a - b) + (c - d)))
	var residual [2][MaxChoices][2]float64
	for i, mask := range masks {
		for choice := range choices {
			option := mask >> choice & 1
			residual[0][choice][option] -= scale * slope * (aOwn[i] - aOther[i])
			residual[1][choice][option] -= scale * slope * (bOwn[i] - bOther[i])
		}
	}
	return scale * loss, residual
}

func (m *Model) goalPairGradient(ctx context.Context, samples []Sample, pairs []GoalPair, options GoalPairOptions, gradient *[ParameterCount]float64) (float64, error) {
	// fit divides loss and gradient by sample count once, after both terms.
	scale := options.Weight * float64(len(samples)) / float64(len(pairs))
	loss := 0.0
	var frozen [2]frozenCases
	for _, pair := range pairs {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		rows := [2]Sample{samples[pair.First], samples[pair.Second]}
		var work [2]Workspace
		for i, row := range rows {
			if err := frozen[i].capture(row.Cases); err != nil {
				return 0, err
			}
			if err := m.forward(row.Inputs, &frozen[i], row.Masks, &work[i]); err != nil {
				return 0, err
			}
		}
		value, residual := goalPairResidual(&work[0], &work[1], rows[0].Masks, len(rows[0].Inputs), rows[0].Acceptable, rows[1].Acceptable, options.Margin, scale)
		loss += value
		for i, row := range rows {
			m.backward(row.Inputs, &frozen[i], &work[i], &residual[i], gradient)
		}
	}
	if math.IsNaN(loss) || math.IsInf(loss, 0) {
		return 0, errors.New("nonfinite goal-pair loss")
	}
	return loss, nil
}
