package contractdecision

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestCanonicalInteractionArtifactAndNumericalIdentity(t *testing.T) {
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		weights := initialInteractionRequirements(53)
		old, err := NewInteractionRequirementConditioned(weights, pooling)
		if err != nil {
			t.Fatal(err)
		}
		model, err := NewCanonicalInteractionRequirementConditioned(weights, pooling)
		if err != nil {
			t.Fatal(err)
		}
		oldRaw, _ := old.Marshal()
		raw, err := model.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeCanonicalInteractionRequirementConditioned(raw)
		if err != nil || decoded.Fingerprint() != model.Fingerprint() || decoded.Fingerprint() == old.Fingerprint() || decoded.Weights() != weights || decoded.Pooling() != pooling {
			t.Fatal("distinct artifact identity", err)
		}
		if decoded.SourceFeatureVersion() != CanonicalOrderedSourceFeatureVersion || decoded.ArtifactSchema() != CanonicalInteractionRequirementSchema {
			t.Fatal("source ABI")
		}
		if _, err := DecodeCanonicalInteractionRequirementConditioned(oldRaw); err == nil {
			t.Fatal("legacy artifact accepted as canonical")
		}
		if _, err := DecodeInteractionRequirementConditioned(raw); err == nil {
			t.Fatal("canonical artifact accepted as legacy")
		}
		for _, changed := range [][]byte{
			append(append([]byte{}, raw...), []byte(" {}")...),
			bytes.Replace(raw, []byte(`"bounds":[16,128,128]`), []byte(`"bounds":[15,128,128]`), 1),
			bytes.Replace(raw, []byte(`"schema":`), []byte(`"unknown":true,"schema":`), 1),
			bytes.Replace(raw, []byte(`"schema":`), []byte(`"schema":"duplicate","schema":`), 1),
		} {
			if _, err := DecodeCanonicalInteractionRequirementConditioned(changed); err == nil {
				t.Fatal("invalid artifact accepted")
			}
		}
		var inputs [2][528]float32
		inputs[0][10], inputs[0][400], inputs[1][22], inputs[1][500] = .125, .25, .5, -.125
		for _, conditions := range []conditionRows{nil, {{.125}}, make(conditionRows, 128)} {
			var a, b InteractionRequirementWorkspace
			var x, y ChoicePrediction
			cases := rows{{.125, .25}}
			if old.PredictChoicesInto(inputs[:], cases, conditions, &a, &x) != nil || decoded.PredictChoicesInto(inputs[:], cases, conditions, &b, &y) != nil || x != y {
				t.Fatal("shared numerical equations changed")
			}
			var px, py Prediction
			if old.PredictInto(inputs[:], cases, conditions, []uint16{0, 1, 2, 3}, &a, &px) != nil || decoded.PredictInto(inputs[:], cases, conditions, []uint16{0, 1, 2, 3}, &b, &py) != nil || px != py {
				t.Fatal("candidate scoring changed")
			}
		}
	}
}

func TestCanonicalInteractionFitAndInputErrors(t *testing.T) {
	samples := []InteractionRequirementSample{{Inputs: make([][528]float32, 1), Cases: rows{{.125}}, Conditions: conditionRows{{.125}}, Masks: []uint16{0, 1}, Acceptable: 1}}
	options := FitOptions{Epochs: 2, LearningRate: .01, Seed: 59}
	model, history, err := FitCanonicalInteractionRequirementConditioned(context.Background(), samples, options, MeanPooling)
	if err != nil || model == nil || len(history) != 2 {
		t.Fatal("canonical fit route", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := FitCanonicalInteractionRequirementConditioned(ctx, samples, options, MeanPooling); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var work InteractionRequirementWorkspace
	var prediction ChoicePrediction
	prediction.Count = 3
	beforeWork, beforePrediction := work, prediction
	samples[0].Inputs[0][255] = 3 // legacy source marker
	if err := model.PredictChoicesInto(samples[0].Inputs, samples[0].Cases, samples[0].Conditions, &work, &prediction); err == nil || !reflect.DeepEqual(work, beforeWork) || prediction != beforePrediction {
		t.Fatal("mixed source ABI or mutated error output")
	}
	if _, _, err := FitCanonicalInteractionRequirementConditioned(context.Background(), samples, options, MeanPooling); err == nil {
		t.Fatal("mixed training input ABI")
	}
	var absent *CanonicalInteractionRequirementModel
	if _, err := absent.Marshal(); err == nil || absent.Fingerprint() != "" {
		t.Fatal("nil artifact")
	}
	if absent.PredictChoicesInto(nil, nil, nil, &work, &prediction) == nil {
		t.Fatal("nil predictor")
	}
}

func TestCanonicalAuditRetainsContradictionsAndBounds(t *testing.T) {
	sample := OrderedRequirementSample{Inputs: make([][528]float32, 1), Cases: rows{{.125}}, Conditions: conditionRows{}, Masks: []uint16{0, 1}, Acceptable: 1}
	other := sample
	other.Acceptable = 2
	audit, err := AuditCanonicalRequirements(context.Background(), []OrderedRequirementSample{sample, other})
	if err != nil || audit.Schema != CanonicalRequirementInputAuditSchema || audit.ConflictingGroups != 1 || audit.BestFirstPasses != 1 || audit.UnavoidableMisses != 1 {
		t.Fatal("contradictory labels concealed", audit, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := AuditCanonicalRequirements(ctx, []OrderedRequirementSample{sample}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := AuditCanonicalRequirements(nil, []OrderedRequirementSample{sample}); err == nil {
		t.Fatal("nil audit context")
	}
	if _, _, err := FitCanonicalInteractionRequirementConditioned(nil, []InteractionRequirementSample{sample}, FitOptions{Epochs: 2, LearningRate: .01}, MeanPooling); err == nil {
		t.Fatal("nil fit context")
	}
	for _, n := range []int{0, MaxChoices + 1} {
		if err := validateCanonicalSourceRows(make([][528]float32, n)); err == nil {
			t.Fatal("unbounded source rows", n)
		}
	}
	sample.Inputs[0][192] = 1
	if _, err := AuditCanonicalRequirements(context.Background(), []OrderedRequirementSample{sample}); err == nil {
		t.Fatal("legacy source channels accepted")
	}
}
