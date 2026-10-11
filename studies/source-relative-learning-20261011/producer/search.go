package main

import (
	"context"
	"reflect"
	"sort"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/record"
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

func rank(x r.Source, p *pathplan.PreparedPlan, row *r.Search, models map[string]predictor) {
	before := time.Now()
	switch row.Mode {
	case "fixed":
		return
	case "rule":
		input, err := p.InitialContractInput(x.Document.TestCases)
		must(err)
		view, err := input.SourceRelativeConditions("comparison")
		must(err)
		row.RelationRows = view.Count
		row.InputSHA = r.Hash(r.Encode(view))
		row.PreparationNS = time.Since(before).Nanoseconds()
		before = time.Now()
		row.Logits, row.RuleBucketChecks = relationRule(view)
		row.RuleNS = time.Since(before).Nanoseconds()
	case "baseline", "relative":
		rows := prepareRows(p, x.Document, row.Mode == "relative")
		require(reflect.DeepEqual(rows.canonical, x.CanonicalInputs) && reflect.DeepEqual(rows.outputs, x.OutputRows) &&
			reflect.DeepEqual(rows.conditions, x.ConditionRows), "fresh base inputs differ from source record")
		input := rows.canonical
		if row.Mode == "relative" {
			require(reflect.DeepEqual(rows.relative, x.RelativeInputs) && reflect.DeepEqual(rows.relations, x.Relations), "fresh relation inputs differ")
			input = rows.relative
		}
		row.RelationRows = rows.relationRows
		row.InputSHA = r.Hash(r.Encode(map[string]any{"source": input, "outputs": rows.outputs, "conditions": rows.conditions}))
		row.PreparationNS = time.Since(before).Nanoseconds()
		model := models[row.Mode]
		require(model != nil, "missing model")
		var work contractdecision.InteractionRequirementWorkspace
		var prediction contractdecision.ChoicePrediction
		before = time.Now()
		must(model.PredictChoicesInto(input, r.Rows(rows.outputs), r.Rows(rows.conditions), &work, &prediction))
		row.PredictionNS = time.Since(before).Nanoseconds()
		require(prediction.Count == 2, "two source choices required")
		row.Logits = [2][2]float32{prediction.Logits[0], prediction.Logits[1]}
		row.Predictions = []r.Prediction{{Model: row.Mode, Fingerprint: model.Fingerprint(), Logits: row.Logits, NS: row.PredictionNS}}
	default:
		panic("unknown study mode")
	}
}

// All policies share this bounded frontier and retain their best partial body
// when no candidate completes the authored requirements. Probe observations are
// collected afterward, never used to choose that body or update the model.
func search(ctx context.Context, x r.Source, p *pathplan.PreparedPlan, mode string,
	models map[string]predictor, event func(any)) r.Search {
	row := r.Search{ID: x.Spec.ID, Mode: mode, Split: x.Spec.Split, Family: x.Spec.Family, Form: x.Spec.Form}
	start := time.Now()
	must(ctx.Err())
	rank(x, p, &row, models)
	row.Order = ordered(row.Logits)
	event(map[string]any{"kind": "ranked", "id": row.ID, "mode": mode, "input_sha": row.InputSHA,
		"predictions": row.Predictions, "logits": row.Logits, "order": row.Order,
		"preparation_ns": row.PreparationNS, "rule_ns": row.RuleNS, "rule_bucket_checks": row.RuleBucketChecks, "relation_rows": row.RelationRows})
	var best *bodyplan.Program
	var bestChoices map[string]string
	bestScore := -1
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
		passed, total := r.Score(candidate.Declared)
		if passed > bestScore {
			best, bestChoices, bestScore = body, selected, passed
			row.ResultMask, row.ResultPassed, row.ResultTotal = mask, passed, total
		}
		if candidate.Acceptable {
			row.Selected = &mask
			break
		}
	}
	require(best != nil, "missing result body")
	row.ResultState = "PARTIAL"
	if row.Selected != nil {
		row.ResultState = "COMPLETE"
	}
	row.NS = time.Since(start).Nanoseconds()
	before := time.Now()
	row.Probe = observe(ctx, p, best, bestChoices, x.Probes, x.ProbeConditions)
	row.ProbeNS = time.Since(before).Nanoseconds()
	event(map[string]any{"kind": "probe", "id": row.ID, "mode": mode, "mask": row.ResultMask,
		"result_state": row.ResultState, "observations": row.Probe})
	return row
}
