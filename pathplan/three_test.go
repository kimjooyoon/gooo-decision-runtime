package pathplan

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
)

func testThreeModel(t *testing.T) *jointdecision.ThreeModel {
	t.Helper()
	dir := t.TempDir()
	raw := make([]byte, 74624)
	for i := 0; i < 8; i++ {
		binary.LittleEndian.PutUint32(raw[len(raw)-32+i*4:], math.Float32bits(float32(7-i)))
	}
	meta := jointdecision.Metadata{Schema: jointdecision.ThreeSchema, Feature: jointdecision.ThreeFeatureVersion, Variant: "fp32", FeatureDim: 768, HiddenDim: 24, MaxBytes: 1600, Labels: []string{"mask_0", "mask_1", "mask_2", "mask_3", "mask_4", "mask_5", "mask_6", "mask_7"}, Temperature: 1, WeightsFile: "weights.bin", WeightsSHA: hash(raw)}
	var offset int64
	for i, name := range [4]string{"w1", "b1", "w2", "b2"} {
		rows, cols := [4]int{24, 1, 8, 1}[i], [4]int{768, 24, 24, 8}[i]
		count := rows * cols
		meta.Tensors = append(meta.Tensors, decision.TensorMetadata{Name: name, Rows: rows, Cols: cols, Count: count, Encoding: "float32_le", Offset: offset, Bytes: int64(count * 4), Scale: 1})
		offset += int64(count * 4)
	}
	encoded, e := json.Marshal(meta)
	if e != nil {
		t.Fatal(e)
	}
	name := filepath.Join(dir, "model.json")
	if e = os.WriteFile(name, encoded, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "weights.bin"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	m, e := jointdecision.LoadThree(name)
	if e != nil {
		t.Fatal(e)
	}
	return m
}

func threePlan() Plan {
	return Plan{Schema: Schema, Base: bodyplan.Plan{Schema: bodyplan.Schema, ID: "three-test", Name: "ThreeTest", ResultType: decision.TypeInt,
		Expressions: []bodyplan.Expr{{Kind: bodyplan.ExprInput, Name: "input"}, {Kind: bodyplan.ExprInt, Int: 1}, {Kind: bodyplan.ExprBinary, Operation: "subtract", Left: 0, Right: 1}, {Kind: bodyplan.ExprLocal, Name: "a"}, {Kind: bodyplan.ExprInt, Int: 3}, {Kind: bodyplan.ExprBinary, Operation: "subtract", Left: 3, Right: 4}, {Kind: bodyplan.ExprLocal, Name: "b"}, {Kind: bodyplan.ExprInt, Int: 9}, {Kind: bodyplan.ExprBinary, Operation: "subtract", Left: 6, Right: 7}},
		Statements:  []bodyplan.Stmt{{Kind: bodyplan.StmtLet, Name: "a", Expr: 2}, {Kind: bodyplan.StmtLet, Name: "b", Expr: 5}, {Kind: bodyplan.StmtReturn, Expr: 8}}, Root: []int{0, 1, 2}},
		Decisions: []Choice{{ID: "first", Kind: OperandOrder, Target: 2, Intent: "Subtract first.", Fallback: "layout_forward", Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}}, {ID: "second", Kind: OperandOrder, Target: 5, Intent: "Subtract second.", Fallback: "layout_forward", Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}}, {ID: "third", Kind: OperandOrder, Target: 8, Intent: "Subtract third.", Fallback: "layout_forward", Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}}}}
}
func threePrepared(t *testing.T, full bool) *PreparedPlan {
	t.Helper()
	plan := threePlan()
	base, e := Prepare(plan)
	if e != nil {
		t.Fatal(e)
	}
	for i, c := range plan.Decisions {
		fields, e := base.SourceFeatures(c.ID)
		if e != nil {
			t.Fatal(e)
		}
		intent := c.Intent
		if full {
			intent = strings.Repeat("x", 364)
		}
		plan.Decisions[i].Intent, e = decision.EncodeSemanticContextInput(fields, intent)
		if e != nil {
			t.Fatal(e)
		}
	}
	p, e := Prepare(plan)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func threeContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestThreeEightMasksGlobalRankingActualCallsAndFinalNoCall(t *testing.T) {
	p, m, ctx := threePrepared(t, false), testThreeModel(t), threeContext(t)
	s, e := p.NewThreeSession(ctx, m, []TestCase{{Input: 3, Expected: 999}}, "")
	if e != nil {
		t.Fatal(e)
	}
	initial, e := s.Observe()
	if e != nil {
		t.Fatal(e)
	}
	if initial.Selection.ModelCalls != 1 || initial.Selection.Three == nil || initial.Selection.Three.Calls != 1 || !initial.Selection.Three.PredictionValid || initial.Selection.Joint != nil || initial.Declared != 8 {
		t.Fatal("three initial prediction accounting")
	}
	for i := 0; i < 8; i++ {
		progress, _, e := s.Advance(ctx, 1)
		if e != nil || len(progress.NewAttempts) != 1 || progress.NewAttempts[0].Mask != uint16(i) {
			t.Fatal("eight complete probabilities lost ordering", i, progress.NewAttempts, e)
		}
		if i < 7 {
			r, e := s.ReconsiderThree(ctx, m, nil)
			if e != nil || r.CIIsAuthority || r.SHA == "" {
				t.Fatal(e)
			}
			calls := 1
			if i == 6 {
				calls = 0
				if !r.RankingUnnecessary {
					t.Fatal("sole remaining mask inferred")
				}
			}
			if r.ModelCalls != calls {
				t.Fatal("feedback actual calls", r.ModelCalls, calls)
			}
			if calls == 1 && (r.Three == nil || !r.Three.PredictionValid || !r.Applied) {
				t.Fatal("three feedback not recorded")
			}
		}
	}
	final, _ := s.Observe()
	if final.Selection.ModelCalls != 7 || final.Attempted != 8 || !final.Exhausted || final.TypeRejected != 0 {
		t.Fatal("eight-mask final totals", final)
	}
	if _, e = s.ReconsiderThree(ctx, m, nil); e == nil {
		t.Fatal("exhausted feedback accepted")
	}
}

func TestThreeBatchedActualFiniteFunctionAndDeterministicControls(t *testing.T) {
	p, m, ctx := threePrepared(t, false), testThreeModel(t), threeContext(t)
	// All three reversed operations compose as 9-(3-(1-input)) = 7-input.
	cases := []TestCase{{Input: 3, Expected: 4}, {Input: 4, Expected: 3}}
	r, body, progress, feedback, e := p.SearchThreeFeedbackBatches(ctx, m, cases, 8, 1, "", 7, nil)
	if e != nil || body == nil || r.Status != "TRAINING_COMPLETE" || len(r.Attempts) != 8 || r.Attempts[7].Mask != 7 || len(progress) != 16 || len(feedback) != 7 || r.Selection.ModelCalls != 7 {
		t.Fatal("three full finite construction", r, e)
	}
	for _, c := range cases {
		v, e := body.Evaluate(c.Input)
		if e != nil || v.Int != c.Expected {
			t.Fatal("selected actual function", e)
		}
	}
	a, e := p.NewThreeSession(ctx, m, cases, "replay-seed")
	if e != nil {
		t.Fatal(e)
	}
	b, e := p.NewThreeSession(ctx, m, cases, "replay-seed")
	if e != nil {
		t.Fatal(e)
	}
	pa, _ := a.Observe()
	pb, _ := b.Observe()
	if pa.Selection.Three.Sampled != pb.Selection.Three.Sampled || pa.Selection.Three.InputSHA != pb.Selection.Three.InputSHA || strings.Contains(pa.Selection.Three.Input, "replay-seed") {
		t.Fatal("sampling changed inputs or replay")
	}
	left, _, _, _, e := p.SearchThreeFeedbackBatches(ctx, nil, cases, 8, 1, "", 7, nil)
	if e != nil {
		t.Fatal(e)
	}
	right, _, _, _, e := p.SearchThreeFeedbackBatches(ctx, nil, cases, 8, 1, "", 7, nil)
	if e != nil || left.Selection.ModelCalls != 0 || right.Selection.ModelCalls != 0 || !reflect.DeepEqual(left.Attempts, right.Attempts) {
		t.Fatal("disconnected order is not deterministic", e)
	}
	unsupported := jointPrepared(t, false)
	u, e := unsupported.NewThreeSession(ctx, m, cases, "")
	if e != nil {
		t.Fatal(e)
	}
	off, e := unsupported.NewSession(ctx, nil, cases, "")
	if e != nil {
		t.Fatal(e)
	}
	ur, _ := u.Observe()
	if ur.Selection.ModelCalls != 0 || ur.Selection.Three == nil || !ur.Selection.Three.Declined || ur.Selection.Three.Decisions != 2 || ur.Selection.Three.PredictionValid {
		t.Fatal("unsupported arity inferred")
	}
	for _, c := range unsupported.plan.Decisions {
		if !strings.Contains(ur.Selection.Three.Input, c.Intent) {
			t.Fatal("unsupported original input was dropped")
		}
	}
	up, _, ue := u.Advance(ctx, 4)
	op, _, oe := off.Advance(ctx, 4)
	if !reflect.DeepEqual(up.NewAttempts, op.NewAttempts) || !errors.Is(ue, oe) {
		t.Fatal("unsupported arity changed deterministic candidates", ue, oe)
	}
}

func TestThreeOverflowOwnershipBusyCancellationAndWrongPins(t *testing.T) {
	p, m, ctx := threePrepared(t, true), testThreeModel(t), threeContext(t)
	cases := []TestCase{{Input: 3, Expected: 999}}
	s, e := p.NewThreeSession(ctx, m, cases, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Advance(ctx, 1); e != nil {
		t.Fatal(e)
	}
	queue := append(searchHeap(nil), s.queue...)
	weights := s.jointLogWeights
	r, e := s.ReconsiderThree(ctx, m, nil)
	if !errors.Is(e, ErrFeedbackContextBound) || !r.ContextDeclined || r.ModelCalls != 0 || r.SHA == "" || r.Three == nil || !r.Three.Declined || r.Three.PredictionValid || !reflect.DeepEqual(queue, s.queue) || weights != s.jointLogWeights {
		t.Fatal("overflow inferred, truncated or changed committed ranking", r, e)
	}
	for i, c := range p.plan.Decisions {
		if !strings.Contains(r.Three.Input, c.Intent) {
			t.Fatal("full overflow original input missing")
		}
		part := c.Intent + "\n" + s.jointFeedbackPrefix(r)
		if r.Three.PartSHA[i] != hash([]byte(part)) {
			t.Fatal("overflow lost complete per-part identity")
		}
	}
	state, _ := s.Observe()
	state.Selection.Three.Probabilities[0] = 999
	owned, _ := s.Observe()
	if owned.Selection.Three.Probabilities[0] == 999 {
		t.Fatal("borrowed three receipt mutated session")
	}
	s.lock.Lock()
	start := time.Now()
	_, e = s.ReconsiderThree(ctx, m, nil)
	if !errors.Is(e, ErrSessionBusy) || time.Since(start) > 100*time.Millisecond {
		t.Fatal("busy three feedback waits", e)
	}
	_, _, e = s.Advance(ctx, 1)
	if !errors.Is(e, ErrSessionBusy) {
		t.Fatal("busy advance waits", e)
	}
	s.lock.Unlock()
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.ReconsiderThree(canceled, m, nil); !errors.Is(e, context.Canceled) {
		t.Fatal("canceled feedback accepted")
	}
	if _, _, e = s.Advance(canceled, 1); !errors.Is(e, context.Canceled) {
		t.Fatal("canceled advance accepted")
	}
	if _, e = s.ReconsiderJoint(ctx, testJointModel(t), nil); e == nil {
		t.Fatal("v1 feedback accepted three model session")
	}
	p = threePrepared(t, false)
	s, e = p.NewThreeSession(ctx, m, cases, "")
	if e != nil {
		t.Fatal(e)
	}
	s.Advance(ctx, 1)
	other := testThreeModel(t)
	otherModelMeta := other.MetadataSHA256()
	if otherModelMeta != m.MetadataSHA256() {
		t.Fatal("controlled fixture differs")
	}
	s.result.Selection.MetadataSHA256 = "wrong"
	if _, e = s.ReconsiderThree(ctx, other, nil); e == nil {
		t.Fatal("different pinned model accepted")
	}
	invalid, e := Prepare(threePlan())
	if e != nil {
		t.Fatal(e)
	}
	declined, e := invalid.NewThreeSession(ctx, m, cases, "")
	if e != nil {
		t.Fatal(e)
	}
	d, _ := declined.Observe()
	if d.Selection.ModelCalls != 0 || d.Selection.Three == nil || !d.Selection.Three.Declined {
		t.Fatal("unencoded original inputs inferred")
	}
}
