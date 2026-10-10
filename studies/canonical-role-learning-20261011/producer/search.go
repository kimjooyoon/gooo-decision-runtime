package main

import (
	"context"
	"math/bits"
	"sort"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-role-learning-20261011/record"
)

type predictor interface {
	Fingerprint() string
	PredictChoicesInto([][528]float32, contractdecision.CaseSource, contractdecision.ConditionSource,
		*contractdecision.InteractionRequirementWorkspace, *contractdecision.ChoicePrediction) error
}

func ordered(logits [2][2]float32) [4]uint16 {
	order := [4]uint16{0, 1, 2, 3}
	score := func(mask uint16) float64 { return float64(logits[0][mask&1]) + float64(logits[1][mask>>1&1]) }
	sort.SliceStable(order[:], func(i, j int) bool { return score(order[i]) > score(order[j]) })
	return order
}

// All three policies use this same four-mask order/compile/check loop. This is
// an experimental exhaustive ranker, separate from the SDK session frontier.
func search(ctx context.Context, x r.Source, p *pathplan.PreparedPlan, mode string,
	models map[string]predictor, event func(any)) r.Search {
	row := r.Search{ID: x.Spec.ID, Mode: mode, Split: x.Spec.Split, Family: x.Spec.Family, Form: x.Spec.Form}
	start := time.Now()
	names := []string{}
	switch mode {
	case "fixed":
	case "joint":
		names = []string{"joint"}
	case "split":
		names = []string{"comparison", "branches"}
	default:
		panic("unknown study mode")
	}
	for _, name := range names {
		must(ctx.Err())
		model := models[name]
		require(model != nil, "missing model")
		var work contractdecision.InteractionRequirementWorkspace
		var prediction contractdecision.ChoicePrediction
		before := time.Now()
		must(model.PredictChoicesInto(x.CanonicalInputs, r.Rows(x.OutputRows), r.Rows(x.ConditionRows), &work, &prediction))
		ns := time.Since(before).Nanoseconds()
		require(prediction.Count == 2, "two source choices required")
		scores := [2][2]float32{prediction.Logits[0], prediction.Logits[1]}
		row.Predictions = append(row.Predictions, r.Prediction{Model: name, Fingerprint: model.Fingerprint(), Logits: scores, NS: ns})
		switch name {
		case "joint":
			row.Logits = scores
		case "comparison":
			row.Logits[0] = scores[0]
		case "branches":
			row.Logits[1] = scores[1]
		}
	}
	row.Order = ordered(row.Logits)
	// Persist the initial predictions before the first body is compiled/executed.
	event(map[string]any{"kind": "ranked", "id": row.ID, "mode": mode, "predictions": row.Predictions, "logits": row.Logits, "order": row.Order})
	for _, mask := range row.Order {
		must(ctx.Err())
		selected := choices(x.Document, mask)
		body, err := p.Compile(selected)
		must(err)
		candidate := r.Candidate{Mask: mask, Gooo: body.GoooSource(), Go: body.GoSource()}
		candidate.Declared = observe(ctx, p, body, selected, x.Document.TestCases, x.Document.Plan.ConditionCases)
		candidate.Acceptable = r.Passed(candidate.Declared)
		row.Attempts = append(row.Attempts, candidate)
		event(map[string]any{"kind": "attempt", "id": row.ID, "mode": mode, "candidate": candidate})
		if candidate.Acceptable {
			row.Selected = &mask
			row.NS = time.Since(start).Nanoseconds()
			row.Probe = observe(ctx, p, body, selected, x.Probes, x.ProbeConditions)
			event(map[string]any{"kind": "probe", "id": row.ID, "mode": mode, "mask": mask, "observations": row.Probe})
			break
		}
	}
	if row.Selected == nil {
		row.NS = time.Since(start).Nanoseconds()
	}
	return row
}

func summarize(report *r.Report, x r.Source, row r.Search) {
	require(len(row.Attempts) > 0, "missing attempt")
	first := row.Attempts[0]
	accepted := uint16(bits.TrailingZeros64(x.Acceptable))
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
		if r.Passed(row.Probe) {
			g.FinalProbe++
		}
		if first.Mask&1 == accepted&1 {
			g.ComparisonCorrect++
		}
		if first.Mask>>1&1 == accepted>>1&1 {
			g.BranchCorrect++
		}
		g.CandidateChecks += len(row.Attempts)
		g.ModelCalls += len(row.Predictions)
		g.SearchNS += row.NS
		for _, prediction := range row.Predictions {
			g.PredictionNS += prediction.NS
		}
		report.Groups[key] = g
	}
	report.Searches++
	report.ModelCalls += len(row.Predictions)
}
