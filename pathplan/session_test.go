package pathplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

func sessionContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}
func zeroPathModel(t *testing.T) *decision.Model {
	t.Helper()
	dir := t.TempDir()
	raw := make([]byte, (decision.FeatureDim*decision.HiddenDim+decision.HiddenDim+decision.HiddenDim*decision.LabelCount+decision.LabelCount)*4)
	threshold := 1.0
	labels := decision.PathLabels()
	metadata := decision.Metadata{Schema: decision.PathMetadataSchema, Variant: "fp32", FeatureDim: decision.FeatureDim, HiddenDim: decision.HiddenDim, MaxBytes: decision.InputMaxBytes, Labels: labels[:], Temperature: 1, ConfidenceThreshold: &threshold, WeightsFile: "weights.bin", WeightsSHA256: hash(raw)}
	offset := int64(0)
	for _, tensor := range []struct {
		name       string
		rows, cols int
	}{{"w1", decision.HiddenDim, decision.FeatureDim}, {"b1", 1, decision.HiddenDim}, {"w2", decision.LabelCount, decision.HiddenDim}, {"b2", 1, decision.LabelCount}} {
		count := tensor.rows * tensor.cols
		bytes := int64(count * 4)
		metadata.Tensors = append(metadata.Tensors, decision.TensorMetadata{Name: tensor.name, Count: count, Rows: tensor.rows, Cols: tensor.cols, Encoding: "float32_le", Offset: offset, Bytes: bytes, Scale: 1})
		offset += bytes
	}
	if err := os.WriteFile(filepath.Join(dir, "weights.bin"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(dir, "model.json")
	if err := os.WriteFile(name, data, 0600); err != nil {
		t.Fatal(err)
	}
	model, err := decision.LoadPath(name)
	if err != nil {
		t.Fatal(err)
	}
	return model
}
func normalizeReceipts(value Selection) Selection {
	value = ownedSelection(value)
	for i := range value.Receipts {
		value.Receipts[i].PredictNS = 0
	}
	return value
}
func TestSessionBatchesKeepLegacySearchAndNoRepeatedPredictions(t *testing.T) {
	ctx := sessionContext(t)
	prepared, err := Prepare(interactingPlan())
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []*decision.Model{nil, zeroPathModel(t)} {
		for _, expected := range []int64{16, 17, 999} {
			cases := []TestCase{{Input: 3, Expected: expected}}
			seed := ""
			if model != nil {
				seed = "session-regression"
			}
			old, oldBody, err := prepared.Search(ctx, model, cases, 4, seed)
			if err != nil {
				t.Fatal(err)
			}
			session, err := prepared.NewSession(ctx, model, cases, seed)
			if err != nil {
				t.Fatal(err)
			}
			var attempts []SearchAttempt
			var progress SessionProgress
			previous := ""
			for range 4 {
				next, body, err := session.Advance(ctx, 1)
				if err != nil && !errors.Is(err, ErrNoTypedCandidate) {
					t.Fatal(err)
				}
				if next.PreviousSHA != previous {
					t.Fatal("broken progress chain")
				}
				previous = next.SHA
				data := next
				data.SHA = ""
				raw, _ := json.Marshal(data)
				if hash(raw) != next.SHA {
					t.Fatal("invalid progress digest")
				}
				attempts = append(attempts, next.NewAttempts...)
				progress = next
				if next.PredictionsThisAdvance != 0 {
					t.Fatal("advance called the model again")
				}
				if next.Status == "TRAINING_COMPLETE" || next.Exhausted {
					if body == nil || body.GoSource() != oldBody.GoSource() {
						t.Fatal("batching changed selected body")
					}
					break
				}
			}
			if !reflect.DeepEqual(attempts, old.Attempts) || !reflect.DeepEqual(normalizeReceipts(progress.Selection), normalizeReceipts(old.Selection)) || progress.Attempted != len(old.Attempts) || progress.Unattempted != old.Unattempted || progress.TypeRejected != old.TypeRejected || progress.Evaluated != old.Evaluated || progress.SelectedPassed != old.SelectedTrainingPassed || progress.Status != old.Status {
				t.Fatalf("session changed finite search: %+v", progress)
			}
			if progress.Selection.ModelCalls != old.Selection.ModelCalls || progress.ScheduledBytes != 8 || len(session.result.Attempts) != 0 {
				t.Fatal("model accounting or bounded state differs")
			}
			extra, _, err := session.Advance(ctx, 1)
			if err != nil || len(extra.NewAttempts) != 0 || extra.Attempted != progress.Attempted {
				t.Fatal("terminal session repeated work")
			}
		}
	}
}
func TestSessionOwnsCasesAndReturnedEvidence(t *testing.T) {
	ctx := sessionContext(t)
	plan := interactingPlan()
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{Input: 3, Expected: 999}}
	session, err := prepared.NewSession(ctx, nil, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	cases[0] = TestCase{Input: 3, Expected: 16}
	plan.Base.Expressions[1].Int = 999
	first, body, err := session.Advance(ctx, 1)
	if err != nil || body == nil || first.Status != "PARTIAL" {
		t.Fatal("caller mutation changed finite expectations")
	}
	saved := first.SHA
	first.Selection.Choices["reference"] = "mutated"
	first.Selection.Receipts[0].Selected = "mutated"
	first.InitialProposals["reference"] = "mutated"
	first.BestCases[0].Actual = 999
	first.NewAttempts[0].Choices["reference"] = "mutated"
	first.NewAttempts[0].Results[0].Actual = 999
	next, body, err := session.Advance(ctx, 3)
	if err != nil || next.PreviousSHA != saved || next.SelectedPassed != 0 || next.Status != "PARTIAL" || !next.Exhausted || next.BestCases[0].Actual != 16 || next.Selection.Choices["reference"] == "mutated" || next.InitialProposals["reference"] == "mutated" {
		t.Fatal("returned evidence mutated session")
	}
	if value, err := body.Evaluate(3); err != nil || value.Int != 16 {
		t.Fatal("returned evidence changed best arena")
	}
}

type interruptedSessionContext struct {
	context.Context
	checks int
}

func (ctx *interruptedSessionContext) Err() error {
	ctx.checks++
	if ctx.checks >= 4 {
		return context.Canceled
	}
	return nil
}
func TestSessionCancellationRequeuesUnfinishedCandidateAndBusyDoesNotWait(t *testing.T) {
	ctx := sessionContext(t)
	prepared, err := Prepare(interactingPlan())
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{Input: 3, Expected: 999}, {Input: 4, Expected: 999}}
	session, err := prepared.NewSession(ctx, nil, cases, "")
	if err != nil {
		t.Fatal(err)
	}
	interrupted, body, err := session.Advance(&interruptedSessionContext{Context: ctx}, 4)
	if !errors.Is(err, context.Canceled) || body != nil || !interrupted.Interrupted || interrupted.Attempted != 0 || len(interrupted.NewAttempts) != 0 || session.queue.Len() != 1 {
		t.Fatal("canceled attempt was committed or lost")
	}
	next, _, err := session.Advance(ctx, 4)
	if err != nil || next.Attempted != 4 || !next.Exhausted {
		t.Fatal("canceled frontier could not resume")
	}
	session.lock.Lock()
	done := make(chan error, 1)
	go func() { _, _, err := session.Advance(ctx, 1); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, ErrSessionBusy) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("busy session blocked")
	}
	session.lock.Unlock()
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, _, err := session.Advance(canceled, 1); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled advance accepted")
	}
	if _, _, err := session.Advance(context.Background(), 1); err == nil {
		t.Fatal("unbounded advance accepted")
	}
	if _, _, err := session.Advance(ctx, 65); err == nil {
		t.Fatal("oversized advance accepted")
	}
	var absent *Session
	if _, _, err := absent.Advance(ctx, 1); err == nil {
		t.Fatal("nil session accepted")
	}
}
func chainedDecisionPlan(count int) Plan {
	base := bodyplan.Plan{Schema: bodyplan.Schema, ID: "session-bit-bounds", Name: "MaskBounds", ResultType: decision.TypeInt, Expressions: []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 2}}, Statements: []bodyplan.Stmt{{Kind: "return", Expr: count + 1}}, Root: []int{0}}
	plan := Plan{Schema: Schema, Base: base}
	for i := range count {
		left := 0
		if i > 0 {
			left = i + 1
		}
		plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "binary", Operation: "add", Left: left, Right: 1})
		plan.Decisions = append(plan.Decisions, Choice{ID: fmt.Sprintf("sum-%d", i), Kind: OperandOrder, Target: i + 2, Intent: "Mask bounds unit fixture.", Fallback: "layout_reverse", Options: []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}})
	}
	return plan
}
func TestSessionCanAdvancePastSixtyFourWithoutRetainingOldAttemptLogs(t *testing.T) {
	ctx := sessionContext(t)
	prepared, err := Prepare(chainedDecisionPlan(7))
	if err != nil {
		t.Fatal(err)
	}
	session, err := prepared.NewSession(ctx, nil, []TestCase{{Input: 3, Expected: 999}}, "")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[uint16]bool{}
	for iteration := 1; iteration <= 2; iteration++ {
		progress, _, err := session.Advance(ctx, 64)
		if err != nil {
			t.Fatal(err)
		}
		if len(progress.NewAttempts) != 64 || progress.Attempted != 64*iteration || progress.ScheduledBytes != 16 {
			t.Fatal("batch budget or bitset differs")
		}
		for _, attempt := range progress.NewAttempts {
			if seen[attempt.Mask] {
				t.Fatal("a completed mask repeated")
			}
			seen[attempt.Mask] = true
		}
		if len(session.result.Attempts) != 0 {
			t.Fatal("session retained old attempt logs")
		}
	}
	if len(seen) != 128 || session.queue.Len() != 0 {
		t.Fatal("finite space was not exhausted")
	}
	high, err := Prepare(chainedDecisionPlan(16))
	if err != nil {
		t.Fatal(err)
	}
	session, err = high.NewSession(ctx, nil, []TestCase{{Input: 3, Expected: 999}}, "")
	if err != nil {
		t.Fatal(err)
	}
	progress, _, err := session.Advance(ctx, 1)
	if err != nil || progress.NewAttempts[0].Mask != 65535 || progress.Declared != 65536 || progress.ScheduledBytes != 8192 {
		t.Fatal("highest uint16 mask or bitset bound failed")
	}
}

