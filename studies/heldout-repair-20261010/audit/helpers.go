// Read-only audit: no model prediction, fitting, source preparation or execution.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

type output struct {
	Input, Expected, Actual int64
	Passed                  bool
}
type candidate struct {
	Mask       uint16
	Outputs    []output
	Conditions []pathplan.ConditionResult
	Acceptable bool
}
type source struct {
	ID, Group, Form, Split, Source, SourceSHA, PlanSHA string
	Document                                           pathplan.Document
	V4, V5                                             [][384]float32
	V4SHA, V5SHA                                       string
	Acceptable                                         uint64
	Candidates                                         []candidate
	Contexts                                           []pathplan.SemanticBranchContext
	V6                                                 [][384]float32
}
type initial struct {
	ID, SourceSHA, InputSHA string
	Acceptable              uint64
	Prediction              flowdecision.Prediction
	Explanation             flowdecision.Explanation
}
type modelRecord struct {
	Name, SHA, Fingerprint, InitialSHA string
	Model                              *flowdecision.Model `json:"-"`
	Initial                            map[string]initial  `json:"-"`
}
type observation struct {
	ID, SourceSHA, PlanSHA, CaseSHA, InputSHA string
	Mask                                      uint16
	NS                                        int64
	ConditionPresent, OutputPresent           bool
	Condition                                 pathplan.ConditionFailure
	Output                                    pathplan.OutputFailure
	Inputs                                    [][384]float32
}
type outcome struct {
	Mask        uint16
	Body, GoSHA string
	Outputs     []pathplan.TestResult
	Conditions  []pathplan.ConditionResult
	Valid       bool
}
type judgment struct {
	ID, Split, Form, Model, InputSHA            string
	ObservedMask, InitialRemaining              uint16
	InitialRemainingValid, RawValid, RawRepeats bool
	Prediction                                  flowdecision.Prediction
	Explanation                                 flowdecision.Explanation
	Selected                                    outcome
	NS                                          int64
}
type group struct {
	Rows, RawValid, RawRepeats, NextValid, InitialRemainingValid int
	FeedbackImproved, FeedbackRegressed, FeedbackUnchanged       int
	OutputPassed, OutputTotal, ConditionPassed, ConditionTotal   int
}
type report struct {
	Schema, Producer, DatasetSHA, Go, OS, Arch                                     string
	Sources, Observations, ModelCalls, SelectedEvaluations, Fits, NativeExecutions int
	ObservationNS                                                                  int64
	Models                                                                         []modelRecord
	Groups                                                                         map[string]group
	MedianNS, P95NS                                                                map[string]int64
}

func must(e error) {
	if e != nil {
		panic(e)
	}
}

func require(ok bool, s string) {
	if !ok {
		panic(s)
	}
}

func hash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

func read(root, name string) []byte {
	b, e := os.ReadFile(filepath.Join(root, name))
	must(e)
	if strings.HasSuffix(name, ".gz") {
		z, e := gzip.NewReader(bytes.NewReader(b))
		must(e)
		b, e = io.ReadAll(z)
		must(e)
		must(z.Close())
	}
	return b
}

func decode[T any](b []byte) T { var x T; must(json.Unmarshal(b, &x)); return x }

func lines[T any](b []byte) []T {
	var result []T
	for line := range bytes.SplitSeq(b, []byte{'\n'}) {
		if len(line) > 0 {
			result = append(result, decode[T](line))
		}
	}
	return result
}

func choiceSHA(f [384]float32) string {
	var b [1536]byte
	for i, x := range f {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(x))
	}
	return hash(b[:])
}

func relational(s source) [][384]float32 {
	require(len(s.Contexts) == len(s.V5), "source context count")
	f := slices.Clone(s.V5)
	for i, view := range s.Contexts {
		if view.Normalized {
			must(decision.RelationalFlowFeaturesInto(s.V5[i], view.Flow, &f[i]))
		}
	}
	return f
}

