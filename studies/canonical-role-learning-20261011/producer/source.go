package main

import (
	"context"
	"math/bits"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-role-learning-20261011/record"
	"github.com/kimjooyoon/meta-ontology-go/internal/bodycodegen"
)

func lower(ctx context.Context, id, source string) (pathplan.Document, *pathplan.PreparedPlan) {
	d, err := bodycodegen.DecodeSourcePathDocument(ctx, id+".gooo", []byte(source), "Choose", nil)
	must(err)
	p, err := d.Prepare()
	must(err)
	return d, p
}

func choices(d pathplan.Document, mask uint16) map[string]string {
	result := map[string]string{}
	for i, c := range d.Plan.Decisions {
		result[c.ID] = c.Options[mask>>i&1].Label
	}
	return result
}

func observe(ctx context.Context, p *pathplan.PreparedPlan, body *bodyplan.Program, selected map[string]string,
	cases []pathplan.TestCase, conditions []pathplan.ConditionCase) r.Observations {
	var result r.Observations
	for _, test := range cases {
		v, err := body.Evaluate(test.Input)
		must(err)
		result.Outputs = append(result.Outputs, pathplan.TestResult{Input: test.Input, Expected: test.Expected,
			Actual: v.Int, Passed: v.Int == test.Expected})
	}
	var err error
	result.Conditions, err = p.CheckConditions(ctx, selected, conditions)
	must(err)
	return result
}

func collect(ctx context.Context, frozen r.Frozen) (r.Source, *pathplan.PreparedPlan) {
	d, p := lower(ctx, frozen.Spec.ID, frozen.Gooo)
	input, err := p.InitialContractInput(d.TestCases)
	must(err)
	require(len(d.Plan.Decisions) == 2 && d.Plan.Decisions[0].ID == "comparison" && d.Plan.Decisions[1].ID == "branches", "source choice order")
	require(string(d.Plan.Decisions[0].Kind) == "operand_order" && string(d.Plan.Decisions[1].Kind) == "branch_layout", "source decision roles")
	x := r.Source{Frozen: frozen, SourceSHA: r.Hash([]byte(frozen.Gooo)), PlanSHA: p.PlanSHA256(), CasesSHA: input.CaseSHA256(), Document: d}
	for _, c := range d.Plan.Decisions {
		var canonical [528]float32
		must(input.CanonicalOrderedSourceFeaturesInto(c.ID, &canonical))
		x.CanonicalInputs = append(x.CanonicalInputs, canonical)
	}
	for i := range input.CaseCount() {
		row, err := input.CaseFeatures(i)
		must(err)
		x.OutputRows = append(x.OutputRows, row)
	}
	for i := range input.ConditionCount() {
		row, err := input.ConditionFeatures(i)
		must(err)
		x.ConditionRows = append(x.ConditionRows, row)
	}
	for mask := range 4 {
		selected := choices(d, uint16(mask))
		body, err := p.Compile(selected)
		must(err)
		c := r.Candidate{Mask: uint16(mask), Gooo: body.GoooSource(), Go: body.GoSource()}
		c.Declared = observe(ctx, p, body, selected, d.TestCases, d.Plan.ConditionCases)
		c.Probe = observe(ctx, p, body, selected, frozen.Probes, frozen.ProbeConditions)
		c.Acceptable, c.ProbeAcceptable = r.Passed(c.Declared), r.Passed(c.Probe)
		if c.Acceptable {
			x.Acceptable |= 1 << mask
		}
		if c.ProbeAcceptable {
			x.ProbeAcceptable |= 1 << mask
		}
		x.Candidates = append(x.Candidates, c)
	}
	require(bits.OnesCount64(x.Acceptable) == 1 && x.Acceptable == x.ProbeAcceptable, "one complete mask must satisfy declared and probe requirements")
	return x, p
}