func TestSessionInitialObservationKeepsCallsFromCanceledRanking(t *testing.T) {
	ctx := sessionContext(t)
	prepared, err := Prepare(interactingPlan())
	if err != nil {
		t.Fatal(err)
	}
	model := zeroPathModel(t)
	incomplete, err := prepared.NewSession(&interruptedSessionContext{Context: ctx}, model, []TestCase{{Input: 3, Expected: 999}}, "")
	if !errors.Is(err, context.Canceled) || incomplete == nil {
		t.Fatal("ranking cancellation did not preserve its observation")
	}
	progress, err := incomplete.Observe()
	if err != nil || progress.Initialized || !progress.Interrupted || progress.InitializationError == "" || progress.Selection.ModelCalls != 2 || progress.Attempted != 0 {
		t.Fatal("canceled ranking calls were lost or candidate work invented")
	}
	next, body, err := incomplete.Advance(ctx, 1)
	if !errors.Is(err, context.Canceled) || body != nil || len(next.NewAttempts) != 0 {
		t.Fatal("incompletely initialized session could assemble")
	}
	ready, err := prepared.NewSession(ctx, model, []TestCase{{Input: 3, Expected: 999}}, "")
	if err != nil {
		t.Fatal(err)
	}
	initial, err := ready.Observe()
	if err != nil || !initial.Initialized || initial.Interrupted || initial.Selection.ModelCalls != 2 || initial.Attempted != 0 || len(initial.NewAttempts) != 0 {
		t.Fatal("initial observation ran finite tests")
	}
	first, _, err := ready.Advance(ctx, 1)
	if err != nil || first.PreviousSHA != initial.SHA || first.Sequence != 2 || first.Selection.ModelCalls != 2 {
		t.Fatal("first advance did not bind the initial model observation")
	}
}

