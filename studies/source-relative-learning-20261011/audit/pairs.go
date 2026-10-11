package main

import (
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/record"
	"math/bits"
	"reflect"
)

func pairings(sources []r.Source, records map[string]r.Search, bySpec map[r.Spec]r.Source) (map[string]*change, map[string]*pairGroup, map[string]int) {
	changes := map[string]*change{}
	paired := map[string]*pairGroup{}
	equivalent := map[string]int{}
	for _, s := range sources {
		for _, mode := range []string{"fixed", "rule", "relative"} {
			a, b := records[s.Spec.ID+"/baseline"], records[s.Spec.ID+"/"+mode]
			key := mode + "/" + s.Spec.Split
			if changes[key] == nil {
				changes[key] = &change{}
			}
			ch := changes[key]
			ch.Documents++
			if !a.Attempts[0].Acceptable && b.Attempts[0].Acceptable {
				ch.Improvements = append(ch.Improvements, s.Spec.ID)
			}
			if a.Attempts[0].Acceptable && !b.Attempts[0].Acceptable {
				ch.Regressions = append(ch.Regressions, s.Spec.ID)
			}
		}
		if s.Spec.Form == "assignment" {
			key := s.Spec
			key.ID = ""
			key.Form = "direct"
			other, ok := bySpec[key]
			require(ok && reflect.DeepEqual(s.CanonicalInputs, other.CanonicalInputs) && reflect.DeepEqual(s.RelativeInputs, other.RelativeInputs), "equivalent input differs")
			for _, mode := range []string{"fixed", "rule", "baseline", "relative"} {
				x, y := records[s.Spec.ID+"/"+mode], records[other.Spec.ID+"/"+mode]
				if x.Logits == y.Logits && x.Order == y.Order {
					equivalent[mode]++
				}
			}
		}
		if s.Spec.Diagnostic != "" {
			continue
		}
		for _, kind := range []string{"output", "condition"} {
			key := s.Spec
			key.ID = ""
			if kind == "output" {
				if key.OutputGoal != 0 {
					continue
				}
				key.OutputGoal = 1
			} else {
				if key.ConditionGoal != 0 {
					continue
				}
				key.ConditionGoal = 1
			}
			other, ok := bySpec[key]
			require(ok && reflect.DeepEqual(s.CanonicalInputs, other.CanonicalInputs), "goal pair input missing")
			label := kind
			if kind == "output" {
				require(reflect.DeepEqual(s.ConditionRows, other.ConditionRows) && !reflect.DeepEqual(s.OutputRows, other.OutputRows) && reflect.DeepEqual(s.RelativeInputs, other.RelativeInputs), "output pair differs")
			} else {
				require(!reflect.DeepEqual(s.ConditionRows, other.ConditionRows) && !reflect.DeepEqual(s.RelativeInputs, other.RelativeInputs), "condition pair unchanged")
				if !reflect.DeepEqual(s.OutputRows, other.OutputRows) {
					label = "condition_with_output_change"
				}
			}
			require(bits.OnesCount64(s.Acceptable) == 1 && bits.OnesCount64(other.Acceptable) == 1, "unique paired labels required")
			for _, mode := range []string{"fixed", "rule", "baseline", "relative"} {
				x, y := records[s.Spec.ID+"/"+mode], records[other.Spec.ID+"/"+mode]
				name := label + "/" + s.Spec.Split + "/" + mode
				if paired[name] == nil {
					paired[name] = &pairGroup{}
				}
				g := paired[name]
				g.Pairs++
				ta, tb := uint16(bits.TrailingZeros64(s.Acceptable)), uint16(bits.TrailingZeros64(other.Acceptable))
				for bit := range 2 {
					if (ta^tb)>>bit&1 == 1 {
						g.RequiredChanges[bit]++
					}
					if (x.Order[0]^y.Order[0])>>bit&1 == 1 {
						g.ProposedChanges[bit]++
					}
					if x.Logits[bit] != y.Logits[bit] {
						g.LogitChanges[bit]++
					}
				}
				if x.Attempts[0].Acceptable && y.Attempts[0].Acceptable {
					g.BothFirstCorrect++
				}
				if x.Attempts[0].Acceptable || y.Attempts[0].Acceptable {
					g.EitherFirstCorrect++
				}
			}
		}
	}
	return changes, paired, equivalent
}
