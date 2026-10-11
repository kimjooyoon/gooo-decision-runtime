package main

import (
	"fmt"
	"reflect"
)

// Preparing source and encoding requirements does not execute a candidate.
func auditSource(s source, index int) error {
	group, parity := index/4, index%4
	split := "train"
	if group >= 20 {
		split = "heldout"
	} else if group >= 15 {
		split = "validation"
	}
	if s.Group != group || s.Parity != parity || s.ID != fmt.Sprintf("layout-%02d-%d", group, parity) || s.Split != split ||
		s.SourceSHA != digest([]byte(s.Gooo)) || s.Acceptable != uint64(1)<<parity || len(s.Inputs) != 2 || len(s.Outputs) != 6 || len(s.Conditions) != 3 || len(s.Candidates) != 4 || len(s.Probes) != 8 {
		return fmt.Errorf("source identity, split or bounded shape differs at %d", index)
	}
	if len(s.Document.Plan.Decisions) != 2 || len(s.Document.TestCases) != 6 || len(s.Document.Plan.ConditionCases) != 3 {
		return fmt.Errorf("original declared requirements missing")
	}
	p, err := s.Document.Prepare()
	if err != nil {
		return err
	}
	input, err := p.InitialContractInput(s.Document.TestCases)
	if err != nil {
		return err
	}
	if p.PlanSHA256() != s.PlanSHA || input.CaseSHA256() != s.CaseSHA {
		return fmt.Errorf("%s plan/cases differ", s.ID)
	}
	for i, choice := range s.Document.Plan.Decisions {
		var row [528]float32
		if err = input.SourceRelativeV2FeaturesInto(choice.ID, &row); err != nil {
			return err
		}
		if row != s.Inputs[i] {
			return fmt.Errorf("%s choice%d V2 source encoding differs", s.ID, i)
		}
	}
	for i, row := range s.Outputs {
		actual, err := input.CaseFeatures(i)
		if err != nil {
			return err
		}
		if actual != row {
			return fmt.Errorf("%s output feature%d differs", s.ID, i)
		}
	}
	for i, row := range s.Conditions {
		actual, err := input.ConditionFeatures(i)
		if err != nil {
			return err
		}
		if actual != row {
			return fmt.Errorf("%s condition feature%d differs", s.ID, i)
		}
	}
	for i, c := range s.Candidates {
		if int(c.Mask) != i || len(c.Outputs) != 6 || len(c.Conditions) != 3 || len(c.ProbeResults) != 8 {
			return fmt.Errorf("candidate shape differs")
		}
		complete := true
		for j, x := range c.Outputs {
			q := s.Document.TestCases[j]
			if x.Input != q.Input || x.Expected != q.Expected || x.Passed != (x.Actual == x.Expected) {
				return fmt.Errorf("output observation differs")
			}
			complete = complete && x.Passed
		}
		for j, x := range c.Conditions {
			status := "MISMATCH"
			if x.Passed {
				status = "MATCH"
			}
			if !reflect.DeepEqual(x.Case, s.Document.Plan.ConditionCases[j]) || !x.Observation.Reached || x.Passed != (x.Observation.Value == x.Case.Expected) || x.Status != status {
				return fmt.Errorf("condition observation differs")
			}
			complete = complete && x.Passed
		}
		for j, x := range c.ProbeResults {
			q := s.Probes[j]
			if x.Input != q.Input || x.Expected != q.Expected || x.Passed != (x.Actual == x.Expected) {
				return fmt.Errorf("probe observation differs")
			}
			for _, used := range s.Document.TestCases {
				if q.Input == used.Input {
					return fmt.Errorf("probe entered source requirements")
				}
			}
			for _, used := range s.Document.Plan.ConditionCases {
				if q.Input == used.Input {
					return fmt.Errorf("probe entered condition requirements")
				}
			}
		}
		if complete != c.Acceptable || complete != (s.Acceptable>>i&1 != 0) {
			return fmt.Errorf("complete label differs")
		}
	}
	return nil
}
