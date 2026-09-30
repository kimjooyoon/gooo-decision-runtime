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
}
