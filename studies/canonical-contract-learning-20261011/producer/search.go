package main

import (
	"context"
	"errors"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
)

type models struct {
	legacy    *contractdecision.InteractionRequirementModel
	canonical *contractdecision.CanonicalInteractionRequirementModel
}

func (m models) session(ctx context.Context, p *pathplan.PreparedPlan, cases []pathplan.TestCase, mode string) (*pathplan.ContractSession, error) {
	switch mode {
	case "legacy":
		return p.NewInteractionRequirementContractSession(ctx, m.legacy, cases)
	case "canonical":
		return p.NewCanonicalInteractionRequirementContractSession(ctx, m.canonical, cases)
	case "fixed":
		return p.NewContractSession(ctx, nil, cases)
	default:
		panic("unknown policy")
	}
}

func searchStage(ctx context.Context, frozen r.Frozen, d pathplan.Document, p *pathplan.PreparedPlan, source, mode string, number int, m models) r.Stage {
	input, err := p.InitialContractInput(d.TestCases)
	must(err)
	row := r.Stage{Number: number, Gooo: source, SourceSHA: r.Hash([]byte(source)), PlanSHA: p.PlanSHA256(), CasesSHA: input.CaseSHA256(), Document: d}
	start := time.Now()
	s, err := m.session(ctx, p, d.TestCases, mode)
	must(err)
	row.Ranking = s.Ranking()
	initial, err := s.Observe()
	must(err)
	require(initial.Attempted == 0, "prediction must precede candidate execution")
	row.Progress = append(row.Progress, initial)
	var selected *bodyplan.Program
	for range 4 {
		progress, body, err := s.Advance(ctx, 1)
		if err != nil && !errors.Is(err, pathplan.ErrNoConditionCandidate) && !errors.Is(err, pathplan.ErrNoTypedCandidate) {
			panic(err)
		}
		require(len(progress.NewAttempts) == 1 && progress.PredictionsThisAdvance == 0, "one candidate without reinference")
		row.Progress = append(row.Progress, progress)
		if body != nil {
			selected = body
		}
		if progress.Status == "TRAINING_COMPLETE" || progress.Exhausted {
			break
		}
	}
	row.NS = time.Since(start).Nanoseconds()
	last := row.Progress[len(row.Progress)-1]
	if selected != nil {
		row.Probe = observe(ctx, p, selected, last.Selection.Choices, frozen.Probes, frozen.ProbeConditions)
	}
	return row
}

func search(ctx context.Context, x r.Source, p *pathplan.PreparedPlan, mode string, m models) r.Search {
	row := r.Search{ID: x.Spec.ID, Mode: mode, Split: x.Spec.Split, Family: x.Spec.Family, Form: x.Spec.Form}
	d, source := x.Document, x.Gooo
	for number := range 3 {
		stage := searchStage(ctx, x.Frozen, d, p, source, mode, number, m)
		if x.Spec.Split != "sparse" || r.Passed(stage.Probe) || number == 2 {
			row.Stages = append(row.Stages, stage)
			break
		}
		// One new requirement per stage, in a fixed order chosen before fitting.
		// Probe expectations never enter training or the preceding prediction.
		for _, result := range stage.Probe.Outputs {
			if !result.Passed {
				c := pathplan.TestCase{Input: result.Input, Expected: result.Expected}
				stage.NextOutput = &c
				d.TestCases = append(append([]pathplan.TestCase{}, d.TestCases...), c)
				break
			}
		}
		if stage.NextOutput == nil {
			for _, result := range stage.Probe.Conditions {
				if !result.Passed {
					c := result.Case
					stage.NextCondition = &c
					d.Plan.ConditionCases = append(append([]pathplan.ConditionCase{}, d.Plan.ConditionCases...), c)
					break
				}
			}
		}
		row.Stages = append(row.Stages, stage)
		if stage.NextOutput == nil && stage.NextCondition == nil {
			break
		}
		source = r.GoooWithCases(x.Spec, d.TestCases, d.Plan.ConditionCases)
		d, p = lower(ctx, x.Spec.ID, source)
	}
	return row
}

func summarize(report *r.Report, source r.Source, row r.Search) {
	initial, final := row.Stages[0], row.Stages[len(row.Stages)-1]
	first := initial.Progress[1].NewAttempts[0]
	initialLast, finalLast := initial.Progress[len(initial.Progress)-1], final.Progress[len(final.Progress)-1]
	firstProbe := source.Candidates[first.Mask].Probe
	keys := []string{row.Mode + "/all", row.Mode + "/" + row.Split, row.Mode + "/" + row.Split + "/" + row.Form, row.Mode + "/family-" + row.Family}
	for _, key := range keys {
		g := report.Groups[key]
		g.Programs++
		if first.Passed == first.Total && pathplan.ConditionsPassed(first.Conditions) {
			g.FirstDeclaredComplete++
		}
		if r.Passed(firstProbe) {
			g.FirstProbeComplete++
		}
		if initialLast.Status == "TRAINING_COMPLETE" {
			g.InitialDeclaredComplete++
		}
		if r.Passed(initial.Probe) {
			g.InitialProbeComplete++
		}
		if finalLast.Status == "TRAINING_COMPLETE" {
			g.FinalDeclaredComplete++
		}
		if r.Passed(final.Probe) {
			g.FinalProbeComplete++
		}
		g.FirstOutputPassed += first.Passed
		g.FirstOutputTotal += first.Total
		g.FirstConditionTotal += len(first.Conditions)
		for _, c := range first.Conditions {
			if c.Passed {
				g.FirstConditionPassed++
			}
		}
		g.FirstProbeOutputTotal += len(firstProbe.Outputs)
		for _, c := range firstProbe.Outputs {
			if c.Passed {
				g.FirstProbeOutputPassed++
			}
		}
		g.FirstProbeConditionTotal += len(firstProbe.Conditions)
		for _, c := range firstProbe.Conditions {
			if c.Passed {
				g.FirstProbeConditionPassed++
			}
		}
		g.InitialAttempts += initialLast.Attempted
		g.FeedbackStages += len(row.Stages) - 1
		for _, stage := range row.Stages {
			g.TotalAttempts += stage.Progress[len(stage.Progress)-1].Attempted
			g.ModelCalls += stage.Ranking.Calls
			g.SearchNS += stage.NS
			g.PredictNS += stage.Ranking.PredictNS
		}
		report.Groups[key] = g
	}
	report.InitialSearches++
	report.FeedbackSearches += len(row.Stages) - 1
	for _, stage := range row.Stages {
		report.ModelCalls += stage.Ranking.Calls
	}
}
