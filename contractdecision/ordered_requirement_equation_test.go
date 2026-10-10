package contractdecision

import (
	"math"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func orderedRequirementFixture(t *testing.T, pooling string) (*OrderedRequirementModel, [][OrderedFeatureDim]float32, rows, conditionRows) {
	t.Helper()
	m, err := NewOrderedRequirementConditioned(initialOrderedRequirements(19), pooling)
	if err != nil {
		t.Fatal(err)
	}
	_, original, outputCases := choiceFixture(t, pooling)
	inputs := make([][OrderedFeatureDim]float32, len(original))
	for i, row := range original {
		copy(inputs[i][:], row[:])
		for j := FeatureDim; j < OrderedFeatureDim; j++ {
			inputs[i][j] = float32((j+3*i)%11-5) / 16
		}
	}
	conditions := make(conditionRows, 3)
	for i := range conditions {
		err := decision.DeclaredConditionFeaturesInto(decision.DeclaredConditionFeatureInput{
			Input: -9007199254740995 + int64(i), Expected: i%2 == 0,
			TargetChoice: i, ChoiceCount: len(inputs), Index: i, Count: len(conditions)}, &conditions[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	return m, inputs, outputCases, conditions
}

// Independent scalar float64 equations join both goal streams after separate
// source-conditioned pooling. This does not call the implementation's encoder.
func referenceOrderedRequirement(m *OrderedRequirementModel, choice int, input [OrderedFeatureDim]float32, outputs rows, conditions conditionRows) [2]float64 {
	var pools [2][PoolDim]float64
	for channel, cases := range []rows{outputs, rows(conditions)} {
		start, bias, width := 0, CaseDim*PoolDim, CaseDim
		if channel == 1 {
			start, width = CaseDim*PoolDim+PoolDim, ConditionDim+1
			bias = start + width*PoolDim
		}
		for i, row := range cases {
			for h := range PoolDim {
				sum := float64(m.weights[bias+h])
				for j, value := range input {
					sum += float64(value) * float64(m.weights[orderedRequirementContextStart+h*OrderedFeatureDim+j])
				}
				for j, value := range row {
					sum += float64(value) * float64(m.weights[start+h*width+j])
				}
				if channel == 1 {
					sum += float64(row[16+choice]) * float64(m.weights[start+h*width+ConditionDim])
				}
				if sum <= 0 {
					sum *= float64(negativeSlope)
				}
				if m.extreme {
					old := pools[channel][h]
					if i == 0 || math.Abs(sum) > math.Abs(old) || math.Abs(sum) == math.Abs(old) && sum > old {
						pools[channel][h] = sum
					}
				} else {
					pools[channel][h] += sum / float64(len(cases))
				}
			}
		}
	}
	result := [2]float64{float64(m.weights[orderedRequirementScoreBias]), float64(m.weights[orderedRequirementScoreBias+1])}
	for h := range HiddenDim {
		sum := float64(m.weights[orderedRequirementHiddenBias+h])
		start := orderedRequirementHiddenStart + h*orderedRequirementJointDim
		for j, value := range input {
			sum += float64(value) * float64(m.weights[start+j])
		}
		for channel := range 2 {
			for j, value := range pools[channel] {
				sum += value * float64(m.weights[start+OrderedFeatureDim+channel*PoolDim+j])
			}
		}
		if sum <= 0 {
			sum *= float64(negativeSlope)
		}
		for option := range 2 {
			result[option] += sum * float64(m.weights[orderedRequirementScoreStart+option*HiddenDim+h])
		}
	}
	return result
}

func TestOrderedRequirementConditionedIndependentEquation(t *testing.T) {
	if OrderedRequirementParameterCount != 17890 {
		t.Fatal("weight contract changed", OrderedRequirementParameterCount)
	}
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		m, inputs, outputs, conditions := orderedRequirementFixture(t, pooling)
		for _, suite := range []conditionRows{nil, conditions} {
			var w OrderedRequirementWorkspace
			var got ChoicePrediction
			if err := m.PredictChoicesInto(inputs, outputs, suite, &w, &got); err != nil {
				t.Fatal(err)
			}
			for choice, input := range inputs {
				want := referenceOrderedRequirement(m, choice, input, outputs, suite)
				for option := range 2 {
					if math.Abs(float64(got.Logits[choice][option])-want[option]) > 5e-6 {
						t.Fatal("joint equation", pooling, choice, got.Logits[choice], want)
					}
				}
				if len(suite) == 0 && w.pool[choice][1] != [PoolDim]float32{} {
					t.Fatal("absent conditions acquired a summary")
				}
			}
		}
	}
}

func TestOrderedRequirementConditionedGradient(t *testing.T) {
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		m, inputs, outputs, conditions := orderedRequirementFixture(t, pooling)
		masks := []uint16{0, 1, 3, 7}
		var w OrderedRequirementWorkspace
		var result Prediction
		if err := m.PredictInto(inputs, outputs, conditions, masks, &w, &result); err != nil {
			t.Fatal(err)
		}
		all, _ := distribution(w.scores[:len(masks)], allCandidates(len(masks)))
		good, _ := distribution(w.scores[:len(masks)], 2)
		var residual [MaxChoices][2]float64
		for i, mask := range masks {
			for c := range inputs {
				residual[c][mask>>c&1] += all[i] - good[i]
			}
		}
		var gradient [OrderedRequirementParameterCount]float64
		m.backwardRequirements(inputs, &w, &residual, &gradient)
		loss := func() float64 {
			var local OrderedRequirementWorkspace
			var out Prediction
			if err := m.PredictInto(inputs, outputs, conditions, masks, &local, &out); err != nil {
				t.Fatal(err)
			}
			_, a := distribution(local.scores[:len(masks)], allCandidates(len(masks)))
			_, b := distribution(local.scores[:len(masks)], 2)
			return a - b
		}
		indices := []int{0, 31, orderedRequirementOutputBias, orderedRequirementConditionStart + 2,
			orderedRequirementConditionStart + 32, orderedRequirementConditionBias, orderedRequirementContextStart + 11,
			orderedRequirementHiddenStart + 17, orderedRequirementHiddenStart + OrderedFeatureDim + PoolDim,
			orderedRequirementHiddenBias + 2, orderedRequirementScoreStart + 1, orderedRequirementScoreBias + 1}
		for h := range PoolDim {
			indices = append(indices, h*CaseDim, h*CaseDim+CaseDim-1, orderedRequirementOutputBias+h,
				orderedRequirementConditionStart+h*(ConditionDim+1)+1, orderedRequirementConditionStart+h*(ConditionDim+1)+2,
				orderedRequirementConditionStart+h*(ConditionDim+1)+ConditionDim, orderedRequirementConditionBias+h,
				orderedRequirementContextStart+h*OrderedFeatureDim, orderedRequirementContextStart+(h+1)*OrderedFeatureDim-1)
		}
		for h := range HiddenDim {
			indices = append(indices, orderedRequirementHiddenStart+h*orderedRequirementJointDim,
				orderedRequirementHiddenStart+h*orderedRequirementJointDim+FeatureDim,
				orderedRequirementHiddenStart+h*orderedRequirementJointDim+OrderedFeatureDim-1,
				orderedRequirementHiddenStart+h*orderedRequirementJointDim+OrderedFeatureDim,
				orderedRequirementHiddenStart+h*orderedRequirementJointDim+OrderedFeatureDim+PoolDim,
				orderedRequirementHiddenStart+(h+1)*orderedRequirementJointDim-1, orderedRequirementHiddenBias+h,
				orderedRequirementScoreStart+h, orderedRequirementScoreStart+HiddenDim+h)
		}
		nonzeroCondition, nonzeroOrderedContext, nonzeroOrderedHidden := false, false, false
		seen := make(map[int]bool)
		for _, i := range indices {
			if seen[i] {
				continue
			}
			seen[i] = true
			old, epsilon := m.weights[i], float32(.001)
			m.weights[i] = old + epsilon
			plus := loss()
			m.weights[i] = old - epsilon
			minus := loss()
			m.weights[i] = old
			numerical := (plus - minus) / float64(2*epsilon)
			if math.Abs(numerical-gradient[i]) > .001 {
				t.Fatalf("%s parameter %d analytic %g numerical %g", pooling, i, gradient[i], numerical)
			}
			if i >= orderedRequirementConditionStart && i <= orderedRequirementConditionBias && math.Abs(gradient[i]) > 1e-7 {
				nonzeroCondition = true
			}
			if math.Abs(gradient[i]) > 1e-7 {
				if i >= orderedRequirementContextStart && i < orderedRequirementHiddenStart && (i-orderedRequirementContextStart)%OrderedFeatureDim >= FeatureDim {
					nonzeroOrderedContext = true
				}
				if i >= orderedRequirementHiddenStart && i < orderedRequirementHiddenBias {
					cell := (i - orderedRequirementHiddenStart) % orderedRequirementJointDim
					nonzeroOrderedHidden = nonzeroOrderedHidden || cell >= FeatureDim && cell < OrderedFeatureDim
				}
			}
		}
		if !nonzeroCondition {
			t.Fatal("condition gradients never exercised")
		}
		if !nonzeroOrderedContext || !nonzeroOrderedHidden {
			t.Fatal("ordered operands did not train both source connections", pooling)
		}
	}
}
