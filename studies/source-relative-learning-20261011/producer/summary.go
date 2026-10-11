package main

import (
	"math/bits"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/record"
)

func summarize(report *r.Report, x r.Source, row r.Search) {
	require(len(row.Attempts) > 0, "missing attempt")
	first := row.Attempts[0]
	valid := bits.OnesCount64(x.Acceptable)
	for _, key := range []string{row.Mode + "/all", row.Mode + "/" + row.Split} {
		g := report.Groups[key]
		g.Documents++
		if first.Acceptable {
			g.FirstDeclared++
		}
		if x.Candidates[first.Mask].ProbeAcceptable {
			g.FirstProbe++
		}
		if row.Selected != nil {
			g.FinalDeclared++
		}
		if row.ResultState == "PARTIAL" {
			g.PartialDocuments++
		}
		if r.Passed(row.Probe) {
			g.ResultProbe++
		}
		if valid == 0 {
			g.ZeroValidPalette++
		}
		if valid > 1 {
			g.MultipleValidPalette++
		}
		if valid == 1 {
			g.UniqueRoleDocuments++
			accepted := uint16(bits.TrailingZeros64(x.Acceptable))
			if first.Mask&1 == accepted&1 {
				g.ComparisonCorrect++
			}
			if first.Mask>>1&1 == accepted>>1&1 {
				g.BranchCorrect++
			}
		}
		g.ResultPassed += row.ResultPassed
		g.ResultTotal += row.ResultTotal
		g.CandidateChecks += len(row.Attempts)
		g.ModelCalls += len(row.Predictions)
		g.RelationRows += row.RelationRows
		g.RuleBucketChecks += row.RuleBucketChecks
		g.SearchNS += row.NS
		g.PreparationNS += row.PreparationNS
		g.PredictionNS += row.PredictionNS
		g.RuleNS += row.RuleNS
		g.ProbeNS += row.ProbeNS
		report.Groups[key] = g
	}
	report.Searches++
	report.ModelCalls += len(row.Predictions)
}
