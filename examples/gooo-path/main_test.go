package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

const fixture = `{"schema":"gooo/body-codegen-typed-path-plan/v1","path_plan":{"schema":"gooo/typed-body-path-plan/v1","base":{"schema":"gooo/typed-body-plan/v1","id":"sdk-context://body","name":"Probe","result_type":"Int","expressions":[{"kind":"input","name":"input"},{"kind":"int","int":2},{"kind":"binary","operation":"subtract","left":0,"right":1}],"statements":[{"kind":"return","expr":2}],"root":[0]},"decisions":[{"id":"operands","kind":"operand_order","target":2,"intent":"입력값에서 2를 뺀다. Subtract two from input.","options":[{"label":"layout_forward"},{"label":"layout_reverse","reverse":true}],"fallback":"layout_forward"}]},"test_cases":[{"input":2,"expected":0},{"input":3,"expected":1}],"max_attempts":2}`

func TestDisconnectedTypedAssemblyIsDeterministic(t *testing.T) {
	a, err := execute([]byte(fixture), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := execute([]byte(fixture), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) || a["gooo_body"] != "return (input - 2)" {
		t.Fatal("disconnected body or receipt changed")
	}
	raw, _ := json.Marshal(a["search"])
	var search pathplan.SearchResult
	if err := json.Unmarshal(raw, &search); err != nil {
		t.Fatal(err)
	}
	if search.Selection.ModelCalls != 0 || search.SelectedTrainingPassed != 2 || search.TrainingTotal != 2 {
		t.Fatal("disconnected prediction or finite contract accounting differs")
	}
}

func TestBoundedContextAndIntentArePreserved(t *testing.T) {
	document, err := pathplan.DecodeDocument([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	choice := document.Plan.Decisions[0]
	text, err := contextIntent(document.Plan, choice)
	if err != nil || !strings.HasSuffix(text, "intent: "+choice.Intent) || !strings.Contains(text, "layout_reverse::true:") {
		t.Fatal("typed choices or bilingual intent lost")
	}
	if document.Plan.Decisions[0].Intent != choice.Intent {
		t.Fatal("caller intent mutated")
	}
	choice.Intent = "literal intent: separator"
	if _, err := contextIntent(document.Plan, choice); err == nil {
		t.Fatal("ambiguous delimiter accepted")
	}
	choice.Intent = strings.Repeat("x", decision.InputMaxBytes)
	if _, err := contextIntent(document.Plan, choice); err == nil {
		t.Fatal("overflow silently truncated")
	}
}

func TestTypedDocumentRejectsMissingExpectedAndInvalidScope(t *testing.T) {
	if _, err := execute([]byte(strings.Replace(fixture, `,"expected":1`, "", 1)), nil); err == nil {
		t.Fatal("missing expected accepted")
	}
	plan := pathplan.Plan{Schema: pathplan.Schema, Base: bodyplan.Plan{Name: "Probe", ResultType: decision.TypeInt}}
	if _, err := pathplan.Prepare(plan); err == nil {
		t.Fatal("empty invalid typed plan accepted")
	}
}
