package main

import (
	"fmt"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type record struct {
	ID, Split, SourceSHA, PlanSHA, CaseSHA string
	Group, Parity                          int
	Gooo                                   string
	Document                               pathplan.Document
	Inputs                                 [][528]float32
	Outputs, Conditions                    [][32]float32
	Acceptable                             uint64
	Candidates                             []candidate
	Probes                                 []pathplan.TestCase
}

type candidate struct {
	Mask         uint16
	Outputs      []pathplan.TestResult
	Conditions   []pathplan.ConditionResult
	ProbeResults []pathplan.TestResult
	Acceptable   bool
}

type requirements struct{ outputs, conditions [][32]float32 }

func (r requirements) CaseCount() int             { return len(r.outputs) }
func (r requirements) ConditionCount() int        { return len(r.conditions) }
func (r requirements) CaseFeatureVersion() string { return decision.DeclaredCaseFeatureVersion }
func (r requirements) ConditionFeatureVersion() string {
	return decision.DeclaredConditionFeatureVersion
}
func rowAt(rows [][32]float32, i int) ([32]float32, error) {
	if i < 0 || i >= len(rows) {
		return [32]float32{}, fmt.Errorf("row outside bounded stream")
	}
	return rows[i], nil
}
func (r requirements) CaseFeatures(i int) ([32]float32, error)      { return rowAt(r.outputs, i) }
func (r requirements) ConditionFeatures(i int) ([32]float32, error) { return rowAt(r.conditions, i) }
