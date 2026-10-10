package contractdecision

import (
	"bytes"
	"context"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

type literalRows struct{ rows }

func (literalRows) CaseFeatureVersion() string { return decision.SourceLiteralCaseFeatureVersion }

func TestCaseVersionArtifactAndAtomicReaderMismatch(t *testing.T) {
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		weights := initial(17)
		old, _ := NewForPooling(weights, pooling)
		model, err := NewForCaseFeatures(weights, pooling, decision.SourceLiteralCaseFeatureVersion)
		if err != nil || model.Weights() != old.Weights() || model.Fingerprint() == old.Fingerprint() || model.ArtifactSchema() != SourceLiteralSchema {
			t.Fatal("explicit ABI", err)
		}
		raw, err := model.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := Decode(raw)
		if err != nil || loaded.Fingerprint() != model.Fingerprint() {
			t.Fatal("round trip", err)
		}
		for _, bad := range [][]byte{
			bytes.Replace(raw, []byte(SourceLiteralSchema), []byte(PoolingSchema), 1),
			bytes.Replace(raw, []byte(decision.SourceLiteralCaseFeatureVersion), []byte(decision.DeclaredCaseFeatureVersion), 1),
		} {
			if _, err := Decode(bad); err == nil {
				t.Fatal("mislabelled feature ABI")
			}
		}
		input := make([][FeatureDim]float32, 1)
		cases := literalRows{rows{{.125, .125}}}
		var work Workspace
		var prediction Prediction
		var trace Explanation
		if err := loaded.ExplainInto(input, cases, []uint16{0, 1}, &work, &prediction, &trace); err != nil {
			t.Fatal(err)
		}
		wantW, wantP, wantT := work, prediction, trace
		if err := loaded.ExplainInto(input, cases.rows, []uint16{0, 1}, &work, &prediction, &trace); err == nil || work != wantW || prediction != wantP || trace != wantT {
			t.Fatal("ABI mismatch changed outputs", err)
		}
		if err := old.PredictInto(input, cases, []uint16{0, 1}, &work, &prediction); err == nil {
			t.Fatal("legacy model accepted new rows")
		}
		var choices ChoicePrediction
		if err := loaded.PredictChoicesInto(input, cases.rows, &work, &choices); err == nil || work != wantW {
			t.Fatal("choice ABI mismatch", err)
		}
		samples := []Sample{{Inputs: input, Cases: cases, Masks: []uint16{0, 1}, Acceptable: 1}}
		options := FitOptions{Epochs: 1, LearningRate: .1, Seed: 17}
		trained, _, err := FitForCaseFeatures(context.Background(), samples, options, pooling, decision.SourceLiteralCaseFeatureVersion)
		if err != nil || trained.CaseFeatureVersion() != decision.SourceLiteralCaseFeatureVersion {
			t.Fatal("fit ABI", err)
		}
		if model, _, err := Fit(context.Background(), samples, options); err == nil || model != nil {
			t.Fatal("legacy fit accepted literal rows")
		}
	}
}
