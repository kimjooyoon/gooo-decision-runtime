package pathplan

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testJointModel(t *testing.T) *jointdecision.Model {
	t.Helper()
	dir := t.TempDir()
	raw := make([]byte, 49648)
	for i, v := range [4]float32{3, 0, 2, 1} {
		binary.LittleEndian.PutUint32(raw[len(raw)-16+i*4:], math.Float32bits(v))
	}
	meta := jointdecision.Metadata{Schema: jointdecision.Schema, Feature: jointdecision.FeatureVersion, Variant: "fp32", FeatureDim: 512, HiddenDim: 24, MaxBytes: 1088, Labels: []string{"mask_0", "mask_1", "mask_2", "mask_3"}, Temperature: 1, WeightsFile: "weights.bin", WeightsSHA: hash(raw)}
	var offset int64
	for i, name := range [4]string{"w1", "b1", "w2", "b2"} {
		rows, cols := [4]int{24, 1, 4, 1}[i], [4]int{512, 24, 24, 4}[i]
		count := rows * cols
		meta.Tensors = append(meta.Tensors, decision.TensorMetadata{Name: name, Rows: rows, Cols: cols, Count: count, Encoding: "float32_le", Offset: offset, Bytes: int64(count * 4), Scale: 1})
		offset += int64(count * 4)
	}
	encoded, _ := json.Marshal(meta)
	name := filepath.Join(dir, "model.json")
	if err := os.WriteFile(name, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "weights.bin"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	m, err := jointdecision.Load(name)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func jointPrepared(t *testing.T, full bool) *PreparedPlan {
	t.Helper()
	plan := interactingPlan()
	base, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range plan.Decisions {
		fields, err := base.SourceFeatures(c.ID)
		if err != nil {
			t.Fatal(err)
		}
		natural := "Use the first declared local."
		if full {
			natural = strings.Repeat("x", 364)
		}
		plan.Decisions[i].Intent, err = decision.EncodeSemanticContextInput(fields, natural)
		if err != nil {
			t.Fatal(err)
		}
	}
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}
func TestJointGlobalRankingOneCallFeedbackAndSolePathZeroCalls(t *testing.T) {
	p, m := jointPrepared(t, false), testJointModel(t)
	ctx := sessionContext(t)
	cases := []TestCase{{Input: 3, Expected: 999}}
	session, err := p.NewJointSession(ctx, m, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	initial, _ := session.Observe()
	if initial.Selection.ModelCalls != 1 || initial.Selection.Joint.Calls != 1 {
		t.Fatal("fabricated/duplicate predictions")
	}
	var seen []uint16
	for i := 0; i < 4; i++ {
		progress, _, err := session.Advance(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		seen = append(seen, progress.NewAttempts[0].Mask)
		if i < 3 {
			r, err := session.ReconsiderJoint(ctx, m, &CIHint{SourceSHA: strings.Repeat("a", 40), Status: "PASS"})
			if err != nil || r.CIIsAuthority {
				t.Fatal(err)
			}
			expected := 1
			if i == 2 {
				expected = 0
				if !r.RankingUnnecessary {
					t.Fatal("sole mask unnecessarily ranked")
				}
			}
			if r.ModelCalls != expected || r.SHA == "" {
				t.Fatal("real feedback accounting", r)
			}
		}
	}
	if seen[0] != 0 || seen[1] != 2 || seen[2] != 3 || seen[3] != 1 {
		t.Fatal("full joint ordering lost correlation", seen)
	}
	final, _ := session.Observe()
	if final.Selection.ModelCalls != 3 || final.Attempted != 4 || !final.Exhausted {
		t.Fatal("joint totals", final.Selection.ModelCalls)
	}
	if _, err = session.ReconsiderJoint(ctx, m, nil); err == nil {
		t.Fatal("exhausted feedback accepted")
	}
}
func TestJointInputOverflowAtomicityOwnershipBusyAndCancellation(t *testing.T) {
	p, m := jointPrepared(t, true), testJointModel(t)
	ctx := sessionContext(t)
	s, err := p.NewJointSession(ctx, m, []TestCase{{Input: 3, Expected: 999}}, "")
	if err != nil {
		t.Fatal(err)
	}
	s.Advance(ctx, 1)
	before := append(searchHeap(nil), s.queue...)
	r, err := s.ReconsiderJoint(ctx, m, nil)
	if !errors.Is(err, ErrFeedbackContextBound) || !r.ContextDeclined || r.ModelCalls != 0 || r.SHA == "" || r.DeclinedInputSHA == "" || r.Joint == nil || !r.Joint.Declined {
		t.Fatal("complete overflow not accounted", r, err)
	}
	if len(before) != len(s.queue) {
		t.Fatal("decline altered frontier")
	}
	for i := range before {
		if before[i] != s.queue[i] {
			t.Fatal("decline changed ranking")
		}
	}
	observed, _ := s.Observe()
	observed.Selection.Joint.Probabilities[0] = 999
	owned, _ := s.Observe()
	if owned.Selection.Joint.Probabilities[0] == 999 {
		t.Fatal("borrowed receipt mutated session")
	}
	s.lock.Lock()
	_, _, err = s.Advance(ctx, 1)
	if !errors.Is(err, ErrSessionBusy) {
		t.Fatal("busy caller blocks")
	}
	_, err = s.ReconsiderJoint(ctx, m, nil)
	if !errors.Is(err, ErrSessionBusy) {
		t.Fatal("feedback busy caller blocks")
	}
	s.lock.Unlock()
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err = s.Advance(canceled, 1); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled advance")
	}
}
func TestJointSeedReplayDisconnectedAndRepresentationDecline(t *testing.T) {
	p, m := jointPrepared(t, false), testJointModel(t)
	ctx := sessionContext(t)
	cases := []TestCase{{Input: 3, Expected: 999}}
	a, err := p.NewJointSession(ctx, m, cases, "seed-one")
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.NewJointSession(ctx, m, cases, "seed-one")
	if err != nil {
		t.Fatal(err)
	}
	left, _ := a.Observe()
	right, _ := b.Observe()
	if left.Selection.Joint.Sampled != right.Selection.Joint.Sampled || left.Selection.Joint.InputSHA != right.Selection.Joint.InputSHA || strings.Contains(left.Selection.Joint.Input, "seed-one") {
		t.Fatal("seed changes input or replay")
	}
	offline, err := p.NewJointSession(ctx, nil, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	state, _ := offline.Observe()
	if state.Selection.ModelCalls != 0 {
		t.Fatal("offline predicted")
	}
	invalid, err := Prepare(interactingPlan())
	if err != nil {
		t.Fatal(err)
	}
	declined, err := invalid.NewJointSession(ctx, m, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = declined.Observe()
	if state.Selection.ModelCalls != 0 || state.Selection.Joint == nil || !state.Selection.Joint.Declined {
		t.Fatal("invalid representation inferred")
	}
}
