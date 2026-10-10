package contractdecision

import (
	"math"
	"sync"
	"testing"
)

func choiceTraceFixture(t *testing.T, pooling string, count int) (*ChoiceModel, [][FeatureDim]float32) {
	t.Helper()
	var weights [ParameterCount]float32
	var contextWeights [ChoiceContextParameterCount]float32
	weights[0] = 1
	weights[sourceWeights], weights[sourceWeights+FeatureDim] = .5, 2
	weights[hiddenBias], weights[outputWeights+HiddenDim] = .25, 1
	contextWeights[0] = 1
	m, err := NewChoiceConditioned(weights, contextWeights, pooling)
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([][FeatureDim]float32, count)
	for i := range inputs {
		inputs[i][0] = 2
		if i%2 == 1 {
			inputs[i][0] = -2
		}
	}
	return m, inputs
}

func TestChoiceExplanationRecordsEachSourceInTheSamePass(t *testing.T) {
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		t.Run(pooling, func(t *testing.T) {
			m, inputs := choiceTraceFixture(t, pooling, MaxChoices)
			reader := &countedCases{n: MaxCases, fail: -1}
			var w, plain ChoiceWorkspace
			var p, q ChoicePrediction
			var e ChoiceExplanation
			if err := m.ExplainChoicesInto(inputs, reader, &w, &p, &e); err != nil || reader.seen != MaxCases {
				t.Fatal("read each caller case once", err, reader.seen)
			}
			if e.ChoiceCount != MaxChoices || e.CaseCount != MaxCases || e.CandidateCount != 0 || e.Pooling != pooling || e.CandidateScores != [MaxCandidates]float64{} {
				t.Fatal("trace bounds or unrequested candidates")
			}
			for i, input := range inputs {
				row, bias := e.Choices[i], input[0]
				pool := float32((127*float64(activate(bias)) + float64(activate(bias+1))) / 128)
				winner := -1
				if pooling == ExtremePooling {
					pool, winner = activate(bias+1), 127
					if bias < 0 {
						pool, winner = activate(bias), 0
					}
				}
				prefix := float32(.25) + .5*bias
				joint := prefix + 2*pool
				if row.ConditionedBias[0] != bias || row.Pool[0] != pool || row.Winner[0] != winner || row.SourcePrefix[0] != prefix || row.Joint[0] != joint || row.Hidden[0] != activate(joint) || row.OptionScores != p.Logits[i] {
					t.Fatal("actual choice intermediate", i, row, pool, winner)
				}
			}
			if err := m.PredictChoicesInto(inputs, &countedCases{n: MaxCases, fail: -1}, &plain, &q); err != nil || w != plain || p != q {
				t.Fatal("trace changed ordinary choice prediction", err)
			}
		})
	}
}

func TestChoiceCandidateExplanationMatchesPredictionAndClearsUnusedCells(t *testing.T) {
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		m, inputs, cases := choiceFixture(t, pooling)
		var w, plain ChoiceWorkspace
		var p, q Prediction
		var e ChoiceExplanation
		masks := []uint16{7, 0, 5, 2, 1}
		if err := m.ExplainInto(inputs, cases, masks, &w, &p, &e); err != nil {
			t.Fatal(err)
		}
		if err := m.PredictInto(inputs, cases, masks, &plain, &q); err != nil || p != q || w != plain || e.CandidateScores != w.scores || e.CandidateCount != len(masks) {
			t.Fatal("candidate explanation changed prediction", err)
		}
		checkChoiceTraceAgainstGlobalEquations(t, m, inputs, cases, e)
		for i, mask := range masks {
			var sum float64
			for c := range inputs {
				sum += float64(e.Choices[c].OptionScores[mask>>c&1])
			}
			if e.CandidateScores[i] != sum {
				t.Fatal("candidate score no longer links to actual option scores")
			}
		}
		if err := m.ExplainInto(inputs[:1], cases, []uint16{0}, &w, &p, &e); err != nil {
			t.Fatal(err)
		}
		for _, row := range e.Choices[1:] {
			if row != (ChoiceExplanationRow{}) {
				t.Fatal("stale choice cells")
			}
		}
		for _, score := range e.CandidateScores[1:] {
			if score != 0 {
				t.Fatal("stale candidate cells")
			}
		}
	}
}

