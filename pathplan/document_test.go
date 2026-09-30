package pathplan

import (
	"encoding/json"
	"testing"
)

func TestExplicitFiniteCaseContract(t *testing.T) {
	for _, raw := range []string{`{}`, `{"input":1}`, `{"expected":0}`, `{"input":null,"expected":0}`, `{"input":0,"expected":null}`, `{"input":0,"expected":false}`, `{"input":0,"expected":0,"expected":1}`, `{"Input":0,"expected":0}`, `{"input":0,"expected":0,"extra":1}`} {
		var test TestCase
		if json.Unmarshal([]byte(raw), &test) == nil {
			t.Fatalf("invalid finite case accepted: %s", raw)
		}
	}
	var test TestCase
	if err := json.Unmarshal([]byte(`{"input":0,"expected":0}`), &test); err != nil {
		t.Fatal(err)
	}
	document := Document{Schema: DocumentSchema, Plan: interactingPlan(), TestCases: []TestCase{{Input: 3, Expected: 17}}, MaxAttempts: 4}
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeDocument(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeDocument(append(raw, raw...)); err == nil {
		t.Fatal("trailing document accepted")
	}
	prepared, err := document.Prepare()
	if err != nil || prepared.ActivityName() != document.Plan.Base.Name {
		t.Fatalf("document preparation: %v", err)
	}
	document.Plan.Base.Expressions[1].Int = 999
	if value, err := prepared.Fallback().Evaluate(3); err != nil || value.Int != 16 {
		t.Fatal("document input mutation changed the prepared fallback")
	}
	document.MaxAttempts = 65
	if _, err := document.Prepare(); err == nil {
		t.Fatal("preparation bypassed document budgets")
	}
}
