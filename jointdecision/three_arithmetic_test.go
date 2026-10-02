package jointdecision

import (
	"encoding/binary"
	"math"
	"testing"
)

// Populate every row/channel so bias-only fixtures cannot conceal wrong strides,
// the third source channel, decoded signs, per-matrix scales or temperature.
func TestThreeNonzeroMatricesAgainstIndependentArithmetic(t *testing.T) {
	parts := threeParts(t)
	text, _ := EncodeThree(parts)
	var features [ThreeFeatureDim]float32
	if err := FeaturesIntoThree(text, &features); err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
		t.Run(variant, func(t *testing.T) {
			name, meta, raw := threeFixture(t, variant)
			meta.Temperature = 2
			var tensors [4][]float32
			for n, tensor := range meta.Tensors {
				tensors[n] = make([]float32, tensor.Count)
				if tensor.Encoding == "ternary_base3_5" {
					meta.Tensors[n].Scale = float64([4]float32{0.125, 1, 0.25, 1}[n])
					for i := 0; i < tensor.Count; i += 5 {
						packed, place := byte(0), byte(1)
						for j := range 5 {
							code := 0
							if i+j < tensor.Count {
								code = ((i+j)*7+(i+j)/11+n)%3 - 1
								tensors[n][i+j] = float32(code) * float32(meta.Tensors[n].Scale)
							}
							packed += byte(code+1) * place
							place *= 3
						}
						raw[int(tensor.Offset)+i/5] = packed
					}
				} else {
					for i := range tensors[n] {
						value := float32(((i*7+i/11+n)%23)-11) / 32
						tensors[n][i] = value
						binary.LittleEndian.PutUint32(raw[int(tensor.Offset)+4*i:], math.Float32bits(value))
					}
				}
			}
			meta.WeightsSHA = digest(raw)
			writeFixture(t, name, meta, raw)
			model, err := LoadThree(name)
			if err != nil {
				t.Fatal(err)
			}
			var workspace ThreeWorkspace
			var prediction ThreePrediction
			if err = model.PredictInto(text, &workspace, &prediction); err != nil {
				t.Fatal(err)
			}
			var hidden [24]float64
			var logits [8]float64
			for row := range hidden {
				value := float64(tensors[1][row])
				for col, feature := range features {
					value += float64(tensors[0][row*768+col]) * float64(feature)
				}
				hidden[row] = math.Max(value, 0)
				if math.Abs(float64(workspace.Hidden[row])-hidden[row]) > 1e-5 {
					t.Fatal("first matrix/reference differs", row)
				}
			}
			maximum := math.Inf(-1)
			for row := range logits {
				logits[row] = float64(tensors[3][row])
				for col, value := range hidden {
					logits[row] += float64(tensors[2][row*24+col]) * value
				}
				if math.Abs(float64(prediction.Logits[row])-logits[row]) > 1e-5 {
					t.Fatal("last matrix/reference differs", row)
				}
				maximum = math.Max(maximum, logits[row]/2)
			}
			var total float64
			for row := range logits {
				logits[row] = math.Exp(logits[row]/2 - maximum)
				total += logits[row]
			}
			best := 0
			for row := range logits {
				if math.Abs(float64(prediction.Probabilities[row])-logits[row]/total) > 1e-6 {
					t.Fatal("eight-label temperature/reference differs", row)
				}
				if logits[row] > logits[best] {
					best = row
				}
			}
			if prediction.Mask != uint16(best) || workspace.Features != features {
				t.Fatal("full mask or projected channel mismatch")
			}
			parts[2] += " 새로운 의도."
			changed, _ := EncodeThree(parts)
			var altered ThreePrediction
			if model.PredictInto(changed, &workspace, &altered) != nil || altered.Logits == prediction.Logits {
				t.Fatal("third intent channel ignored")
			}
			parts = threeParts(t)
		})
	}
}

func TestThreePredictionNilAndNumericalErrorsKeepCallerStorage(t *testing.T) {
	text, _ := EncodeThree(threeParts(t))
	name, meta, raw := threeFixture(t, "fp32")
	var features [ThreeFeatureDim]float32
	FeaturesIntoThree(text, &features)
	for i, value := range features {
		weight := float32(math.MaxFloat32)
		if value < 0 {
			weight = -weight
		}
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(weight))
	}
	meta.WeightsSHA = digest(raw)
	writeFixture(t, name, meta, raw)
	m, err := LoadThree(name)
	if err != nil {
		t.Fatal(err)
	}
	workspace := ThreeWorkspace{Hidden: [24]float32{19}}
	output := ThreePrediction{Mask: 5}
	before, expected := workspace, output
	for _, model := range []*ThreeModel{nil, {}, m} {
		if model.PredictInto(text, &workspace, &output) == nil || workspace != before || output != expected {
			t.Fatal("invalid model/numerical overflow changed caller storage")
		}
	}
	if m.PredictInto(text, nil, &output) == nil || m.PredictInto(text, &workspace, nil) == nil {
		t.Fatal("nil request storage accepted")
	}
}
