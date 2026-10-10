package main

import (
	"math"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/ordered-requirement-learning-20261010/record"
)

type DecisionDiagnostic struct {
	Programs, ModelApplied, Declined       int
	ComparisonCorrect, BranchLayoutCorrect int
	OutputMacroSum, ConditionMacroSum      float64
	MaxAbsoluteScoreGap                    [2]float64
}

type InteractionRow struct {
	ID                                   string
	Reverse, OutputGoal, ConditionGoal   int
	RequiredComparison, RequiredBranches int
	FirstComparisons, FirstBranches      map[string]int
}

// The finite distance template has a known three-way branch relation. Check it
// against the full independently validated acceptable mask, then inspect the
// original recorded proposals. This performs no inference or candidate calls.
func diagnose(sources []r.Source, byMode map[string]r.Search) ([]InteractionRow, map[string]DecisionDiagnostic) {
	rows := make([]InteractionRow, 0, len(sources))
	groups := map[string]DecisionDiagnostic{}
	for _, x := range sources {
		s := x.Spec
		comparison := s.ConditionGoal
		branches := s.Reverse ^ s.OutputGoal ^ s.ConditionGoal
		mask := comparison | branches<<1
		need(x.Acceptable == uint64(1)<<mask, "distance template interaction differs from oracle")
		row := InteractionRow{ID: s.ID, Reverse: s.Reverse, OutputGoal: s.OutputGoal,
			ConditionGoal: s.ConditionGoal, RequiredComparison: comparison, RequiredBranches: branches,
			FirstComparisons: map[string]int{}, FirstBranches: map[string]int{}}
		for _, mode := range []string{"deterministic", "requirements", "ordered"} {
			search := byMode[s.ID+"/"+mode]
			first := search.Progress[1].NewAttempts[0]
			row.FirstComparisons[mode] = int(first.Mask & 1)
			row.FirstBranches[mode] = int(first.Mask >> 1 & 1)
			keys := []string{mode + "/all", mode + "/" + s.Split}
			if s.Split != "train" {
				keys = append(keys, mode+"/heldout_satisfiable")
			}
			for _, key := range keys {
				g := groups[key]
				g.Programs++
				if search.Ranking.Applied {
					g.ModelApplied++
				}
				if search.Ranking.Declined {
					g.Declined++
				}
				if row.FirstComparisons[mode] == comparison {
					g.ComparisonCorrect++
				}
				if row.FirstBranches[mode] == branches {
					g.BranchLayoutCorrect++
				}
				g.OutputMacroSum += float64(first.Passed) / float64(first.Total)
				conditionPassed := 0
				for _, c := range first.Conditions {
					if c.Passed {
						conditionPassed++
					}
				}
				g.ConditionMacroSum += float64(conditionPassed) / float64(len(first.Conditions))
				for i := range 2 {
					gap := math.Abs(float64(search.Ranking.Logits[i][1]) - float64(search.Ranking.Logits[i][0]))
					g.MaxAbsoluteScoreGap[i] = max(g.MaxAbsoluteScoreGap[i], gap)
				}
				groups[key] = g
			}
		}
		rows = append(rows, row)
	}
	return rows, groups
}
