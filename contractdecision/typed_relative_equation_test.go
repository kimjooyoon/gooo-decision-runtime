package contractdecision

import (
	"math"
	"testing"
)

// Independent scalar layout/equations: no production encoder, term builder,
// forward method or layout constant participates in this reference calculation.
func referenceTypedRelative(t *testing.T, weights *typedRelativeHead, choice int, input [528]float32,
	outputs CaseSource, conditions ConditionSource) [2]float64 {
	t.Helper()
	var source [4]float64
	for h := range 4 {
		sum := float64(weights[2468+h])
		for j, v := range input {
			sum += float64(v) * 8 * float64(weights[356+h*528+j])
		}
		source[h] = math.Tanh(sum)
	}
	var pools [2][4]float64
	for channel := range 2 {
		count, start, bias, width := outputs.CaseCount(), 0, 172, 43
		if channel == 1 {
			count, start, bias, width = conditions.ConditionCount(), 176, 352, 44
		}
		for i := range count {
			var row [32]float32
			var err error
			if channel == 0 {
				row, err = outputs.CaseFeatures(i)
			} else {
				row, err = conditions.ConditionFeatures(i)
			}
			if err != nil {
				t.Fatal(err)
			}
			var fields [43]float64
			for j, v := range row {
				fields[j] = float64(v) * 8
			}
			polarity := (float64(row[6]) - float64(row[4])) * 8
			signs := [3]float64{float64(row[1]), float64(row[2]), float64(row[3])}
			byteStart := 12
			if channel == 1 {
				polarity = (float64(row[2]) - float64(row[1])) * 8
				signs = [3]float64{float64(row[4]), float64(row[3]), float64(row[0]) - float64(row[3]) - float64(row[4])}
				byteStart = 5
			}
			for j, v := range signs {
				fields[32+j] = v * 8 * polarity
			}
			for j := range 8 {
				fields[35+j] = float64(row[byteStart+j]) * 8 * polarity
			}
			for h := range 4 {
				sum := float64(weights[bias+h])
				for j, v := range fields {
					sum += v * float64(weights[start+h*width+j])
				}
				if channel == 1 {
					sum += float64(row[16+choice]) * 8 * float64(weights[start+h*width+43])
				}
				pools[channel][h] += math.Tanh(sum) / float64(count)
			}
		}
	}
	result := [2]float64{float64(weights[9180]), float64(weights[9181])}
	for h := range 12 {
		start := 2472 + h*556
		sum := float64(weights[9144+h])
		for j, v := range input {
			sum += float64(v) * float64(weights[start+j])
		}
		for j := range 4 {
			s, o, c := source[j], pools[0][j], pools[1][j]
			for term, v := range [7]float64{s, o, c, s * o, s * c, o * c, s * o * c} {
				sum += v * float64(weights[start+528+4*term+j])
			}
		}
		if sum < 0 {
			sum *= .01
		}
		for option := range 2 {
			result[option] += sum * float64(weights[9156+12*option+h])
		}
	}
	return result
}

func TestTypedSourceRelativeIndependentEquations(t *testing.T) {
	s := typedRelativeFixture()
	m := &TypedSourceRelativeModel{weights: initialTypedRelative(2026101269)}
	var workspace TypedSourceRelativeWorkspace
	var out ChoicePrediction
	if err := m.PredictChoicesInto(s.Inputs, s.Cases, s.Conditions, &workspace, &out); err != nil {
		t.Fatal(err)
	}
	for choice, row := range s.Inputs {
		want := referenceTypedRelative(t, &m.weights[choice], choice, row, s.Cases, s.Conditions)
		for option := range 2 {
			if math.Abs(float64(out.Logits[choice][option])-want[option]) > 2e-5 {
				t.Fatalf("choice%d option%d: got%g want%g", choice, option, out.Logits[choice][option], want[option])
			}
		}
	}
}
