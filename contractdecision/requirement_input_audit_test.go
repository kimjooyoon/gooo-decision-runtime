package contractdecision

import (
	"context"
	"errors"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestRequirementInputAuditIncludesAllConditionCellsAndKeepsLegacyScope(t *testing.T) {
	inputs := make([][FeatureDim]float32, 1)
	a, b := make(conditionRows, 128), make(conditionRows, 128)
	for i := range a {
		if err := decision.DeclaredConditionFeaturesInto(decision.DeclaredConditionFeatureInput{
			Input: 9007199254740993 + int64(i), ChoiceCount: 1, Index: i, Count: 128}, &a[i]); err != nil {
			t.Fatal(err)
		}
	}
	copy(b, a)
	b[127][1], b[127][2] = 0, 1.0/8
	first := Sample{Inputs: inputs, Cases: rows{{}}, Masks: []uint16{0, 1}, Acceptable: 1}
	second := first
	second.Acceptable = 2
	samples := []RequirementSample{{Sample: first, Conditions: a}, {Sample: second, Conditions: b}}
	legacy, err := AuditInputs(context.Background(), []Sample{first, second})
	if err != nil || legacy.BestFirstPasses != 1 || legacy.UnavoidableMisses != 1 {
		t.Fatal("old input contract changed", legacy, err)
	}
	current, err := AuditRequirements(context.Background(), samples)
	if err != nil || current.Schema != RequirementInputAuditSchema || len(current.Groups) != 2 || current.BestFirstPasses != 2 || current.UnavoidableMisses != 0 {
		t.Fatal("last condition did not distinguish goals", current, err)
	}
	samples[1].Conditions = a
	current, err = AuditRequirements(context.Background(), samples)
	if err != nil || len(current.Groups) != 1 || current.BestFirstPasses != 1 || current.UnavoidableMisses != 1 {
		t.Fatal("true condition-input conflict hidden", current, err)
	}
	for _, reader := range []ConditionSource{nil, &countedConditions{n: 129}, &countedConditions{n: 128, fail: 127}, &countedConditions{n: 1, fail: -1, nonfinite: true}} {
		samples[1].Conditions = reader
		if result, err := AuditRequirements(context.Background(), samples); err == nil || result != nil {
			t.Fatal("invalid condition audit", err)
		}
	}
	samples[1].Conditions = b
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := AuditRequirements(ctx, samples); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled audit", err)
	}
}
