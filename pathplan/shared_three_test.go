package pathplan

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
)

func testSharedThreeModel(t *testing.T) *jointdecision.ThreeModel {
	t.Helper()
	raw := make([]byte, 8288)
	meta := jointdecision.Metadata{Schema: jointdecision.SharedThreeSchema, Feature: jointdecision.ThreeFeatureVersion, Variant: "fp32", FeatureDim: 768, HiddenDim: 8, MaxBytes: 1600, Temperature: 1, WeightsFile: "weights.bin", WeightsSHA: hash(raw), Labels: []string{"mask_0", "mask_1", "mask_2", "mask_3", "mask_4", "mask_5", "mask_6", "mask_7"}}
	var at int64
	for i, name := range [3]string{"w1", "b1", "w2"} {
		rows, cols := [3]int{8, 1, 2}[i], [3]int{256, 8, 8}[i]
		n := rows * cols
		meta.Tensors = append(meta.Tensors, decision.TensorMetadata{Name: name, Rows: rows, Cols: cols, Count: n, Offset: at, Bytes: int64(n * 4), Encoding: "float32_le", Scale: 1})
		at += int64(n * 4)
	}
	b, e := json.Marshal(meta)
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	if e = os.WriteFile(filepath.Join(dir, "model.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "weights.bin"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	m, e := jointdecision.LoadSharedThree(filepath.Join(dir, "model.json"))
	if e != nil {
		t.Fatal(e)
	}
	return m
}

func TestSharedThreeSessionReceiptsAndFeedbackUseActualArtifact(t *testing.T) {
	p, m, ctx := threePrepared(t, false), testSharedThreeModel(t), threeContext(t)
	cases := []TestCase{{Input: 3, Expected: 999}}
	s, e := p.NewThreeSession(ctx, m, cases, "")
	if e != nil {
		t.Fatal(e)
	}
	first, e := s.Observe()
	if e != nil {
		t.Fatal(e)
	}
	if first.Selection.Three.Schema != jointdecision.SharedThreeSchema || first.Selection.ModelCalls != 1 || first.Selection.MetadataSHA256 != m.MetadataSHA256() || first.Selection.WeightsSHA256 != m.WeightsSHA256() {
		t.Fatal("compact initial receipt lost artifact identity")
	}
	if _, _, e = s.Advance(ctx, 1); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ReconsiderThree(ctx, testThreeModel(t), nil); e == nil {
		t.Fatal("different artifact accepted")
	}
	f, e := s.ReconsiderThree(ctx, m, nil)
	if e != nil || f.Three == nil || f.Three.Schema != jointdecision.SharedThreeSchema || f.ModelCalls != 1 || !f.Applied {
		t.Fatal("compact feedback receipt", f, e)
	}
	for _, seed := range []string{"seed-one", "seed-two", "한글"} {
		a, e := p.NewThreeSession(ctx, m, cases, seed)
		if e != nil {
			t.Fatal(e)
		}
		b, e := p.NewThreeSession(ctx, m, cases, seed)
		if e != nil {
			t.Fatal(e)
		}
		aa, _ := a.Observe()
		bb, _ := b.Observe()
		if aa.Selection.Three.Sampled != bb.Selection.Three.Sampled || aa.Selection.SeedSHA256 == "" {
			t.Fatal("same artifact and seed not reproducible")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = p.NewThreeSession(canceled, m, cases, ""); !errors.Is(e, context.Canceled) {
		t.Fatal("canceled compact session", e)
	}
	zero, e := p.NewThreeSession(ctx, nil, cases, "")
	if e != nil {
		t.Fatal(e)
	}
	off, _ := zero.Observe()
	if off.Selection.ModelCalls != 0 || off.Selection.Three != nil {
		t.Fatal("disconnected model called")
	}
}

func TestSharedThreeUnsupportedArityRetainsFullIntentAndDeterministicContinuation(t *testing.T) {
	plan := threePrepared(t, false).plan
	plan.Decisions = plan.Decisions[:2]
	p, e := Prepare(plan)
	if e != nil {
		t.Fatal(e)
	}
	ctx, m := threeContext(t), testSharedThreeModel(t)
	cases := []TestCase{{Input: 3, Expected: 999}}
	s, e := p.NewThreeSession(ctx, m, cases, "")
	if e != nil {
		t.Fatal(e)
	}
	r, _ := s.Observe()
	if r.Selection.Three == nil || !r.Selection.Three.Declined || r.Selection.Three.Schema != jointdecision.SharedThreeSchema || r.Selection.ModelCalls != 0 {
		t.Fatal("unsupported compact input inferred")
	}
	for _, c := range plan.Decisions {
		if !strings.Contains(r.Selection.Three.Input, c.Intent) {
			t.Fatal("original intent truncated")
		}
	}
	base, e := p.NewSession(ctx, nil, cases, "")
	if e != nil {
		t.Fatal(e)
	}
	a, _, ae := s.Advance(ctx, 4)
	b, _, be := base.Advance(ctx, 4)
	if !reflect.DeepEqual(a.NewAttempts, b.NewAttempts) || !errors.Is(ae, be) || a.Selection.ModelCalls != 0 {
		t.Fatal("deterministic continuation changed")
	}
}

func TestSharedThreeOversizeFeedbackRetainsAllPartsWithoutCallingModel(t *testing.T) {
	p, m, ctx := threePrepared(t, true), testSharedThreeModel(t), threeContext(t)
	s, e := p.NewThreeSession(ctx, m, []TestCase{{Input: 3, Expected: 999}}, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Advance(ctx, 1); e != nil {
		t.Fatal(e)
	}
	queue := append(searchHeap(nil), s.queue...)
	weights, calls := s.jointLogWeights, s.result.Selection.ModelCalls
	r, e := s.ReconsiderThree(ctx, m, nil)
	if !errors.Is(e, ErrFeedbackContextBound) || r.ModelCalls != 0 || !r.ContextDeclined || r.Three == nil ||
		!r.Three.Declined || r.Three.Schema != jointdecision.SharedThreeSchema || s.result.Selection.ModelCalls != calls ||
		weights != s.jointLogWeights || !reflect.DeepEqual(queue, s.queue) {
		t.Fatal("oversize compact feedback changed ranking", r, e)
	}
	for _, c := range p.plan.Decisions {
		if !strings.Contains(r.Three.Input, c.Intent) {
			t.Fatal("full intent lost")
		}
	}
}
