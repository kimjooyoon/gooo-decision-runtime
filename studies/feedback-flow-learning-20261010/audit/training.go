package main

import (
	"encoding/json"
	"reflect"
	"slices"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type trainingObservation struct {
	ID, SourceSHA, PlanSHA, CaseSHA, InputSHA string
	SampleIndex                               int
	NS                                        int64
	Mask                                      uint16
	ConditionPresent, OutputPresent           bool
	Condition                                 pathplan.ConditionFailure
	Output                                    pathplan.OutputFailure
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
func observedFeatures(s source, o trainingObservation) [][384]float32 {
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

func auditTraining(root string, sources []source, samples []flowdecision.Sample) {
	require(len(samples) == 64, "64 fixed training samples")
	var initial []flowdecision.Sample
	for _, s := range sources {
		if s.Split == "future_train" {
			require(s.Form == "direct", "only direct training form")
			initial = append(initial, flowdecision.Sample{Inputs: s.V6, Masks: []uint16{0, 1, 2, 3}, Acceptable: s.Acceptable})
		}
	}
	require(len(initial) == 16 && reflect.DeepEqual(samples[:16], initial) && hash(jsonBytes(initial)) == "e62fd490c2224aef24c25906e0768e666390d8e4f1b6381258bcb24af64251c4", "unchanged original training inputs")
	observations := lines[trainingObservation](read(root, "training-observations.jsonl.gz"))
	require(len(observations) == 48, "48 actual observations")
	n := 0
	for _, s := range sources {
		if s.Split != "future_train" {
			continue
		}
		for mask := range uint16(4) {
			if s.Acceptable>>mask&1 != 0 {
				continue
			}
			o := observations[n]
			require(o.ID == s.ID && o.SourceSHA == s.SourceSHA && o.PlanSHA == s.PlanSHA && o.CaseSHA == hash(jsonBytes(s.Document.TestCases)) && o.Mask == mask && o.SampleIndex == n+16 && o.NS > 0, "observation identity and order")
			c, hc, v, hv := expectedFailures(s, mask)
			require((hc || hv) && o.ConditionPresent == hc && o.OutputPresent == hv && reflect.DeepEqual(o.Condition, c) && o.Output == v, "actual failures agree with frozen exact outcomes")
			features := observedFeatures(s, o)
			want := flowdecision.Sample{Inputs: features, Masks: []uint16{0, 1, 2, 3}, Acceptable: s.Acceptable}
			require(o.InputSHA == featureSHA(features) && reflect.DeepEqual(samples[n+16], want), "exact observed inputs with complete original labels")
			n++
		}
	}
	require(n == 48, "no held-out training context")
}
