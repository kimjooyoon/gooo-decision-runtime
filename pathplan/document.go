package pathplan

import (
	"encoding/json"
	"errors"
	"unicode/utf8"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/internal/strictjson"
)

const DocumentSchema = "gooo/body-codegen-typed-path-plan/v1"

// Document carries a typed source-bound plan and explicit finite expectations.
// A caller must independently bind Plan.Base to its authoritative source.
type Document struct {
	Schema      string     `json:"schema"`
	Plan        Plan       `json:"path_plan"`
	TestCases   []TestCase `json:"test_cases"`
	MaxAttempts int        `json:"max_attempts"`
	Seed        string     `json:"seed,omitempty"`
}

func (test *TestCase) UnmarshalJSON(data []byte) error {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	if len(keys) != 2 || keys["input"] == nil || keys["expected"] == nil {
		return errors.New("finite case requires exactly input and expected")
	}
	var fields struct {
		Input    *int64 `json:"input"`
		Expected *int64 `json:"expected"`
	}
	if err := strictjson.Decode(data, &fields); err != nil {
		return err
	}
	if fields.Input == nil || fields.Expected == nil {
		return errors.New("finite case requires explicit non-null integer values")
	}
	*test = TestCase{Input: *fields.Input, Expected: *fields.Expected}
	return nil
}

func DecodeDocument(raw []byte) (Document, error) {
	var document Document
	if len(raw) == 0 || len(raw) > 128<<10 {
		return document, errors.New("typed path document byte budget exceeded")
	}
	if err := strictjson.Decode(raw, &document); err != nil {
		return Document{}, err
	}
	if err := document.Validate(); err != nil {
		return Document{}, err
	}
	return document, nil
}

func (document Document) Validate() error {
	_, err := document.Prepare()
	return err
}

// Prepare checks the finite envelope and owns one validated plan snapshot.
// The caller must bind its cached fallback to authoritative source.
func (document Document) Prepare() (*PreparedPlan, error) {
	if document.Schema != DocumentSchema || len(document.TestCases) == 0 || len(document.TestCases) > 128 || document.MaxAttempts < 1 || document.MaxAttempts > 64 || len(document.Seed) > 512 || !utf8.ValidString(document.Seed) {
		return nil, errors.New("typed path document schema or finite budgets are invalid")
	}
	if document.Plan.Base.ResultType != decision.TypeInt {
		return nil, errors.New("integer tests require an integer body result")
	}
	return Prepare(document.Plan)
}
