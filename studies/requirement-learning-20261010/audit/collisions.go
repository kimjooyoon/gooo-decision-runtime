package main

import (
	"reflect"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/requirement-learning-20261010/record"
)

type InputCollision struct {
	Family                       string
	IDs, Bodies                  []string
	Acceptable                   []uint64
	SourceFeatureSHA             [2]string
	CaseSHA, ConditionFeatureSHA string
}

func inputCollisions(sources []r.Source, report r.Report) []InputCollision {
	byID := map[string]r.Source{}
	for _, x := range sources {
		byID[x.Spec.ID] = x
	}
	var result []InputCollision
	for _, g := range report.InputAudits["requirements"].Groups {
		if g.UnavoidableMisses == 0 {
			continue
		}
		need(len(g.SampleIndices) == 2 && g.BestFirstPasses == 1 && g.CommonAcceptable == 0, "original conflicting pair scope")
		a, b := byID[report.AuditedIDs[g.SampleIndices[0]]], byID[report.AuditedIDs[g.SampleIndices[1]]]
		need(a.Spec.Family == b.Spec.Family && a.Spec.Reverse != b.Spec.Reverse && a.Spec.OutputGoal == b.Spec.OutputGoal && a.Spec.ConditionGoal == b.Spec.ConditionGoal, "conflict is a reversed-body pair")
		need(reflect.DeepEqual(a.Inputs, b.Inputs) && reflect.DeepEqual(a.CaseRows, b.CaseRows) && reflect.DeepEqual(a.ConditionRows, b.ConditionRows), "all actual model channels identical")
		need(a.Acceptable&b.Acceptable == 0 && r.Body(a.Spec) != r.Body(b.Spec), "different source paths require different masks")
		result = append(result, InputCollision{Family: a.Spec.Family, IDs: []string{a.Spec.ID, b.Spec.ID},
			Bodies: []string{r.Body(a.Spec), r.Body(b.Spec)}, Acceptable: []uint64{a.Acceptable, b.Acceptable},
			SourceFeatureSHA: a.InputSHA, CaseSHA: a.CaseSHA, ConditionFeatureSHA: conditionSHA(a.ConditionRows)})
	}
	return result
}
