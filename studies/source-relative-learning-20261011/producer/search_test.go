package main

import (
	"context"
	"math/bits"
	"reflect"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	previous "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/record"
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
func TestPoliciesPredictBeforeCandidatesAndKeepPartial(t *testing.T) {
	ctx := context.Background()
	for _, diagnostic := range []string{"", "missing_conditions", "unsatisfiable_conditions"} {
		spec := r.Spec{Spec: previous.Spec{ID: "unit-relative-only", Family: "bound", Form: "assignment", Split: "unit", K: 5, Reverse: 1}, Diagnostic: diagnostic}
		x, p := collect(ctx, r.Freeze(spec))
		wantValid := map[string]int{"": 1, "missing_conditions": 2, "unsatisfiable_conditions": 0}[diagnostic]
		if bits.OnesCount64(x.Acceptable) != wantValid {
			t.Fatal(diagnostic, x.Acceptable)
		}
		for _, mode := range []string{"fixed", "rule", "baseline", "relative"} {
			model := &fakeModel{Scores: [2][2]float32{{0, 2}, {0, 1}}}
			models := map[string]predictor{"baseline": model, "relative": model}
			wantCalls := 0
			if mode == "baseline" || mode == "relative" {
				wantCalls = 1
			}
			events := 0
			row := search(ctx, x, p, mode, models, func(value any) {
				event := value.(map[string]any)
				if model.Calls != wantCalls {
					t.Fatal("prediction must precede first event and not repeat", mode, event)
				}
				if events == 0 && event["kind"] != "ranked" {
					t.Fatal("first event must preserve ranking")
				}
				events++
			})
			if len(row.Predictions) != wantCalls || len(row.Attempts) > 4 || events != len(row.Attempts)+2 {
				t.Fatal(mode, row)
			}
			bestScore, bestMask := -1, uint16(0)
			for _, a := range row.Attempts {
				score, _ := r.Score(a.Declared)
				if score > bestScore {
					bestScore, bestMask = score, a.Mask
				}
			}
			if row.ResultPassed != bestScore || row.ResultMask != bestMask || !reflect.DeepEqual(row.Probe, x.Candidates[bestMask].Probe) {
				t.Fatal("partial selection used probe or lost observations")
			}
			if wantValid == 0 {
				if row.Selected != nil || row.ResultState != "PARTIAL" || len(row.Attempts) != 4 || row.ResultPassed >= row.ResultTotal {
					t.Fatal("unsatisfiable disappeared", row)
				}
			} else if row.Selected == nil || row.ResultState != "COMPLETE" || row.ResultPassed != row.ResultTotal {
				t.Fatal("complete result missing", row)
			}
			report := r.Report{Groups: map[string]r.Group{}}
			summarize(&report, x, row)
			group := report.Groups[mode+"/all"]
			if group.Documents != 1 || group.ModelCalls != wantCalls || group.ResultPassed != row.ResultPassed {
				t.Fatal(group)
			}
		}
	}
}
func TestRelationRuleAndStableFrontier(t *testing.T) {
	view := pathplan.SourceRelativeConditionContext{Available: true, Count: 3}
	view.Features[1], view.Features[4], view.Features[7] = 1, 1, 1
	view.Buckets = [3][2]uint16{{0, 1}, {1, 0}, {1, 0}}
	scores, checks := relationRule(view)
	if scores != ([2][2]float32{{3, 1}, {0, 0}}) || checks != 6 {
		t.Fatal(scores, checks)
	}
	view.Features[6], view.Features[7] = 1, 0
	reversed, n := relationRule(view)
	if reversed != ([2][2]float32{{1, 3}, {0, 0}}) || n != 6 {
		t.Fatal(reversed, n)
	}
	if ordered([2][2]float32{}) != ([4]uint16{0, 1, 2, 3}) || ordered([2][2]float32{{0, 4}, {0, 2}}) != ([4]uint16{3, 1, 2, 0}) {
		t.Fatal("incomplete or unstable order")
	}
}
