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
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/interaction-requirement-learning-20261011/record"
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
	var ordered []contractdecision.OrderedRequirementSample
	ids := map[string][]string{}
	for _, x := range sources {
		rows := storedRows{x}
		if x.OrderedReason == "" {
			ordered = append(ordered, contractdecision.OrderedRequirementSample{Inputs: x.OrderedInputs, Cases: rows, Conditions: rows, Masks: []uint16{0, 1, 2, 3}, Acceptable: x.Acceptable})
			ids["both_models"] = append(ids["both_models"], x.Spec.ID)
		}
	}
	need(len(ordered) == 72 && reflect.DeepEqual(ids, report.AuditedIDs), "audit source coverage")
	b, e := contractdecision.AuditOrderedRequirements(context.Background(), ordered)
	must(e)
	need(len(report.InputAudits) == 1 && reflect.DeepEqual(b, report.InputAudits["both_models"]), "complete shared input audit")
}

// Reconstruct the new ordered suffix from the fixed source template's meaning.
// Uses byte-wise integer encoding, not the production expression encoder.
func checkOrderedRows(x r.Source) {
	if x.Spec.Form == "rare" {
		need(x.OrderedReason == "ORDERED_SOURCE_CONTEXT_UNAVAILABLE: SINGLE_ROOT_BRANCH_REQUIRED" && len(x.OrderedInputs) == 0 && x.OrderedSHA == [2]string{}, "unsupported source has no tensor")
		return
	}
	need(x.OrderedReason == "" && len(x.OrderedInputs) == 2, "ordered source scope")
	var suffix [144]float32
	atom := func(dst []float32, input bool) {
		dst[0] = .125
		if input {
			dst[1] = .125
			return
		}
		dst[2], dst[7] = .125, .125
		for i := range 8 {
			dst[8+i] = float32(byte(uint64(x.Spec.K)>>uint(56-i*8))) / 2048
		}
	}
	suffix[4] = .125
	atom(suffix[16:32], true)
	atom(suffix[32:48], false)
	for arm := range 2 {
		block := suffix[(arm+1)*48 : (arm+2)*48]
		block[2] = .125
		inputFirst := (arm == 0) == (x.Spec.Reverse == 0)
		atom(block[16:32], inputFirst)
		atom(block[32:48], !inputFirst)
	}
	for i, row := range x.OrderedInputs {
		need(reflect.DeepEqual(row[:384], x.Inputs[i][:]) && reflect.DeepEqual(row[384:], suffix[:]), "exact ordered source meaning")
		need(r.OrderedFeatureSHA(row) == x.OrderedSHA[i], "full ordered source hash")
	}
}
