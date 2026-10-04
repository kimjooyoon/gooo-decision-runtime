package jointdecision

import (
	"encoding/json"
	"math"
	"strings"
	"sync"
	"testing"
)

func originChoices() [3]RecordOriginChoice {
	var result [3]RecordOriginChoice
	for i, field := range [3]string{"title", "state", "reason"} {
		result[i].RecordChoice = RecordChoice{field, "copy." + field, "saved." + field, "저장한 원래 값을 유지한다. Keep the saved value."}
		result[i].Origins[0][0], result[i].Origins[0][8], result[i].Origins[0][15] = 1, 1, 1
		result[i].Origins[1][0], result[i].Origins[1][5], result[i].Origins[1][15] = 1, 1, 1
	}
	return result
}

func originText(t *testing.T) string {
	t.Helper()
	text, err := EncodeRecordOriginThree(originChoices())
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func TestRecordOriginFeaturesDistinguishOriginsAndPreserveCompleteInputs(t *testing.T) {
	choices := originChoices()
	text := originText(t)
	parts, err := RecordOriginThreeParts(text)
	if err != nil {
		t.Fatal(err)
	}
	for i, part := range parts {
		raw, _ := json.Marshal(choices[i])
		if part != string(raw) {
			t.Fatal("complete expression/intent/ancestry was changed")
		}
	}
	var current, changed, repeated [768]float32
	if err = FeaturesIntoRecordOriginThree(text, &current); err != nil {
		t.Fatal(err)
	}
	choices[2].Origins[1] = choices[2].Origins[0]
	other, err := EncodeRecordOriginThree(choices)
	if err != nil || FeaturesIntoRecordOriginThree(other, &changed) != nil || current == changed {
		t.Fatal("distinct source ancestry collided", err)
	}
	if FeaturesIntoRecordOriginThree(text, &repeated) != nil || current != repeated {
		t.Fatal("repeated array changed")
	}
	var square float64
	for _, value := range current {
		square += float64(value) * float64(value)
	}
	if math.Abs(square-1) > 1e-6 || FeaturesIntoRecordThree(text, &repeated) == nil {
		t.Fatal("normalization or old input separation differs")
	}
}

func TestRecordOriginInputDeclinesAtomically(t *testing.T) {
	text := originText(t)
	output := [768]float32{99}
	for _, bad := range []string{"", text + "x", text[:len(text)-1], recordOriginPrefix + "99999:x",
		strings.Replace(text, "|", "|0", 1), strings.Replace(text, `"field":"title"`, `"extra":0,"field":"title"`, 1),
		strings.Replace(text, `"origins":[[1`, `"origins":[[513`, 1)} {
		before := output
		if FeaturesIntoRecordOriginThree(bad, &output) == nil || output != before {
			t.Fatal("invalid origin input committed output", bad)
		}
	}
	for _, mutate := range []func(*RecordOriginChoice){
		func(c *RecordOriginChoice) { c.Origins[0] = [16]uint16{} },
		func(c *RecordOriginChoice) { c.Origins[0][0], c.Origins[0][1] = 300, 300 },
		func(c *RecordOriginChoice) { c.Intent = strings.Repeat("한", 400) },
		func(c *RecordOriginChoice) { c.Intent = string([]byte{0xff}) },
	} {
		choices := originChoices()
		mutate(&choices[0])
		if _, err := EncodeRecordOriginThree(choices); err == nil {
			t.Fatal("invalid or incomplete ancestry accepted")
		}
	}
}

func TestRecordOriginSharedModelContractsAndRequestOwnedArrays(t *testing.T) {
	text := originText(t)
	for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
		t.Run(variant, func(t *testing.T) {
			name, meta, raw := recordSharedFixture(t, variant)
			if _, err := LoadRecordOriginSharedThree(name); err == nil {
				t.Fatal("older weights accepted")
			}
			// Synthetic ABI fixture only: these are not newly trained weights.
			meta.Feature, meta.MaxBytes = RecordOriginSharedFeatureVersion, RecordOriginInputMaxBytes
			writeFixture(t, name, meta, raw)
			m, err := LoadRecordOriginSharedThree(name)
			if err != nil || m.FeatureVersion() != RecordOriginSharedFeatureVersion {
				t.Fatal(err)
			}
			if _, err = LoadRecordSharedThree(name); err == nil {
				t.Fatal("old loader accepted new origin contract")
			}
			var w, prepared ThreeWorkspace
			var p, q ThreePrediction
			if err = m.PredictRecordOriginSharedInto(text, &w, &p); err != nil {
				t.Fatal(err)
			}
			if m.PredictRecordOriginSharedFeaturesInto(&w.Features, &prepared, &q) != nil || w != prepared || p != q {
				t.Fatal("text/prepared origin predictions differ")
			}
			if allocations := testing.AllocsPerRun(100, func() {
				if m.PredictRecordOriginSharedFeaturesInto(&w.Features, &prepared, &q) != nil {
					t.Fatal("prepared prediction failed")
				}
			}); allocations != 0 {
				t.Fatal("prepared prediction allocated", allocations)
			}
			before, prior := w, p
			if m.PredictRecordSharedInto(sharedRecordText(t), &w, &p) == nil || w != before || p != prior {
				t.Fatal("old prediction accepted changed meaning")
			}
			if m.PredictRecordOriginSharedInto("incomplete", &w, &p) == nil || w != before || p != prior {
				t.Fatal("invalid input changed caller storage")
			}
			var wait sync.WaitGroup
			for range 12 {
				wait.Go(func() {
					var scratch ThreeWorkspace
					var result ThreePrediction
					if m.PredictRecordOriginSharedInto(text, &scratch, &result) != nil || result != prior {
						t.Error("concurrent request changed prediction")
					}
				})
			}
			wait.Wait()
			meta.MaxBytes = ThreeInputMaxBytes
			writeFixture(t, name, meta, raw)
			if _, err = LoadRecordOriginSharedThree(name); err == nil {
				t.Fatal("old byte bound accepted")
			}
		})
	}
}
