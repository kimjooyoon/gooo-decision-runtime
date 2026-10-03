package orderjudge

import (
	"context"
	"encoding/binary"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/orderfacts"
)

func signature() orderfacts.Signature {
	var s orderfacts.Signature
	s[0], s[1] = 1, 2
	for i, opcode := range []byte{1, 3} {
		s[8+20*i], s[9+20*i], s[10+20*i] = opcode, 1, 3
		binary.LittleEndian.PutUint64(s[20+20*i:28+20*i], uint64(i+1))
	}
	return s
}

func example() Sample {
	var s Sample
	_ = IntentFeatures("Add one, then multiply by two. 1을 더한 뒤 2를 곱한다.", &s.Intent)
	descriptor := signature()
	for mask := range 8 {
		x := descriptor
		if mask&1 != 0 {
			copy(x[8:28], descriptor[28:48])
			copy(x[28:48], descriptor[8:28])
		}
		_ = SourceFeatures(x, &s.Candidates[mask])
	}
	s.Acceptable = 0x55
	return s
}

func TestFeatureBoundsAndAtomicity(t *testing.T) {
	for _, text := range []string{"", strings.Repeat("a", 513), string([]byte{0xff})} {
		out := [IntentDim]float32{9}
		if IntentFeatures(text, &out) == nil || out != ([IntentDim]float32{9}) {
			t.Fatal("invalid intent partially wrote features")
		}
	}
	for _, n := range []int64{17, -17, math.MinInt64, math.MaxInt64} {
		x := signature()
		binary.LittleEndian.PutUint64(x[20:28], uint64(n))
		out := [SourceDim]float32{9}
		if SourceFeatures(x, &out) == nil || out != ([SourceDim]float32{9}) {
			t.Fatal("out-of-range literal accepted")
		}
	}
	for _, offset := range []int{0, 2, 11, 9} {
		x := signature()
		x[offset] = 255
		if SourceFeatures(x, new([SourceDim]float32)) == nil {
			t.Fatal("invalid descriptor accepted")
		}
	}
}

func TestFitRoundtripAndImmutableParallelPrediction(t *testing.T) {
	s := example()
	m, history, err := Fit(context.Background(), []Sample{s}, FitOptions{40, 2, 0.0001})
	if err != nil || len(history) != 40 || history[39].Loss >= history[0].Loss {
		t.Fatal("fit did not reduce training loss", err)
	}
	metadata, raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(metadata, raw)
	if err != nil {
		t.Fatal(err)
	}
	var expected Prediction
	var work Workspace
	if err := m.Predict(&s.Intent, &s.Candidates, &work, &expected); err != nil {
		t.Fatal(err)
	}
	if s.Acceptable&(1<<expected.Ranking[0]) == 0 {
		t.Fatal("fitted example did not select acceptable mask")
	}
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			var work Workspace
			var p Prediction
			if err := loaded.Predict(&s.Intent, &s.Candidates, &work, &p); err != nil || p != expected {
				t.Error("parallel reload prediction differs", err)
			}
		})
	}
	group.Wait()
	raw[0] ^= 1
	if _, err := Load(metadata, raw); err == nil {
		t.Fatal("changed tensor accepted")
	}
	if _, err := Load(append(metadata, []byte("{}")...), raw); err == nil {
		t.Fatal("trailing metadata accepted")
	}
}

func TestCancellationAndNonfiniteInputs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if m, _, err := Fit(ctx, []Sample{example()}, FitOptions{1, 2, 0}); err == nil || m != nil {
		t.Fatal("cancelled fit returned model")
	}
	var w [ParameterCount]float32
	w[0] = float32(math.NaN())
	if _, err := New(w); err == nil {
		t.Fatal("NaN weights accepted")
	}
	m, _ := New([ParameterCount]float32{})
	s := example()
	s.Intent[0] = float32(math.NaN())
	output := Prediction{Ranking: [8]uint8{7}}
	before := output
	if m.Predict(&s.Intent, &s.Candidates, new(Workspace), &output) == nil || output != before {
		t.Fatal("nonfinite input partially wrote output")
	}
}

func BenchmarkPredict(b *testing.B) {
	s := example()
	m, _ := New([ParameterCount]float32{})
	var work Workspace
	var p Prediction
	b.ReportAllocs()
	for b.Loop() {
		if err := m.Predict(&s.Intent, &s.Candidates, &work, &p); err != nil {
			b.Fatal(err)
		}
	}
}
