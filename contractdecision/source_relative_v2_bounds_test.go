package contractdecision

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
)

// Fresh synthetic rows and zero weights exercise API behavior, not the frozen
// quality corpus, published trained models or recorded performance studies.
func v2SyntheticSample() InteractionRequirementSample {
	s := sourceRelativeFixture()
	for i := range s.Inputs {
		for j := range 3 {
			s.Inputs[i][384+48*j] = 1.0 / 8
		}
	}
	s.Inputs[0][432], s.Inputs[0][441] = 0, 1.0/8
	return s
}

func TestSourceRelativeV2ConcurrentCallerWorkspacesAndTransactionalErrors(t *testing.T) {
	s := v2SyntheticSample()
	m, err := NewSourceRelativeV2([InteractionRequirementParameterCount]float32{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			var work InteractionRequirementWorkspace
			var out Prediction
			if err := m.PredictInto(s.Inputs, s.Cases, s.Conditions, s.Masks, &work, &out); err != nil {
				t.Error(err)
				return
			}
			if out.Count != 4 || out.Selected != 0 {
				t.Error("zero-weight tie changed", out)
			}
		})
	}
	wg.Wait()
	var work InteractionRequirementWorkspace
	before := Prediction{Count: 7, Selected: 9}
	out := before
	bad := v2SyntheticSample()
	bad.Inputs[0][441] = float32(math.NaN())
	if err := m.PredictInto(bad.Inputs, bad.Cases, bad.Conditions, bad.Masks, &work, &out); err == nil || out != before || work != (InteractionRequirementWorkspace{}) {
		t.Fatal("invalid V2 input changed caller destinations")
	}
}

func TestSourceRelativeV2FitBoundsStopBeforeLearning(t *testing.T) {
	options := FitOptions{Epochs: 1, LearningRate: .01}
	s := v2SyntheticSample()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if m, h, err := FitSourceRelativeV2(ctx, []InteractionRequirementSample{s}, options); !errors.Is(err, context.Canceled) || m != nil || h != nil {
		t.Fatal("cancelled fit entered learning", err)
	}
	s.Inputs[0][442] = 1.0 / 8
	if m, h, err := FitSourceRelativeV2(context.Background(), []InteractionRequirementSample{s}, options); err == nil || m != nil || h != nil {
		t.Fatal("invalid V2 flags entered learning")
	}
	if m, h, err := FitSourceRelativeV2(nil, []InteractionRequirementSample{s}, options); err == nil || m != nil || h != nil {
		t.Fatal("nil context entered learning")
	}
}
