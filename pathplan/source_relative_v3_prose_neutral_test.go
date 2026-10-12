package pathplan

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func TestProseNeutralV3ClearsOnlyIntentAndRetainsOwnedRequirements(t *testing.T) {
	cases := []TestCase{{-13, -4}, {10, 0}, {27, -17}, {9007199254740993, -9007199254740983}}
	p, input := outputRelationFixture(t, cases)
	id := p.plan.Decisions[0].ID
	var original, neutral [528]float32
	if err := input.SourceRelativeV3FeaturesInto(id, &original); err != nil {
		t.Fatal(err)
	}
	if err := input.SourceRelativeV3ProseNeutralFeaturesInto(id, &neutral); err != nil {
		t.Fatal(err)
	}
	seen := false
	for slot, value := range original {
		if slot >= 64 && slot < 192 {
			if neutral[slot] != 0 {
				t.Fatal("intent was retained", slot)
			}
			seen = seen || value != 0
		} else if neutral[slot] != value {
			t.Fatal("source or requirement channel changed", slot)
		}
	}
	if !seen {
		t.Fatal("fixture did not exercise a populated intent")
	}
	plan := p.plan
	plan.Decisions = append([]Choice(nil), plan.Decisions...)
	for i := range plan.Decisions {
		plan.Decisions[i].Intent = "선언한 조건과 출력에 맞게 선택한다"
	}
	other, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	otherInput, err := other.InitialContractInput(cases)
	if err != nil {
		t.Fatal(err)
	}
	var translated [528]float32
	if err := otherInput.SourceRelativeV3ProseNeutralFeaturesInto(id, &translated); err != nil || translated != neutral {
		t.Fatal("prose changed neutral model array", err)
	}
	if other.sha == p.sha || other.plan.Decisions[0].Intent != plan.Decisions[0].Intent || input.CaseCount() != len(cases) || input.ConditionCount() != len(p.plan.ConditionCases) {
		t.Fatal("original source identity or requirement rows lost")
	}
	changed := append([]TestCase(nil), cases...)
	changed[0].Expected = 31
	_, changedInput := outputRelationFixture(t, changed)
	var changedFeatures [528]float32
	if err := changedInput.SourceRelativeV3ProseNeutralFeaturesInto(id, &changedFeatures); err != nil || changedFeatures == neutral {
		t.Fatal("changed declared goal disappeared", err)
	}
	before := neutral
	var missing *ContractInput
	if missing.SourceRelativeV3ProseNeutralFeaturesInto(id, &neutral) == nil || neutral != before || input.SourceRelativeV3ProseNeutralFeaturesInto("absent", &neutral) == nil || neutral != before || input.SourceRelativeV3ProseNeutralFeaturesInto(id, nil) == nil {
		t.Fatal("invalid feature request changed output")
	}
}

func TestProseNeutralV3SessionBindsActualInputsAndPredictsBeforeFiniteChecks(t *testing.T) {
	cases := []TestCase{{-13, -4}, {10, 0}, {27, -17}}
	p, input := outputRelationFixture(t, cases)
	m, err := contractdecision.NewSourceRelativeV3ProseNeutral([contractdecision.InteractionRequirementParameterCount]float32{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := sessionContext(t)
	s, err := p.NewSourceRelativeV3ProseNeutralContractSession(ctx, m, cases)
	if err != nil {
		t.Fatal(err)
	}
	r := s.Ranking()
	if r.Calls != 1 || !r.Applied || r.Schema != "gooo/source-relative-v3-prose-neutral-path-ranking/v1" || r.SourceFeatures != contractdecision.SourceRelativeV3ProseNeutralFeatureVersion || r.ModelFingerprint != m.Fingerprint() {
		t.Fatal("neutral ranking identity", r)
	}
	initial, err := s.Observe()
	if err != nil || initial.Attempted != 0 {
		t.Fatal("initial prediction executed a candidate", err)
	}
	for i, c := range p.plan.Decisions {
		var features [528]float32
		if err := input.SourceRelativeV3ProseNeutralFeaturesInto(c.ID, &features); err != nil {
			t.Fatal(err)
		}
		var raw [528 * 4]byte
		for j, value := range features {
			binary.LittleEndian.PutUint32(raw[j*4:], math.Float32bits(value))
		}
		if r.FeatureSHA[i] != hash(raw[:]) {
			t.Fatal("receipt hashed a different input", i)
		}
	}
	progress, body, err := s.Advance(ctx, 4)
	if err != nil || body == nil || progress.Status != "TRAINING_COMPLETE" || progress.PredictionsThisAdvance != 0 || s.Ranking() != r || progress.Selection.ModelVariant != "source_relative_v3_prose_neutral_contract_fp32" {
		t.Fatal("finite construction", err, progress)
	}
	for _, c := range cases {
		value, err := body.Evaluate(c.Input)
		if err != nil || value.Int != c.Expected {
			t.Fatal("original body requirement", err, c)
		}
	}
	deterministic, err := p.NewContractSession(ctx, nil, cases)
	if err != nil {
		t.Fatal(err)
	}
	without, err := p.NewSourceRelativeV3ProseNeutralContractSession(ctx, nil, cases)
	if err != nil || !reflect.DeepEqual(without.Ranking(), deterministic.Ranking()) {
		t.Fatal("nil model changed deterministic traversal", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := p.NewSourceRelativeV3ProseNeutralContractSession(cancelled, m, cases); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled construction", err)
	}
	plan := p.plan
	plan.Base.Expressions = append(plan.Base.Expressions[:0:0], plan.Base.Expressions...)
	plan.Base.Expressions[5].Left = 4
	unsupported, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	declined, err := unsupported.NewSourceRelativeV3ProseNeutralContractSession(ctx, m, cases)
	if err != nil || !declined.Ranking().Declined || declined.Ranking().Calls != 0 || declined.Ranking().Error == "" {
		t.Fatal("unsupported shape predicted or lost reason", err)
	}
}
