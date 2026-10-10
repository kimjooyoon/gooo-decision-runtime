package contractdecision

import (
	"math"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func interactionRequirementFixture(t *testing.T, pooling string) (*InteractionRequirementModel, [][OrderedFeatureDim]float32, rows, conditionRows) {
	t.Helper()
	m, err := NewInteractionRequirementConditioned(initialInteractionRequirements(19), pooling)
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

// Independent scalar equations spell out each associated field and polynomial
// term, without calling the production encoders, term builder or backprop.
func referenceInteractionRequirement(m *InteractionRequirementModel, choice int, input [OrderedFeatureDim]float32, outputs rows, conditions conditionRows) [2]float64 {
	var source [PoolDim]float64
	for h := range PoolDim {
		sum := float64(m.weights[interactionRequirementContextBias+h])
		for j, v := range input {
			sum += float64(v) * 8 * float64(m.weights[interactionRequirementContextStart+h*OrderedFeatureDim+j])
		}
		source[h] = math.Tanh(sum)
	}
	var pools [2][PoolDim]float64
	for channel, cases := range []rows{outputs, rows(conditions)} {
		start, bias, width := 0, 43*PoolDim, 43
		if channel == 1 {
			start, width = 43*PoolDim+PoolDim, 44
			bias = start + width*PoolDim
		}
		for i, row := range cases {
			var encoded [43]float64
			for j, v := range row {
				encoded[j] = float64(v) * 8
			}
			polarity := (float64(row[6]) - float64(row[4])) * 8
			signs := [3]float64{float64(row[1]), float64(row[2]), float64(row[3])}
			bytes := 12
			if channel == 1 {
				polarity = (float64(row[2]) - float64(row[1])) * 8
				signs = [3]float64{float64(row[4]), float64(row[3]), float64(row[0]) - float64(row[3]) - float64(row[4])}
				bytes = 5
			}
			for j, v := range signs {
				encoded[32+j] = v * 8 * polarity
			}
			for j := range 8 {
				encoded[35+j] = float64(row[bytes+j]) * 8 * polarity
			}
			for h := range PoolDim {
				sum := float64(m.weights[bias+h])
				for j, v := range encoded {
					sum += v * float64(m.weights[start+h*width+j])
				}
				if channel == 1 {
					sum += float64(row[16+choice]) * 8 * float64(m.weights[start+h*width+43])
				}
				value := math.Tanh(sum)
				if m.extreme {
					old := pools[channel][h]
					if i == 0 || math.Abs(value) > math.Abs(old) || math.Abs(value) == math.Abs(old) && value > old {
						pools[channel][h] = value
					}
				} else {
					pools[channel][h] += value / float64(len(cases))
				}
			}
		}
	}
	result := [2]float64{float64(m.weights[interactionRequirementScoreBias]), float64(m.weights[interactionRequirementScoreBias+1])}
	for h := range HiddenDim {
		sum := float64(m.weights[interactionRequirementHiddenBias+h])
		start := interactionRequirementHiddenStart + h*interactionRequirementJointDim
		for j, v := range input {
			sum += float64(v) * float64(m.weights[start+j])
		}
		for j := range PoolDim {
			s, o, c := source[j], pools[0][j], pools[1][j]
			terms := [7]float64{s, o, c, s * o, s * c, o * c, s * o * c}
			for term, value := range terms {
				sum += value * float64(m.weights[start+OrderedFeatureDim+term*PoolDim+j])
			}
		}
		if sum <= 0 {
			sum *= .01
		}
		for option := range 2 {
			result[option] += sum * float64(m.weights[interactionRequirementScoreStart+option*HiddenDim+h])
		}
	}
	return result
}

func TestInteractionRequirementConditionedIndependentEquation(t *testing.T) {
	if InteractionRequirementParameterCount != 19034 {
		t.Fatal("weight contract changed", InteractionRequirementParameterCount)
	}
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		m, inputs, outputs, conditions := interactionRequirementFixture(t, pooling)
		for _, suite := range []conditionRows{nil, conditions} {
			var w InteractionRequirementWorkspace
			var got ChoicePrediction
			if err := m.PredictChoicesInto(inputs, outputs, suite, &w, &got); err != nil {
				t.Fatal(err)
			}
			for choice, input := range inputs {
				want := referenceInteractionRequirement(m, choice, input, outputs, suite)
				for option := range 2 {
					if math.Abs(float64(got.Logits[choice][option])-want[option]) > 3e-5 {
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

func TestInteractionRequirementConditionedGradient(t *testing.T) {
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		m, inputs, outputs, conditions := interactionRequirementFixture(t, pooling)
		masks := []uint16{0, 1, 3, 7}
		var w InteractionRequirementWorkspace
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
		var gradient [InteractionRequirementParameterCount]float64
		m.backwardInteractions(inputs, &w, &residual, &gradient)
		loss := func() float64 {
			var local InteractionRequirementWorkspace
			var out Prediction
			if err := m.PredictInto(inputs, outputs, conditions, masks, &local, &out); err != nil {
				t.Fatal(err)
			}
			_, a := distribution(local.scores[:len(masks)], allCandidates(len(masks)))
			_, b := distribution(local.scores[:len(masks)], 2)
			return a - b
		}
		indices := []int{0, 31, interactionRequirementOutputBias, interactionRequirementConditionStart + 2,
			interactionRequirementConditionStart + interactionCaseDim, interactionRequirementConditionBias, interactionRequirementContextStart + 11,
			interactionRequirementHiddenStart + 17, interactionRequirementHiddenStart + OrderedFeatureDim + PoolDim,
			interactionRequirementHiddenBias + 2, interactionRequirementScoreStart + 1, interactionRequirementScoreBias + 1}
		for h := range PoolDim {
			indices = append(indices, h*interactionCaseDim, h*interactionCaseDim+interactionCaseDim-1, interactionRequirementOutputBias+h,
				interactionRequirementConditionStart+h*(interactionCaseDim+1)+1, interactionRequirementConditionStart+h*(interactionCaseDim+1)+2,
				interactionRequirementConditionStart+h*(interactionCaseDim+1)+interactionCaseDim, interactionRequirementConditionBias+h,
				interactionRequirementContextStart+h*OrderedFeatureDim, interactionRequirementContextStart+(h+1)*OrderedFeatureDim-1)
		}
		for h := range PoolDim {
			indices = append(indices, interactionRequirementContextBias+h,
				h*interactionCaseDim+CaseDim, interactionRequirementConditionStart+h*(interactionCaseDim+1)+CaseDim,
				interactionRequirementConditionStart+h*(interactionCaseDim+1)+interactionCaseDim-1)
		}
		for h := range HiddenDim {
			for term := range interactionTermCount {
				indices = append(indices, interactionRequirementHiddenStart+h*interactionRequirementJointDim+OrderedFeatureDim+term*PoolDim,
					interactionRequirementHiddenStart+h*interactionRequirementJointDim+OrderedFeatureDim+(term+1)*PoolDim-1)
			}
			indices = append(indices, interactionRequirementHiddenStart+h*interactionRequirementJointDim,
				interactionRequirementHiddenStart+h*interactionRequirementJointDim+FeatureDim,
				interactionRequirementHiddenStart+h*interactionRequirementJointDim+OrderedFeatureDim-1,
				interactionRequirementHiddenStart+h*interactionRequirementJointDim+OrderedFeatureDim,
				interactionRequirementHiddenStart+h*interactionRequirementJointDim+OrderedFeatureDim+PoolDim,
				interactionRequirementHiddenStart+(h+1)*interactionRequirementJointDim-1, interactionRequirementHiddenBias+h,
				interactionRequirementScoreStart+h, interactionRequirementScoreStart+HiddenDim+h)
		}
		nonzeroCondition, nonzeroOrderedContext, nonzeroOrderedHidden := false, false, false
		associatedGradient, tripleGradient := false, false
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
			if i >= interactionRequirementConditionStart && i <= interactionRequirementConditionBias && math.Abs(gradient[i]) > 1e-7 {
				nonzeroCondition = true
			}
			if math.Abs(gradient[i]) > 1e-7 {
				if i >= interactionRequirementConditionStart && i < interactionRequirementConditionBias {
					field := (i - interactionRequirementConditionStart) % (interactionCaseDim + 1)
					associatedGradient = associatedGradient || field >= CaseDim && field < interactionCaseDim
				}
				if i >= interactionRequirementHiddenStart && i < interactionRequirementHiddenBias {
					field := (i - interactionRequirementHiddenStart) % interactionRequirementJointDim
					tripleGradient = tripleGradient || field >= OrderedFeatureDim+6*PoolDim
				}

				if i >= interactionRequirementContextStart && i < interactionRequirementHiddenStart && (i-interactionRequirementContextStart)%OrderedFeatureDim >= FeatureDim {
					nonzeroOrderedContext = true
				}
				if i >= interactionRequirementHiddenStart && i < interactionRequirementHiddenBias {
					cell := (i - interactionRequirementHiddenStart) % interactionRequirementJointDim
					nonzeroOrderedHidden = nonzeroOrderedHidden || cell >= FeatureDim && cell < OrderedFeatureDim
				}
			}
		}
		if !associatedGradient || !tripleGradient {
			t.Fatal("association/triple gradients never exercised", pooling)
		}
		if !nonzeroCondition {
			t.Fatal("condition gradients never exercised")
		}
		if !nonzeroOrderedContext || !nonzeroOrderedHidden {
			t.Fatal("ordered operands did not train both source connections", pooling)
		}
	}
}
