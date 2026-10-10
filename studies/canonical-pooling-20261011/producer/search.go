package main

import (
	"context"
	"errors"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-pooling-20261011/record"
)

func search(ctx context.Context, x r.Source, p *pathplan.PreparedPlan, mode string,
	model *contractdecision.CanonicalInteractionRequirementModel) r.Search {
	stage := r.Stage{Gooo: x.Gooo, SourceSHA: x.SourceSHA, PlanSHA: x.PlanSHA, CasesSHA: x.CasesSHA, Document: x.Document}
	start := time.Now()
	var session *pathplan.ContractSession
	var err error
	if mode == "fixed" {
		session, err = p.NewContractSession(ctx, nil, x.Document.TestCases)
	} else {
		session, err = p.NewCanonicalInteractionRequirementContractSession(ctx, model, x.Document.TestCases)
	}
	must(err)
	stage.Ranking = session.Ranking()
	initial, err := session.Observe()
	must(err)
	require(initial.Attempted == 0, "ranking must precede candidate execution")
	require((mode == "fixed" && stage.Ranking.Calls == 0) || (mode != "fixed" && stage.Ranking.Calls == 1 && stage.Ranking.Applied), "one prediction per model session")
	stage.Progress = append(stage.Progress, initial)
	var selected *bodyplan.Program
	for range 4 {
		progress, body, err := session.Advance(ctx, 1)
		if err != nil && !errors.Is(err, pathplan.ErrNoConditionCandidate) && !errors.Is(err, pathplan.ErrNoTypedCandidate) {
			panic(err)
		}
		require(len(progress.NewAttempts) == 1 && progress.PredictionsThisAdvance == 0, "one candidate without reinference")
		stage.Progress = append(stage.Progress, progress)
		if body != nil {
			selected = body
		}
		if progress.Status == "TRAINING_COMPLETE" || progress.Exhausted {
			break
		}
	}
	stage.NS = time.Since(start).Nanoseconds()
	last := stage.Progress[len(stage.Progress)-1]
	if selected != nil {
		stage.Probe = observe(ctx, p, selected, last.Selection.Choices, x.Probes, x.ProbeConditions)
	}
	return r.Search{ID: x.Spec.ID, Mode: mode, Split: x.Spec.Split, Family: x.Spec.Family, Form: x.Spec.Form, Stages: []r.Stage{stage}}
}

func summarize(report *r.Report, source r.Source, row r.Search) {
	stage := row.Stages[0]
	first := stage.Progress[1].NewAttempts[0]
	last := stage.Progress[len(stage.Progress)-1]
	for _, key := range []string{row.Mode + "/all", row.Mode + "/" + row.Split, row.Mode + "/" + row.Split + "/" + row.Form} {
		g := report.Groups[key]
		g.Programs++
		if first.Passed == first.Total && pathplan.ConditionsPassed(first.Conditions) {
			g.FirstDeclaredComplete++
		}
		if r.Passed(source.Candidates[first.Mask].Probe) {
			g.FirstProbeComplete++
		}
		if last.Status == "TRAINING_COMPLETE" {
			g.InitialDeclaredComplete++
		}
		if r.Passed(stage.Probe) {
			g.InitialProbeComplete++
		}
		g.InitialAttempts += last.Attempted
		g.TotalAttempts += last.Attempted
		g.ModelCalls += stage.Ranking.Calls
		g.SearchNS += stage.NS
		g.PredictNS += stage.Ranking.PredictNS
		report.Groups[key] = g
	}
	report.InitialSearches++
	report.ModelCalls += stage.Ranking.Calls
}
