package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/requirement-learning-20261010/record"
)

func checkConditionRows(x r.Source) {
	for i, c := range x.Document.Plan.ConditionCases {
		var row [32]float32
		row[0], row[1] = .125, .125
		if c.Expected {
			row[1], row[2] = 0, .125
		}
		if c.Input == 0 {
			row[3] = .125
		}
		if c.Input < 0 {
			row[4] = .125
		}
		var raw [8]byte
		binary.BigEndian.PutUint64(raw[:], uint64(c.Input))
		for j, b := range raw {
			row[5+j] = float32(b) / 2048
		}
		row[13], row[14], row[15] = float32(i)/1024, float32(len(x.ConditionRows))/1024, 2.0/128
		row[16+x.Spec.Order] = .125
		need(row == x.ConditionRows[i], "complete exact-int64 condition encoding")
	}
}
func conditionSHA(rows [][32]float32) string {
	h := sha256.New()
	h.Write([]byte(decision.DeclaredConditionFeatureVersion + "\x00"))
	var raw [128]byte
	binary.LittleEndian.PutUint32(raw[:4], uint32(len(rows)))
	h.Write(raw[:4])
	for _, row := range rows {
		for i, v := range row {
			binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(v))
		}
		h.Write(raw[:])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

type storedRows struct{ source r.Source }

func (s storedRows) CaseCount() int             { return len(s.source.CaseRows) }
func (s storedRows) CaseFeatureVersion() string { return decision.DeclaredCaseFeatureVersion }
func (s storedRows) CaseFeatures(i int) ([32]float32, error) {
	return s.source.CaseRows[i], nil
}
func (s storedRows) ConditionCount() int             { return len(s.source.ConditionRows) }
func (s storedRows) ConditionFeatureVersion() string { return decision.DeclaredConditionFeatureVersion }
func (s storedRows) ConditionFeatures(i int) ([32]float32, error) {
	return s.source.ConditionRows[i], nil
}

// Recompute information bounds from stored arrays; this makes no prediction,
// performs no source lowering and never executes a candidate.
func checkInputAudits(sources []r.Source, report r.Report) {
	var old []contractdecision.Sample
	var required []contractdecision.RequirementSample
	var ids []string
	for _, x := range sources {
		if x.Acceptable == 0 {
			continue
		}
		rows := storedRows{x}
		sample := contractdecision.Sample{Inputs: x.Inputs, Cases: rows, Masks: []uint16{0, 1, 2, 3}, Acceptable: x.Acceptable}
		old = append(old, sample)
		required = append(required, contractdecision.RequirementSample{Sample: sample, Conditions: rows})
		ids = append(ids, x.Spec.ID)
	}
	need(len(ids) == 168 && reflect.DeepEqual(ids, report.AuditedIDs), "audited source identities")
	a, err := contractdecision.AuditInputs(context.Background(), old)
	must(err)
	b, err := contractdecision.AuditRequirements(context.Background(), required)
	must(err)
	need(len(report.InputAudits) == 2 && reflect.DeepEqual(a, report.InputAudits["choice"]) && reflect.DeepEqual(b, report.InputAudits["requirements"]), "complete model input bounds")
}
