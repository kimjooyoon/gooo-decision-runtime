package contractdecision

import (
	"context"
	"math"
	"sync"
	"testing"
	"unsafe"
)

func choiceFixture(t *testing.T, pooling string) (*ChoiceModel, [][FeatureDim]float32, rows) {
	t.Helper()
	m, err := NewChoiceConditioned(initial(37), initialChoiceContext(29), pooling)
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([][FeatureDim]float32, 3)
	cases := make(rows, 5)
	seed := uint64(123)
	next := func() float32 {
		seed ^= seed << 13
		seed ^= seed >> 7
		seed ^= seed << 17
		return (float32(seed>>40)/float32(1<<24) - .5) * .5
	}
	for i := range inputs {
		for j := range inputs[i] {
			inputs[i][j] = next()
		}
	}
	for i := range cases {
		for j := range cases[i] {
			cases[i][j] = next()
		}
	}
	return m, inputs, cases
}

// A separate scalar float64 equation includes both connections from the source:
// one before each case activation, the other after choice-specific pooling.
func referenceChoice(m *ChoiceModel, source [FeatureDim]float32, cases rows) [2]float64 {
	var pool [PoolDim]float64
	for i, row := range cases {
		for h := range PoolDim {
			v := float64(m.base.weights[caseBias+h])
			for j, x := range source {
				v += float64(x) * float64(m.context[h*FeatureDim+j])
			}
			for j, x := range row {
				v += float64(x) * float64(m.base.weights[h*CaseDim+j])
			}
			if v <= 0 {
				v *= float64(negativeSlope)
			}
			if m.base.extreme {
				if i == 0 || math.Abs(v) > math.Abs(pool[h]) || math.Abs(v) == math.Abs(pool[h]) && v > pool[h] {
					pool[h] = v
				}
			} else {
				pool[h] += v / float64(len(cases))
			}
		}
	}
	var result [2]float64
	for o := range 2 {
		result[o] = float64(m.base.weights[outputBias+o])
	}
	for h := range HiddenDim {
		v := float64(m.base.weights[hiddenBias+h])
		start := sourceWeights + h*jointDim
		for j, x := range source {
			v += float64(x) * float64(m.base.weights[start+j])
		}
		for j, x := range pool {
			v += x * float64(m.base.weights[start+FeatureDim+j])
		}
		if v <= 0 {
			v *= float64(negativeSlope)
		}
		for o := range 2 {
			result[o] += v * float64(m.base.weights[outputWeights+o*HiddenDim+h])
		}
	}
	return result
}

func TestChoiceConditionedEquationAndZeroContext(t *testing.T) {
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		m, inputs, cases := choiceFixture(t, pooling)
		var w ChoiceWorkspace
		var got ChoicePrediction
		if err := m.PredictChoicesInto(inputs, cases, &w, &got); err != nil {
			t.Fatal(err)
		}
		for c, input := range inputs {
			want := referenceChoice(m, input, cases)
			for o := range 2 {
				if math.Abs(float64(got.Logits[c][o])-want[o]) > 2e-6 {
					t.Fatal("independent equation", pooling, c, o, got.Logits[c], want)
				}
			}
		}
		clear(m.context[:])
		var old Workspace
		var before ChoicePrediction
		if err := m.base.PredictChoicesInto(inputs, cases, &old, &before); err != nil {
			t.Fatal(err)
		}
		if err := m.PredictChoicesInto(inputs, cases, &w, &got); err != nil || got != before {
			t.Fatal("zero context changes legacy logits", err)
		}
		masks := []uint16{7, 0, 3, 2}
		var a, b Prediction
		if err := m.base.PredictInto(inputs, cases, masks, &old, &a); err != nil {
			t.Fatal(err)
		}
		if err := m.PredictInto(inputs, cases, masks, &w, &b); err != nil || a != b {
			t.Fatal("candidate scores or tie policy differ", err, a, b)
		}
	}
}

func TestChoiceConditionedAllCasesAtomicityAndConcurrency(t *testing.T) {
	m, inputs, _ := choiceFixture(t, ExtremePooling)
	reader := &countedCases{n: 128, fail: -1}
	var w ChoiceWorkspace
	var p Prediction
	if err := m.PredictInto(inputs, reader, []uint16{0, 1, 7}, &w, &p); err != nil || reader.seen != 128 {
		t.Fatal("must read each case once", reader.seen, err)
	}
	wantW, wantP := w, p
	for _, run := range []func() error{
		func() error { return m.PredictInto(inputs, &countedCases{n: 128, fail: 127}, []uint16{0, 1}, &w, &p) },
		func() error { return m.PredictInto(inputs, rows{{}}, []uint16{0, 0}, &w, &p) },
		func() error { return (*ChoiceModel)(nil).PredictInto(inputs, rows{{}}, []uint16{0}, &w, &p) },
	} {
		if err := run(); err == nil || w != wantW || p != wantP {
			t.Fatal("failed prediction changed destinations", err)
		}
	}
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			var local ChoiceWorkspace
			var result Prediction
			if err := m.PredictInto(inputs, &countedCases{n: 128, fail: -1}, []uint16{0, 1, 7}, &local, &result); err != nil || result != wantP {
				t.Error("shared model", err)
			}
		})
	}
	group.Wait()
	t.Logf("weights=%d bytes model=%d workspace=%d", ChoiceParameterCount*4, unsafe.Sizeof(*m), unsafe.Sizeof(w))
}

