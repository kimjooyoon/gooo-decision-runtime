package jointdecision

import (
	"encoding/json"
	"math"
	"sync"
	"testing"
)

func recordSharedFixture(t *testing.T, variant string) (string, Metadata, []byte) {
	t.Helper()
	_, name, meta, raw := compactFixture(t, variant)
	meta.Feature, meta.Arithmetic = RecordSharedFeatureVersion, SeparateArithmeticVersion
	writeFixture(t, name, meta, raw)
	return name, meta, raw
}

func sharedRecordText(t *testing.T) string {
	t.Helper()
	text, err := EncodeRecordThree([3]RecordChoice{
		{"title", "input0.title", "input0.state", "Keep the original title."},
		{"state", `"ready"`, `"wait"`, "상태를 ready 문자열로 설정한다."},
		{"reason", `input0.reason + ":accepted"`, `":accepted" + input0.reason`, "Append :accepted to the original reason."},
	})
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func TestSharedRecordModelLoadsAndMatchesPreparedIndependentFieldScores(t *testing.T) {
	text := sharedRecordText(t)
	for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
		t.Run(variant, func(t *testing.T) {
			name, _, _ := recordSharedFixture(t, variant)
			m, err := LoadRecordSharedThree(name)
			if err != nil {
				t.Fatal(err)
			}
			packed, resident, scales := 8288, 8288, 0
			if variant != "fp32" {
				packed, resident, scales = 446, 2096, 8
			}
			if m.Schema() != SharedThreeSchema || m.FeatureVersion() != RecordSharedFeatureVersion || m.ArithmeticVersion() != SeparateArithmeticVersion || m.PackedFileBytes() != packed || m.ResidentTensorBytes() != resident || m.MatrixScaleBytes() != scales {
				t.Fatal("shared record memory/contract differs")
			}
			var w, prepared ThreeWorkspace
			var p, q ThreePrediction
			if err = m.PredictRecordSharedInto(text, &w, &p); err != nil {
				t.Fatal(err)
			}
			if err = m.PredictRecordSharedFeaturesInto(&w.Features, &prepared, &q); err != nil || w != prepared || p != q {
				t.Fatal("prepared and source projections differ", err)
			}
			var local [3][2]float64
			for part := range 3 {
				for bit := range 2 {
					for hidden := range 8 {
						weight := float64(0)
						if variant == "fp32" {
							weight = float64(m.inner.floatWeights[sharedW1+SharedHiddenDim+bit*8+hidden])
						} else {
							weight = float64(m.inner.codes[sharedW1+bit*8+hidden]) * float64(m.inner.w2Scale)
						}
						local[part][bit] += float64(w.Hidden[part*8+hidden]) * weight
					}
				}
			}
			var total float64
			var weights [8]float64
			for mask := range 8 {
				score := local[0][mask&1] + local[1][(mask>>1)&1] + local[2][(mask>>2)&1]
				if math.Abs(float64(p.Logits[mask])-score) > 1e-5 {
					t.Fatal("independent shared mask sum differs")
				}
				weights[mask] = math.Exp(score / float64(m.inner.temperature))
				total += weights[mask]
			}
			for i, v := range weights {
				if math.Abs(float64(p.Probabilities[i])-v/total) > 1e-6 {
					t.Fatal("composed probability differs")
				}
			}
			marginals, err := RecordChoiceMarginals(p)
			if err != nil {
				t.Fatal(err)
			}
			for part := range 3 {
				if math.Abs(marginals[part][0]+marginals[part][1]-1) > 1e-6 {
					t.Fatal("field probability sum differs")
				}
			}
			before, prior := w, p
			if m.PredictInto(text, &w, &p) == nil || m.PredictRecordInto(text, &w, &p) == nil || w != before || p != prior {
				t.Fatal("old explicit contracts changed or accepted record-shared ABI")
			}
			if _, err = LoadSharedThree(name); err == nil {
				t.Fatal("old integer shared loader accepted record weights")
			}
			if _, err = LoadRecordThree(name); err == nil {
				t.Fatal("dense record loader accepted shared weights")
			}
			var wait sync.WaitGroup
			for range 12 {
				wait.Go(func() {
					var scratch ThreeWorkspace
					var out ThreePrediction
					if m.PredictRecordSharedInto(text, &scratch, &out) != nil || out != prior {
						t.Error("request-owned concurrent prediction differs")
					}
				})
			}
			wait.Wait()
		})
	}
}

func TestSharedRecordErrorsPreserveCallerStateAndVersion(t *testing.T) {
	name, meta, raw := recordSharedFixture(t, "fp32")
	m, err := LoadRecordSharedThree(name)
	if err != nil {
		t.Fatal(err)
	}
	w := ThreeWorkspace{Hidden: [24]float32{17}}
	p := ThreePrediction{Mask: 6}
	before, prior := w, p
	var features [768]float32
	features[0] = float32(math.Inf(1))
	for _, err := range []error{m.PredictRecordSharedInto("incomplete", &w, &p), m.PredictRecordSharedFeaturesInto(&features, &w, &p), m.PredictRecordSharedInto(sharedRecordText(t), nil, &p)} {
		if err == nil || w != before || p != prior {
			t.Fatal("invalid shared record input mutated storage")
		}
	}
	for _, change := range []func(*Metadata){func(v *Metadata) { v.Feature = RecordFieldFeatureVersion }, func(v *Metadata) { v.Arithmetic = "" }, func(v *Metadata) { v.HiddenDim = 24 }, func(v *Metadata) { v.Labels[0] = "first" }} {
		copyRaw, _ := json.Marshal(meta)
		var altered Metadata
		json.Unmarshal(copyRaw, &altered)
		change(&altered)
		writeFixture(t, name, altered, raw)
		if _, err = LoadRecordSharedThree(name); err == nil {
			t.Fatal("different shared record contract accepted")
		}
	}
	if _, err = RecordChoiceMarginals(ThreePrediction{}); err == nil {
		t.Fatal("unnormalized field probabilities accepted")
	}
}
