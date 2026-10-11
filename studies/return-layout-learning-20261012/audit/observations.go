package main

import (
	"fmt"
	"math"
	"reflect"
)

func measure(c candidate) counts {
	r := counts{Documents: 1, Outputs: len(c.Outputs), Conditions: len(c.Conditions), Probes: len(c.ProbeResults)}
	if c.Acceptable {
		r.Complete = 1
	}
	for _, x := range c.Outputs {
		if x.Passed {
			r.OutputMatches++
		}
	}
	for _, x := range c.Conditions {
		if x.Passed {
			r.ConditionMatches++
		}
	}
	for _, x := range c.ProbeResults {
		if x.Passed {
			r.ProbeMatches++
		}
	}
	return r
}
func sum(a, b counts) counts {
	return counts{a.Documents + b.Documents, a.Complete + b.Complete, a.OutputMatches + b.OutputMatches,
		a.Outputs + b.Outputs, a.ConditionMatches + b.ConditionMatches, a.Conditions + b.Conditions, a.ProbeMatches + b.ProbeMatches, a.Probes + b.Probes}
}
func auditObservations(sources []source, rows []observation, expected map[string]splitResult) error {
	if len(rows) != len(sources) {
		return fmt.Errorf("observations missing")
	}
	splits := map[string]splitResult{}
	for i, r := range rows {
		s := sources[i]
		if r.ID != s.ID || r.Split != s.Split || r.FirstMask > 3 || r.CheckedMask > 3 || r.PredictNS <= 0 {
			return fmt.Errorf("prediction identity differs")
		}
		total := float64(0)
		best := 0
		for j, p := range r.Probabilities {
			if math.IsNaN(float64(p)) || math.IsInf(float64(p), 0) || p < 0 || p > 1 {
				return fmt.Errorf("invalid ranking probability")
			}
			total += float64(p)
			if p > r.Probabilities[best] {
				best = j
			}
		}
		if math.Abs(total-1) > 1e-6 || best != int(r.FirstMask) {
			return fmt.Errorf("first ranked mask differs")
		}
		var seen [4]bool
		mask, attempts := -1, 0
		for attempt := range 4 {
			next := -1
			for j, p := range r.Probabilities {
				if !seen[j] && (next < 0 || p > r.Probabilities[next]) {
					next = j
				}
			}
			if attempt == 0 {
				next = int(r.FirstMask)
			}
			seen[next] = true
			attempts++
			if s.Candidates[next].Acceptable {
				mask = next
				break
			}
		}
		if mask < 0 || mask != int(r.CheckedMask) || attempts != r.Attempts || r.First != measure(s.Candidates[r.FirstMask]) || r.Checked != measure(s.Candidates[r.CheckedMask]) {
			return fmt.Errorf("first/checked observation differs")
		}
		x := splits[r.Split]
		x.First, x.Checked, x.FixedOrder = sum(x.First, r.First), sum(x.Checked, r.Checked), sum(x.FixedOrder, measure(s.Candidates[0]))
		x.CheckedAttempts += r.Attempts
		splits[r.Split] = x
	}
	if !reflect.DeepEqual(splits, expected) {
		return fmt.Errorf("split summary differs")
	}
	return nil
}
