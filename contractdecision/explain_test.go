package contractdecision

import (
	"sync"
	"testing"
)

func traceFixture(t *testing.T, pooling string) (*Model, [][FeatureDim]float32) {
	t.Helper()
	var weights [ParameterCount]float32
	weights[0] = 1
	weights[sourceWeights], weights[sourceWeights+FeatureDim] = 0.5, 2
	weights[hiddenBias], weights[outputWeights+HiddenDim] = 0.25, 1
	m, err := NewForPooling(weights, pooling)
	if err != nil {
		t.Fatal(err)
	}
	input := make([][FeatureDim]float32, 1)
	input[0][0] = 2
	return m, input
}

func TestExplanationUsesActualSinglePass(t *testing.T) {
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		t.Run(pooling, func(t *testing.T) {
			m, input := traceFixture(t, pooling)
			reader := &countedCases{n: 128, fail: -1}
			var w, plain Workspace
			var p, q Prediction
			var e Explanation
			if err := m.ExplainInto(input, reader, []uint16{0, 1}, &w, &p, &e); err != nil || reader.seen != 128 {
				t.Fatal("exactly one read of each case required", err, reader.seen)
			}
			wantPool, winner := float32(1.0/128), -1
			if pooling == ExtremePooling {
				wantPool, winner = 1, 127
			}
			if e.SourcePrefix[0][0] != 1.25 || e.Joint[0][0] != 1.25+2*wantPool || e.Pool[0] != wantPool || e.Winner[0] != winner || e.Pooling != pooling || e.CaseCount != 128 || e.ChoiceCount != 1 || e.CandidateCount != 2 {
				t.Fatal("incorrect actual intermediate", e)
			}
			if err := m.PredictInto(input, reader, []uint16{0, 1}, &plain, &q); err != nil || w != plain || p != q || e.Hidden != w.hidden || e.OptionScores != w.logits || e.CandidateScores != w.scores {
				t.Fatal("trace changed normal computation", err)
			}
			if n := testing.AllocsPerRun(20, func() {
				if err := m.ExplainInto(input, reader, []uint16{0, 1}, &w, &p, &e); err != nil {
					panic(err)
				}
			}); n != 0 {
				t.Fatal("trace kernel heap allocation", n)
			}
		})
	}
}

func TestExplanationErrorsPreserveAllDestinations(t *testing.T) {
	m, input := traceFixture(t, ExtremePooling)
	var w Workspace
	var p Prediction
	var e Explanation
	if err := m.ExplainInto(input, rows{{1}}, []uint16{0, 1}, &w, &p, &e); err != nil {
		t.Fatal(err)
	}
	wantW, wantP, wantE := w, p, e
	for _, check := range []func() error{
		func() error {
			return m.ExplainInto(input, &countedCases{n: 128, fail: 127}, []uint16{0, 1}, &w, &p, &e)
		},
		func() error { return m.ExplainInto(input, rows{{}}, []uint16{0, 0}, &w, &p, &e) },
		func() error { return m.ExplainInto(input, rows{{}}, []uint16{0, 1}, &w, &p, nil) },
		func() error { return (*Model)(nil).ExplainInto(input, rows{{}}, []uint16{0, 1}, &w, &p, &e) },
		func() error { return m.ExplainInto(input, rows{{}}, []uint16{0, 1}, nil, &p, &e) },
		func() error { return m.ExplainInto(input, rows{{}}, []uint16{0, 1}, &w, nil, &e) },
	} {
		if err := check(); err == nil || w != wantW || p != wantP || e != wantE {
			t.Fatal("non-atomic trace error", err)
		}
	}
}

func TestExplanationConcurrentReaders(t *testing.T) {
	m, input := traceFixture(t, MeanPooling)
	before := m.Weights()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			var w Workspace
			var p Prediction
			var e Explanation
			for range 10 {
				if err := m.ExplainInto(input, rows{{1}}, []uint16{0, 1}, &w, &p, &e); err != nil || e.Joint[0][0] != 3.25 || p.Selected != 1 {
					t.Error("shared immutable trace", err)
				}
			}
		})
	}
	wg.Wait()
	if m.Weights() != before {
		t.Fatal("trace mutated model")
	}
}