func TestSessionBatchAdapterPreservesLegacyEnvelope(t *testing.T) {
	ctx := sessionContext(t)
	prepared, err := Prepare(interactingPlan())
	if err != nil {
		t.Fatal(err)
	}
	cases := []TestCase{{Input: 3, Expected: 999}}
	for _, model := range []*decision.Model{nil, zeroPathModel(t)} {
		old, oldBody, err := prepared.Search(ctx, model, cases, 4, "")
		if err != nil {
			t.Fatal(err)
		}
		result, body, records, err := prepared.SearchBatches(ctx, model, cases, 4, 1, "")
		old.Selection = normalizeReceipts(old.Selection)
		result.Selection = normalizeReceipts(result.Selection)
		if err != nil || !reflect.DeepEqual(result, old) || body.GoSource() != oldBody.GoSource() || len(records) != 5 || records[0].Attempted != 0 || records[4].Attempted != 4 {
			t.Fatal("batch adapter changed finite legacy result")
		}
		for _, record := range records {
			if record.PredictionsThisAdvance != 0 || record.Selection.ModelCalls != result.Selection.ModelCalls {
				t.Fatal("batch adapter called model again")
			}
		}
	}
	if _, _, _, err := prepared.SearchBatches(ctx, nil, cases, 65, 1, ""); err == nil {
		t.Fatal("adapter exceeded native total budget")
	}
	if _, _, _, err := prepared.SearchBatches(ctx, nil, cases, 4, 0, ""); err == nil {
		t.Fatal("invalid step accepted")
	}
}
