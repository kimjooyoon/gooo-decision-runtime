package main

import (
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/record"
	"math/bits"
	"reflect"
)

func auditSource(s r.Source) {
	require(s.SourceSHA == r.Hash([]byte(s.Gooo)) && len(s.Candidates) == 4, "source digest or palette differs")
	n := 3
	if s.Spec.Diagnostic == "missing_conditions" {
		n = 0
	}
	require(len(s.CanonicalInputs) == 2 && len(s.RelativeInputs) == 2 && len(s.Relations) == 2 && len(s.OutputRows) == 8 && len(s.ConditionRows) == n, "input dimensions differ")
	p, err := s.Document.Prepare()
	must(err)
	input, err := p.InitialContractInput(s.Document.TestCases)
	must(err)
	require(s.PlanSHA == p.PlanSHA256() && s.CasesSHA == input.CaseSHA256(), "plan or output requirement identity differs")
	decisions := s.Document.Plan.Decisions
	require(len(decisions) == 2 && decisions[0].ID == "comparison" && string(decisions[0].Kind) == "operand_order" && decisions[1].ID == "branches" && string(decisions[1].Kind) == "branch_layout", "roles differ")
	for i, c := range decisions {
		var row [528]float32
		must(input.CanonicalOrderedSourceFeaturesInto(c.ID, &row))
		require(row == s.CanonicalInputs[i], "canonical source array differs")
		view, err := input.SourceRelativeConditions(c.ID)
		must(err)
		require(reflect.DeepEqual(view, s.Relations[i]) && view.Available && view.Count == n, "relation context differs")
		copy(row[192:208], view.Features[:])
		require(row == s.RelativeInputs[i], "relative source array differs")
	}
	require(input.CaseCount() == len(s.OutputRows) && input.ConditionCount() == len(s.ConditionRows), "declared input counts differ")
	for i, row := range s.OutputRows {
		got, err := input.CaseFeatures(i)
		must(err)
		require(got == row, "output array differs")
	}
	for i, row := range s.ConditionRows {
		got, err := input.ConditionFeatures(i)
		must(err)
		require(got == row, "condition array differs")
	}
	var good, probe uint64
	for mask, c := range s.Candidates {
		require(int(c.Mask) == mask && len(c.Declared.Outputs) == 8 && len(c.Declared.Conditions) == n && len(c.Probe.Outputs) == 16 && len(c.Probe.Conditions) == 3, "observation dimensions differ")
		for i, o := range c.Declared.Outputs {
			want := s.Document.TestCases[i]
			require(o.Input == want.Input && o.Expected == want.Expected, "authored output changed")
		}
		for i, o := range c.Declared.Conditions {
			require(o.Case == s.Document.Plan.ConditionCases[i], "authored condition changed")
		}
		for i, o := range c.Probe.Outputs {
			want := s.Probes[i]
			require(o.Input == want.Input && o.Expected == want.Expected, "probe output changed")
		}
		for i, o := range c.Probe.Conditions {
			require(o.Case == s.ProbeConditions[i], "probe condition changed")
		}
		require(c.Acceptable == observed(c.Declared) && c.ProbeAcceptable == observed(c.Probe), "candidate completeness differs")
		if c.Acceptable {
			good |= 1 << mask
		}
		if c.ProbeAcceptable {
			probe |= 1 << mask
		}
	}
	require(good == s.Acceptable && probe == s.ProbeAcceptable, "accepted masks differ")
}
func auditSearch(s r.Source, row r.Search, report r.Report) {
	wantCalls := 0
	var scores [2][2]float32
	inputSHA := ""
	relationRows, ruleChecks := 0, 0
	switch row.Mode {
	case "fixed":
	case "rule":
		view := s.Relations[0]
		inputSHA = r.Hash(r.Encode(view))
		relationRows = view.Count
		for option := range 2 {
			for relation, goals := range view.Buckets {
				if goals[0]+goals[1] == 0 {
					continue
				}
				ruleChecks++
				effective := relation
				if view.Features[6+option] == 1 {
					effective = 2 - relation
				}
				truth := (view.Features[1] == 1 && effective < 1) || (view.Features[2] == 1 && effective <= 1) || (view.Features[3] == 1 && effective == 1)
				goal := 0
				if truth {
					goal = 1
				}
				scores[0][option] += float32(goals[goal])
			}
		}
	case "baseline", "relative":
		wantCalls = 1
		inputs := s.CanonicalInputs
		if row.Mode == "relative" {
			inputs = s.RelativeInputs
			for _, view := range s.Relations {
				relationRows += view.Count
			}
		}
		inputSHA = r.Hash(r.Encode(map[string]any{"source": inputs, "outputs": s.OutputRows, "conditions": s.ConditionRows}))
	default:
		panic("unknown policy")
	}
	require(len(row.Predictions) == wantCalls && row.InputSHA == inputSHA && row.RelationRows == relationRows && row.RuleBucketChecks == ruleChecks, "ranking inputs or work differs")
	if wantCalls == 1 {
		p := row.Predictions[0]
		require(p.Model == row.Mode && p.Fingerprint == report.Models[row.Mode].Fingerprint && p.NS == row.PredictionNS && p.NS > 0, "model observation differs")
		scores = p.Logits
	} else {
		require(row.PredictionNS == 0, "unrecorded model call")
	}
	require(row.Logits == scores && row.Order == order(scores), "ranking differs")
	require(row.NS > 0 && row.ProbeNS > 0 && row.PreparationNS >= 0 && row.RuleNS >= 0 && row.NS >= row.PreparationNS+row.PredictionNS+row.RuleNS, "timing bounds differ")
	require((row.Mode == "fixed") == (row.PreparationNS == 0) && (row.Mode == "rule") == (row.RuleNS > 0), "policy preparation/rule timing differs")
	require(len(row.Attempts) > 0 && len(row.Attempts) <= 4, "attempt budget differs")
	bestScore, bestTotal, bestMask := -1, 0, uint16(0)
	for i, a := range row.Attempts {
		require(a.Mask == row.Order[i], "attempt order differs")
		original := s.Candidates[a.Mask]
		require(a.Gooo == original.Gooo && a.Go == original.Go && reflect.DeepEqual(a.Declared, original.Declared), "attempt body or observation differs")
		require(a.Acceptable == observed(a.Declared) && (!a.Acceptable || i == len(row.Attempts)-1), "continued after completion")
		score, total := r.Score(a.Declared)
		if score > bestScore {
			bestScore, bestTotal, bestMask = score, total, a.Mask
		}
	}
	last := row.Attempts[len(row.Attempts)-1]
	require((row.Selected != nil) == last.Acceptable, "selection differs")
	state := "PARTIAL"
	if row.Selected != nil {
		state = "COMPLETE"
		require(*row.Selected == last.Mask, "selected mask differs")
	} else {
		require(len(row.Attempts) == 4, "partial stopped early")
	}
	require(row.ResultState == state && row.ResultMask == bestMask && row.ResultPassed == bestScore && row.ResultTotal == bestTotal, "best partial result differs")
	require(reflect.DeepEqual(row.Probe, s.Candidates[bestMask].Probe), "probe influenced result selection")
	observed(row.Probe)
}
func aggregate(groups map[string]r.Group, s r.Source, row r.Search) {
	first := row.Attempts[0]
	valid := bits.OnesCount64(s.Acceptable)
	for _, key := range []string{row.Mode + "/all", row.Mode + "/" + row.Split} {
		g := groups[key]
		g.Documents++
		if first.Acceptable {
			g.FirstDeclared++
		}
		if s.Candidates[first.Mask].ProbeAcceptable {
			g.FirstProbe++
		}
		if row.Selected != nil {
			g.FinalDeclared++
		} else {
			g.PartialDocuments++
		}
		if observed(row.Probe) {
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
			target := uint16(bits.TrailingZeros64(s.Acceptable))
			if first.Mask&1 == target&1 {
				g.ComparisonCorrect++
			}
			if first.Mask>>1&1 == target>>1&1 {
				g.BranchCorrect++
			}
		}
		g.CandidateChecks += len(row.Attempts)
		g.ModelCalls += len(row.Predictions)
		g.ResultPassed += row.ResultPassed
		g.ResultTotal += row.ResultTotal
		g.RuleBucketChecks += row.RuleBucketChecks
		g.RelationRows += row.RelationRows
		g.SearchNS += row.NS
		g.PreparationNS += row.PreparationNS
		g.PredictionNS += row.PredictionNS
		g.RuleNS += row.RuleNS
		g.ProbeNS += row.ProbeNS
		groups[key] = g
	}
}
