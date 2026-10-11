package main

import (
	"context"
	"fmt"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

type counts struct {
	Documents, Complete, OutputMatches, Outputs, ConditionMatches, Conditions, ProbeMatches, Probes int
}
type observation struct {
	ID, Split              string
	FirstMask, CheckedMask uint16
	PredictNS              int64
	Probabilities          [4]float32
	First, Checked         counts
	Attempts               int
}
type splitResult struct {
	First, Checked, FixedOrder counts
	CheckedAttempts            int
}

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
		a.Outputs + b.Outputs, a.ConditionMatches + b.ConditionMatches, a.Conditions + b.Conditions,
		a.ProbeMatches + b.ProbeMatches, a.Probes + b.Probes}
}

// This scores retained original interpreter observations. It runs no compiler
// or native executable and sends no candidate outcome or probe row to the model.
func evaluate(ctx context.Context, model *contractdecision.SourceRelativeV2Model, rows []record) ([]observation, map[string]splitResult, error) {
	results := make([]observation, 0, len(rows))
	splits := map[string]splitResult{}
	for _, r := range rows {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		req := requirements{r.Outputs, r.Conditions}
		var work contractdecision.InteractionRequirementWorkspace
		var prediction contractdecision.Prediction
		start := time.Now()
		err := model.PredictInto(r.Inputs, req, req, []uint16{0, 1, 2, 3}, &work, &prediction)
		elapsed := time.Since(start).Nanoseconds()
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", r.ID, err)
		}
		o := observation{ID: r.ID, Split: r.Split, FirstMask: prediction.Selected, PredictNS: elapsed}
		copy(o.Probabilities[:], prediction.Probabilities[:4])
		o.First = measure(r.Candidates[o.FirstMask])
		var seen [4]bool
		for attempt := range 4 {
			best := -1
			for i := range 4 {
				if !seen[i] && (best < 0 || o.Probabilities[i] > o.Probabilities[best]) {
					best = i
				}
			}
			if attempt == 0 {
				best = int(o.FirstMask)
			}
			seen[best] = true
			o.Attempts++
			if r.Candidates[best].Acceptable {
				o.CheckedMask = uint16(best)
				o.Checked = measure(r.Candidates[best])
				break
			}
		}
		s := splits[r.Split]
		s.First, s.Checked, s.FixedOrder = sum(s.First, o.First), sum(s.Checked, o.Checked), sum(s.FixedOrder, measure(r.Candidates[0]))
		s.CheckedAttempts += o.Attempts
		splits[r.Split] = s
		results = append(results, o)
	}
	return results, splits, nil
}