func featureSHA(inputs [][384]float32) string {
	h := sha256.New()
	_, _ = h.Write([]byte{byte(len(inputs))})
	for _, f := range inputs {
		var b [1536]byte
		for i, x := range f {
			binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(x))
		}
		_, _ = h.Write(b[:])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func auditOutputs(s source, mask uint16, outs []pathplan.TestResult) int {
	require(mask < 4, "finite mask")
	good := 0
	for i, o := range outs {
		require(i < len(s.Document.TestCases), "finite output index")
		c := s.Document.TestCases[i]
		expected := s.Candidates[mask].Outputs[i]
		require(o.Input == c.Input && o.Expected == c.Expected && o.Actual == expected.Actual && o.Passed == (o.Actual == o.Expected), "exact recorded output")
		if o.Passed {
			good++
		}
	}
	return good
}

func auditConditions(s source, mask uint16, conditions []pathplan.ConditionResult) int {
	good := 0
	for i, c := range conditions {
		require(i < len(s.Document.Plan.ConditionCases) && c.Case == s.Document.Plan.ConditionCases[i], "authored condition")
		require(reflect.DeepEqual(c, s.Candidates[mask].Conditions[i]), "frozen condition outcome")
		if c.Passed {
			good++
		}
	}
	return good
}

func auditTrace(j judgment) {
	e := j.Explanation
	require(e.ChoiceCount == 2 && e.CandidateCount == 4, "first-call diagnostic trace")
	for c := range 16 {
		for _, h := range e.Hidden[c] {
			require(!math.IsNaN(float64(h)) && !math.IsInf(float64(h), 0), "finite signed activation")
		}
		if c >= 2 {
			require(e.Hidden[c] == [24]float32{} && e.OptionScores[c] == [2]float32{}, "unused choice trace")
		}
	}
	for mask := range 64 {
		if mask >= 4 {
			require(e.CandidateScores[mask] == 0, "unused candidate trace")
			continue
		}
		want := float64(e.OptionScores[0][mask&1]) + float64(e.OptionScores[1][mask>>1&1])
		require(e.CandidateScores[mask] == want, "actual additive candidate score")
	}
	best := 0
	for i := 1; i < 4; i++ {
		if e.CandidateScores[i] > e.CandidateScores[best] {
			best = i
		}
	}
	require(j.Prediction.Selected == uint16(best), "trace scores select the recorded candidate")
}

func jsonBytes(v any) []byte { b, e := json.Marshal(v); must(e); return b }

func expectedFailures(s source, mask uint16) (pathplan.ConditionFailure, bool, pathplan.OutputFailure, bool) {
	var condition pathplan.ConditionFailure
	var output pathplan.OutputFailure
	var hasCondition, hasOutput bool
	for _, c := range s.Candidates[mask].Conditions {
		if !c.Passed {
			condition, hasCondition = pathplan.ConditionFailure{Mask: mask, Result: c}, true
			break
		}
	}
	for _, o := range s.Candidates[mask].Outputs {
		if !o.Passed {
			output = pathplan.OutputFailure{Mask: mask, Result: pathplan.TestResult{Input: o.Input, Expected: o.Expected, Actual: o.Actual, Passed: false}}
			hasOutput = true
			break
		}
	}
	return condition, hasCondition, output, hasOutput
}

// Rebuild observation channels from recorded exact integer/Boolean facts only.
// This performs no parsing, body evaluation or model call.

func observedFeatures(s source, o observation) [][384]float32 {
	features := slices.Clone(s.V6)
	for i, choice := range s.Document.Plan.Decisions {
		var condition decision.ConditionFeedback
		var output decision.OutputFeedback
		if o.ConditionPresent {
			c := o.Condition.Result
			condition = decision.ConditionFeedback{Present: true, ChoiceCount: 2, Choice: uint8(i), CandidateMask: o.Mask, Input: c.Case.Input, Expected: c.Case.Expected, Reached: c.Observation.Reached, Actual: c.Observation.Reached && c.Observation.Value}
			found := false
			for j, declared := range s.Document.Plan.Decisions {
				if declared.ID == c.Case.ChoiceID {
					condition.ObservedChoice, found = uint8(j), true
				}
			}
			require(found, "observed declared condition choice")
		}
		if o.OutputPresent {
			v := o.Output.Result
			output = decision.OutputFeedback{Present: true, ChoiceCount: 2, Choice: uint8(i), CandidateMask: o.Mask, Input: v.Input, Expected: v.Expected, Actual: v.Actual}
		}
		var encoded [320]float32
		must(decision.ExecutionFeaturesInto(s.Contexts[i].Source, [20]byte{}, choice.Intent, condition, output, &encoded))
		copy(features[i][192:236], encoded[192:236])
		copy(features[i][256:320], encoded[256:320])
	}
	return features
}

func next(scores [64]float64, rejected uint16) uint16 {
	best := uint16(4)
	for mask := range uint16(4) {
		if mask != rejected && (best == 4 || scores[mask] > scores[best]) {
			best = mask
		}
	}
	return best
}

func addGroup(g group, j judgment) group {
	g.Rows++
	if j.RawValid {
		g.RawValid++
	}
	if j.RawRepeats {
		g.RawRepeats++
	}
	if j.Selected.Valid {
		g.NextValid++
	}
	if j.InitialRemainingValid {
		g.InitialRemainingValid++
	}
	switch {
	case !j.InitialRemainingValid && j.Selected.Valid:
		g.FeedbackImproved++
	case j.InitialRemainingValid && !j.Selected.Valid:
		g.FeedbackRegressed++
	default:
		g.FeedbackUnchanged++
	}
	for _, o := range j.Selected.Outputs {
		g.OutputTotal++
		if o.Passed {
			g.OutputPassed++
		}
	}
	for _, c := range j.Selected.Conditions {
		g.ConditionTotal++
		if c.Passed {
			g.ConditionPassed++
		}
	}
	return g
}