func TestChoiceConditionedGradient(t *testing.T) {
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		m, inputs, cases := choiceFixture(t, pooling)
		masks := []uint16{0, 2, 5, 7}
		acceptable := uint64(1<<1 | 1<<3)
		loss := func() float64 {
			var w ChoiceWorkspace
			if err := w.cases.capture(cases); err != nil {
				t.Fatal(err)
			}
			if err := m.forward(inputs, masks, &w); err != nil {
				t.Fatal(err)
			}
			_, a := distribution(w.scores[:4], 15)
			_, b := distribution(w.scores[:4], acceptable)
			return a - b
		}
		var w ChoiceWorkspace
		_ = w.cases.capture(cases)
		if err := m.forward(inputs, masks, &w); err != nil {
			t.Fatal(err)
		}
		all, _ := distribution(w.scores[:4], 15)
		good, _ := distribution(w.scores[:4], acceptable)
		var residual [MaxChoices][2]float64
		for i, mask := range masks {
			for c := range inputs {
				residual[c][mask>>c&1] += all[i] - good[i]
			}
		}
		var gradient [ChoiceParameterCount]float64
		if err := m.backward(inputs, &w, &residual, &gradient); err != nil {
			t.Fatal(err)
		}
		indices := []int{0, 31, 128, 255, caseBias, caseBias + 7, sourceWeights, sourceWeights + 383, sourceWeights + 384, hiddenBias, outputWeights, outputBias, ParameterCount - 1}
		for h := range PoolDim {
			indices = append(indices, ParameterCount+h*FeatureDim, ParameterCount+h*FeatureDim+383)
		}
		nonzero := 0
		for _, i := range indices {
			weight := &m.base.weights[0]
			if i < ParameterCount {
				weight = &m.base.weights[i]
			} else {
				weight = &m.context[i-ParameterCount]
			}
			old := *weight
			*weight = old + .001
			upper := *weight
			a := loss()
			*weight = old - .001
			lower := *weight
			b := loss()
			*weight = old
			finiteDifference := (a - b) / float64(upper-lower)
			if math.Abs(finiteDifference-gradient[i]) > 8e-4 {
				t.Fatalf("%s parameter%d analytic%g numerical%g", pooling, i, gradient[i], finiteDifference)
			}
			if i >= ParameterCount && math.Abs(gradient[i]) > 1e-6 {
				nonzero++
			}
		}
		if nonzero == 0 {
			t.Fatal("context gradients never exercised")
		}
	}
}

func TestChoiceConditionedFitChangesBothWeightSets(t *testing.T) {
	_, inputs, cases := choiceFixture(t, MeanPooling)
	samples := []Sample{{Inputs: inputs, Cases: cases, Masks: []uint16{0, 2, 5, 7}, Acceptable: 2}}
	opts := FitOptions{Epochs: 3, LearningRate: .3, L2: .0001, Seed: 7}
	m, history, err := FitChoiceConditioned(context.Background(), samples, opts, MeanPooling)
	if err != nil || len(history) != 3 || m.base.weights == initial(7) || m.context == initialChoiceContext(7) {
		t.Fatal("joint update", history, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if m, _, err := FitChoiceConditioned(ctx, samples, opts, MeanPooling); err == nil || m != nil {
		t.Fatal("cancelled fit accepted")
	}
}

func TestChoiceConditionedTailAndAllChoiceBounds(t *testing.T) {
	var weights [ParameterCount]float32
	var contextWeights [ChoiceContextParameterCount]float32
	weights[0] = 1
	weights[sourceWeights+FeatureDim] = 1
	weights[outputWeights+HiddenDim] = 1
	contextWeights[0] = .5
	model, err := NewChoiceConditioned(weights, contextWeights, ExtremePooling)
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([][FeatureDim]float32, 16)
	inputs[15][0] = .5
	cases := make(rows, 128)
	var w ChoiceWorkspace
	var before, after ChoicePrediction
	if err := model.PredictChoicesInto(inputs, cases, &w, &before); err != nil {
		t.Fatal(err)
	}
	cases[127][0] = 1
	if err := model.PredictChoicesInto(inputs, cases, &w, &after); err != nil || after.Count != 16 || after.Logits[15][1] != 1.25 || before.Logits[15][1] != .25 || after.Logits[0][1] != 1 {
		t.Fatal("tail case or final source choice omitted", before, after, err)
	}
	wantW, wantP := w, after
	if err := model.PredictChoicesInto(append(inputs, [FeatureDim]float32{}), cases, &w, &after); err == nil || w != wantW || after != wantP {
		t.Fatal("17 choices accepted or changed outputs", err)
	}
	inputs[0][0] = math.MaxFloat32
	model.context[0] = math.MaxFloat32
	if err := model.PredictChoicesInto(inputs, cases, &w, &after); err == nil || w != wantW || after != wantP {
		t.Fatal("overflow changed outputs", err)
	}
}
