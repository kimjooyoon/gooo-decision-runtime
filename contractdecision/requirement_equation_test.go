package contractdecision

import (
	"math"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

type conditionRows [][ConditionDim]float32

func (r conditionRows) ConditionCount() int { return len(r) }
func (r conditionRows) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}
func (r conditionRows) ConditionFeatures(i int) ([ConditionDim]float32, error) { return r[i], nil }

func requirementFixture(t *testing.T, pooling string) (*RequirementModel, [][FeatureDim]float32, rows, conditionRows) {
	t.Helper()
	m, err := NewRequirementConditioned(initialRequirements(19), pooling)
	if err != nil {
		t.Fatal(err)
	}
	_, inputs, outputCases := choiceFixture(t, pooling)
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
func referenceRequirement(m *RequirementModel, choice int, input [FeatureDim]float32, outputs rows, conditions conditionRows) [2]float64 {
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
					sum += float64(value) * float64(m.weights[requirementContextStart+h*FeatureDim+j])
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
	result := [2]float64{float64(m.weights[requirementScoreBias]), float64(m.weights[requirementScoreBias+1])}
	for h := range HiddenDim {
		sum := float64(m.weights[requirementHiddenBias+h])
		start := requirementHiddenStart + h*requirementJointDim
		for j, value := range input {
			sum += float64(value) * float64(m.weights[start+j])
		}
		for channel := range 2 {
			for j, value := range pools[channel] {
				sum += value * float64(m.weights[start+FeatureDim+channel*PoolDim+j])
			}
		}
		if sum <= 0 {
			sum *= float64(negativeSlope)
		}
		for option := range 2 {
			result[option] += sum * float64(m.weights[requirementScoreStart+option*HiddenDim+h])
		}
	}
	return result
}

func TestRequirementConditionedIndependentEquation(t *testing.T) {
	if RequirementParameterCount != 13282 {
		t.Fatal("weight contract changed", RequirementParameterCount)
	}
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		m, inputs, outputs, conditions := requirementFixture(t, pooling)
		for _, suite := range []conditionRows{nil, conditions} {
			var w RequirementWorkspace
			var got ChoicePrediction
			if err := m.PredictChoicesInto(inputs, outputs, suite, &w, &got); err != nil {
				t.Fatal(err)
			}
			for choice, input := range inputs {
				want := referenceRequirement(m, choice, input, outputs, suite)
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

func TestRequirementConditionedGradient(t *testing.T) {
	for _, pooling := range []string{MeanPooling, ExtremePooling} {
		m, inputs, outputs, conditions := requirementFixture(t, pooling)
		masks := []uint16{0, 1, 3, 7}
		var w RequirementWorkspace
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
		var gradient [RequirementParameterCount]float64
		m.backwardRequirements(inputs, &w, &residual, &gradient)
		loss := func() float64 {
			var local RequirementWorkspace
			var out Prediction
			if err := m.PredictInto(inputs, outputs, conditions, masks, &local, &out); err != nil {
				t.Fatal(err)
			}
			_, a := distribution(local.scores[:len(masks)], allCandidates(len(masks)))
			_, b := distribution(local.scores[:len(masks)], 2)
			return a - b
		}
		indices := []int{0, 31, requirementOutputBias, requirementConditionStart + 2,
			requirementConditionStart + 32, requirementConditionBias, requirementContextStart + 11,
			requirementHiddenStart + 17, requirementHiddenStart + FeatureDim + PoolDim,
			requirementHiddenBias + 2, requirementScoreStart + 1, requirementScoreBias + 1}
		for h := range PoolDim {
			indices = append(indices, h*CaseDim, h*CaseDim+CaseDim-1, requirementOutputBias+h,
				requirementConditionStart+h*(ConditionDim+1)+1, requirementConditionStart+h*(ConditionDim+1)+2,
				requirementConditionStart+h*(ConditionDim+1)+ConditionDim, requirementConditionBias+h,
				requirementContextStart+h*FeatureDim, requirementContextStart+(h+1)*FeatureDim-1)
		}
		for h := range HiddenDim {
			indices = append(indices, requirementHiddenStart+h*requirementJointDim,
				requirementHiddenStart+h*requirementJointDim+FeatureDim,
				requirementHiddenStart+h*requirementJointDim+FeatureDim+PoolDim,
				requirementHiddenStart+(h+1)*requirementJointDim-1, requirementHiddenBias+h,
				requirementScoreStart+h, requirementScoreStart+HiddenDim+h)
		}
		nonzeroCondition := false
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
			if i >= requirementConditionStart && i <= requirementConditionBias && math.Abs(gradient[i]) > 1e-7 {
				nonzeroCondition = true
			}
		}
		if !nonzeroCondition {
			t.Fatal("condition gradients never exercised")
		}
	}
}
