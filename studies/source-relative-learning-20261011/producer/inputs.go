package main

import (
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type inputRows struct {
	canonical, relative [][528]float32
	relations           []pathplan.SourceRelativeConditionContext
	outputs, conditions [][32]float32
	relationRows        int
}

// Every construction prepares its actual model inputs again. Source collection
// keeps independent copies for auditing/training; those copies do not remove
// preparation work from the construction timing.
func prepareRows(p *pathplan.PreparedPlan, d pathplan.Document, relative bool) inputRows {
	input, err := p.InitialContractInput(d.TestCases)
	must(err)
	var rows inputRows
	for _, choice := range d.Plan.Decisions {
		var source [528]float32
		must(input.CanonicalOrderedSourceFeaturesInto(choice.ID, &source))
		rows.canonical = append(rows.canonical, source)
		if relative {
			view, err := input.SourceRelativeConditions(choice.ID)
			must(err)
			require(view.Available, "source-relative context unavailable: "+view.Reason)
			rows.relations = append(rows.relations, view)
			rows.relationRows += view.Count
			copy(source[192:208], view.Features[:])
			rows.relative = append(rows.relative, source)
		}
	}
	for i := range input.CaseCount() {
		row, err := input.CaseFeatures(i)
		must(err)
		rows.outputs = append(rows.outputs, row)
	}
	for i := range input.ConditionCount() {
		row, err := input.ConditionFeatures(i)
		must(err)
		rows.conditions = append(rows.conditions, row)
	}
	return rows
}

// This small deterministic partial evaluator checks populated relation buckets
// for each comparator option. Preparation already counted the source rows.
// The counter reports actual operator checks on buckets, not neural calls or
// executions of complete candidate bodies. Branch-layout scores remain tied.
func relationRule(view pathplan.SourceRelativeConditionContext) ([2][2]float32, int) {
	require(view.Available && view.Features[4] == 1, "comparison context required")
	var scores [2][2]float32
	checks := 0
	for option := range 2 {
		for relation, goals := range view.Buckets {
			if goals[0]+goals[1] == 0 {
				continue
			}
			resolved := relation
			if view.Features[6+option] == 1 {
				resolved = 2 - relation
			}
			truth := resolved == 0
			if view.Features[2] == 1 {
				truth = resolved <= 1
			} else if view.Features[3] == 1 {
				truth = resolved == 1
			}
			checks++
			goal := 0
			if truth {
				goal = 1
			}
			scores[0][option] += float32(goals[goal])
		}
	}
	return scores, checks
}
