package main

import (
	"context"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-role-learning-20261011/record"
	"testing"
)

type fakeModel struct {
	Calls  int
	Scores [2][2]float32
}

func (m *fakeModel) Fingerprint() string { return "unit-fixture" }
func (m *fakeModel) PredictChoicesInto(_ [][528]float32, _ contractdecision.CaseSource, _ contractdecision.ConditionSource, _ *contractdecision.InteractionRequirementWorkspace, out *contractdecision.ChoicePrediction) error {
	m.Calls++
	out.Count = 2
	out.Logits[0], out.Logits[1] = m.Scores[0], m.Scores[1]
	return nil
}
func TestRolePolicyPredictsBeforeFirstCandidate(t *testing.T) {
	ctx := context.Background()
	x, p := collect(ctx, r.Freeze(r.Spec{ID: "unit-role-only", Family: "distance", Form: "assignment", Split: "unit", K: 5, Reverse: 1, ConditionGoal: 1}))
	for _, mode := range []string{"fixed", "joint", "split"} {
		cmp := &fakeModel{Scores: [2][2]float32{{0, 2}, {9, 0}}}
		branch := &fakeModel{Scores: [2][2]float32{{9, 0}, {0, 1}}}
		joint := &fakeModel{Scores: [2][2]float32{{0, 2}, {0, 1}}}
		models := map[string]predictor{"joint": joint, "comparison": cmp, "branches": branch}
		wantCalls := map[string]int{"fixed": 0, "joint": 1, "split": 2}[mode]
		events := 0
		row := search(ctx, x, p, mode, models, func(value any) {
			event := value.(map[string]any)
			if cmp.Calls+branch.Calls+joint.Calls != wantCalls {
				t.Fatal("model calls did not precede candidates", event)
			}
			if events == 0 && event["kind"] != "ranked" {
				t.Fatal("first event must retain ranking", event)
			}
			events++
		})
		if len(row.Predictions) != wantCalls || row.Selected == nil || !r.Passed(row.Probe) || len(row.Attempts) > 4 {
			t.Fatal(mode, row)
		}
		if mode != "fixed" && (row.Order[0] != 3 || row.Logits != [2][2]float32{{0, 2}, {0, 1}}) {
			t.Fatal("selected another role's scores", mode, row.Logits, row.Order)
		}
		report := r.Report{Groups: map[string]r.Group{}}
		summarize(&report, x, row)
		if g := report.Groups[mode+"/all"]; g.Documents != 1 || g.FinalDeclared != 1 || g.FinalProbe != 1 || g.ModelCalls != wantCalls {
			t.Fatal(g)
		}
	}
}
func TestOrderedMasksAreCompleteAndStable(t *testing.T) {
	for _, scores := range [][2][2]float32{{}, {{1, 1}, {2, 2}}, {{0, 4}, {0, 2}}} {
		order := ordered(scores)
		seen := map[uint16]bool{}
		for _, mask := range order {
			if mask > 3 || seen[mask] {
				t.Fatal(order)
			}
			seen[mask] = true
		}
	}
	if got := ordered([2][2]float32{}); got != [4]uint16{0, 1, 2, 3} {
		t.Fatal(got)
	}
	if got := ordered([2][2]float32{{0, 4}, {0, 2}}); got != [4]uint16{3, 1, 2, 0} {
		t.Fatal(got)
	}
}