func checkChoiceTraceAgainstGlobalEquations(t *testing.T, m *ChoiceModel, inputs [][FeatureDim]float32, cases rows, trace ChoiceExplanation) {
	t.Helper()
	for i, input := range inputs {
		weights := m.base.Weights()
		var bias [PoolDim]float32
		for h := range PoolDim {
			for j, x := range input {
				weights[caseBias+h] += x * m.context[h*FeatureDim+j]
			}
			bias[h] = weights[caseBias+h]
		}
		independent, err := NewForPooling(weights, m.Pooling())
		if err != nil {
			t.Fatal(err)
		}
		var w Workspace
		var p Prediction
		var expected Explanation
		if err := independent.ExplainInto([][FeatureDim]float32{input}, cases, []uint16{0, 1}, &w, &p, &expected); err != nil {
			t.Fatal(err)
		}
		row := trace.Choices[i]
		if row.ConditionedBias != bias || row.Pool != expected.Pool || row.Winner != expected.Winner || row.SourcePrefix != expected.SourcePrefix[0] || row.Joint != expected.Joint[0] || row.Hidden != expected.Hidden[0] || row.OptionScores != expected.OptionScores[0] {
			t.Fatal("choice trace differs from independent global forward equations", i)
		}
	}
}

func TestChoiceExplanationErrorsPreserveDestinations(t *testing.T) {
	m, inputs := choiceTraceFixture(t, ExtremePooling, 2)
	var w ChoiceWorkspace
	var p Prediction
	var q ChoicePrediction
	var e ChoiceExplanation
	if err := m.ExplainInto(inputs, rows{{1}}, []uint16{0, 1}, &w, &p, &e); err != nil {
		t.Fatal(err)
	}
	q.Count = 42
	wantW, wantP, wantQ, wantE := w, p, q, e
	bad := append([][FeatureDim]float32(nil), inputs...)
	bad[0][0] = float32(math.Inf(1))
	overflow := *m
	overflow.base.weights[0] = math.MaxFloat32
	for _, check := range []func() error{
		func() error { return m.ExplainInto(inputs, &countedCases{n: 128, fail: 127}, []uint16{0}, &w, &p, &e) },
		func() error { return m.ExplainInto(inputs, rows{{}}, []uint16{0, 0}, &w, &p, &e) },
		func() error { return m.ExplainInto(inputs, rows{{}}, []uint16{0}, &w, &p, nil) },
		func() error { return (*ChoiceModel)(nil).ExplainInto(inputs, rows{{}}, []uint16{0}, &w, &p, &e) },
		func() error { return m.ExplainInto(inputs, rows{{}}, []uint16{0}, nil, &p, &e) },
		func() error { return m.ExplainInto(inputs, rows{{}}, []uint16{0}, &w, nil, &e) },
		func() error { return m.ExplainChoicesInto(inputs, &countedCases{n: 128, fail: 127}, &w, &q, &e) },
		func() error { return m.ExplainChoicesInto(inputs, rows{{}}, &w, &q, nil) },
		func() error { return (*ChoiceModel)(nil).ExplainChoicesInto(inputs, rows{{}}, &w, &q, &e) },
		func() error { return m.ExplainChoicesInto(inputs, rows{{}}, nil, &q, &e) },
		func() error { return m.ExplainChoicesInto(inputs, rows{{}}, &w, nil, &e) },
		func() error { return m.ExplainChoicesInto(bad, rows{{}}, &w, &q, &e) },
		func() error { return overflow.ExplainChoicesInto(inputs, rows{{2}}, &w, &q, &e) },
	} {
		if err := check(); err == nil || w != wantW || p != wantP || q != wantQ || e != wantE {
			t.Fatal("non-atomic explanation error", err)
		}
	}
}

func TestChoiceExplanationConcurrentReaders(t *testing.T) {
	m, inputs := choiceTraceFixture(t, ExtremePooling, 2)
	beforeWeights, beforeContext := m.Weights()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			var w ChoiceWorkspace
			var p ChoicePrediction
			var e ChoiceExplanation
			for range 10 {
				if err := m.ExplainChoicesInto(inputs, &countedCases{n: 128, fail: -1}, &w, &p, &e); err != nil || e.Choices[0].Winner[0] != 127 || e.Choices[1].Winner[0] != 0 {
					t.Error("independent trace", err)
				}
			}
		})
	}
	wg.Wait()
	afterWeights, afterContext := m.Weights()
	if beforeWeights != afterWeights || beforeContext != afterContext {
		t.Fatal("explanation changed weights")
	}
}
