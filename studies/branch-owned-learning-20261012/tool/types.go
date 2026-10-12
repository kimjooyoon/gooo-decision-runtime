package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type probe struct {
	Input    int64 `json:"input"`
	Expected int64 `json:"expected"`
}
type specification struct {
	ID        string   `json:"id"`
	Split     string   `json:"split"`
	Family    int      `json:"family"`
	Parameter int      `json:"parameter"`
	Goal      int      `json:"goal"`
	Wording   int      `json:"wording"`
	Form      int      `json:"form"`
	Contract  string   `json:"contract"`
	PartsSHA  string   `json:"parts_sha256"`
	Parity    int      `json:"parity"`
	Language  string   `json:"language"`
	Source    string   `json:"source"`
	SourceSHA string   `json:"source_sha256"`
	Probes    [4]probe `json:"probes"`
}
type candidate struct {
	Mask       uint16                     `json:"mask"`
	Outputs    []pathplan.TestResult      `json:"outputs"`
	Conditions []pathplan.ConditionResult `json:"conditions"`
	Probes     []pathplan.TestResult      `json:"probes"`
	Acceptable bool                       `json:"acceptable"`
}
type record struct {
	Specification   specification                           `json:"specification"`
	Document        pathplan.Document                       `json:"document"`
	PlanSHA         string                                  `json:"plan_sha256"`
	CaseSHA         string                                  `json:"case_sha256"`
	V4              [2][528]float32                         `json:"v4_inputs"`
	V3              [2][528]float32                         `json:"v3_inputs"`
	Outputs         [8][32]float32                          `json:"outputs"`
	Conditions      [3][32]float32                          `json:"conditions"`
	OutputRelations [2]pathplan.SourceRelativeOutputContext `json:"output_relations"`
	BranchOutputs   [2]pathplan.BranchOutputContext         `json:"branch_outputs"`
	Candidates      [4]candidate                            `json:"candidates"`
	Acceptable      uint64                                  `json:"acceptable"`
}
type requirements struct {
	Outputs    [8][32]float32
	Conditions [3][32]float32
}

func (r requirements) CaseCount() int             { return 8 }
func (r requirements) ConditionCount() int        { return 3 }
func (r requirements) CaseFeatureVersion() string { return decision.DeclaredCaseFeatureVersion }
func (r requirements) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}
func (r requirements) CaseFeatures(i int) ([32]float32, error) {
	if i < 0 || i >= 8 {
		return [32]float32{}, fmt.Errorf("case index")
	}
	return r.Outputs[i], nil
}
func (r requirements) ConditionFeatures(i int) ([32]float32, error) {
	if i < 0 || i >= 3 {
		return [32]float32{}, fmt.Errorf("condition index")
	}
	return r.Conditions[i], nil
}
func writeJSON(name string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(name, append(raw, '\n'), 0600)
}
func begin(root, name string) error {
	file, err := os.OpenFile(filepath.Join(root, name+".started"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	return file.Close()
}
