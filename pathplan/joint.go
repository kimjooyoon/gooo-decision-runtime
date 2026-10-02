package pathplan

import (
	"container/heap"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

// JointReceipt records one real four-mask judgment, without fabricating two
// independent eight-label predictions. Returned arrays/strings are caller-owned.
type JointReceipt struct {
	Schema        string     `json:"schema"`
	Feature       string     `json:"feature_version"`
	Input         string     `json:"input,omitempty"`
	InputSHA      string     `json:"input_sha256"`
	PartSHA       [2]string  `json:"individual_input_sha256"`
	Bytes         int        `json:"input_bytes"`
	Probabilities [4]float32 `json:"full_mask_probabilities"`
	Proposed      uint16     `json:"proposed_mask"`
	Sampled       uint16     `json:"initial_mask"`
	PredictNS     int64      `json:"predict_ns"`
	Calls         int        `json:"actual_predictions"`
	Declined      bool       `json:"representation_declined"`
	Error         string     `json:"error,omitempty"`
}

// JointInput uses complete already canonical v3 strings. Native callers must
// bind source and project its facts first; arbitrary caller context grants no
// authority to change source. Sessions retain the prepared immutable snapshot.
func (prepared *PreparedPlan) JointInput() (string, error) {
	if prepared == nil || len(prepared.plan.Decisions) != 2 {
		return "", errors.New("joint paths require exactly two two-option decisions")
	}
	parts := [2]string{prepared.plan.Decisions[0].Intent, prepared.plan.Decisions[1].Intent}
	text, err := jointdecision.Encode(parts)
	if err != nil {
		text = fmt.Sprintf("gooo;joint2|%d:%s%d:%s", len(parts[0]), parts[0], len(parts[1]), parts[1])
	}
	return text, err
}
func jointReceipt(text string) JointReceipt {
	r := JointReceipt{Schema: jointdecision.Schema, Feature: jointdecision.FeatureVersion, Input: text, InputSHA: hash([]byte(text)), Bytes: len(text)}
	if parts, err := jointdecision.Parts(text); err == nil {
		for i, part := range parts {
			r.PartSHA[i] = hash([]byte(part))
		}
	}
	return r
}
func sampleJoint(planSHA string, model *jointdecision.Model, seed string, probabilities [4]float32) uint16 {
	bound := planSHA + "\x00" + model.MetadataSHA256() + "\x00" + model.WeightsSHA256() + "\x00" + seed
	var encoded [32]byte
	var total float64
	for i, p := range probabilities {
		total += float64(p)
		binary.BigEndian.PutUint64(encoded[i*8:], math.Float64bits(float64(p)))
	}
	sum := sha256.Sum256(append([]byte(bound), encoded[:]...))
	draw := float64(binary.BigEndian.Uint64(sum[:8])>>11) / (1 << 53) * total
	for i, p := range probabilities {
		draw -= float64(p)
		if draw < 0 {
			return uint16(i)
		}
	}
	return 3
}
func (session *Session) applyJointPrediction(model *jointdecision.Model, r JointReceipt, seed string) {
	initial := r.Proposed
	if seed != "" {
		initial = sampleJoint(session.prepared.sha, model, seed, r.Probabilities)
		session.result.Selection.SeedSHA256 = hash([]byte(seed))
	}
	r.Sampled = initial
	session.result.Selection.Joint = &r
	session.joint, session.ranked = true, true
	session.jointInput = r.Input
	for mask, p := range r.Probabilities {
		session.jointLogWeights[mask] = math.Log(math.Max(float64(p), 1e-12))
	}
	for i, choice := range session.prepared.plan.Decisions {
		label := choice.Options[(initial>>i)&1].Label
		session.result.InitialProposals[choice.ID] = label
		session.result.Selection.Receipts[i].Proposed = label
		session.result.Selection.Receipts[i].Selected = label
		session.result.Selection.Receipts[i].Mode = "joint_mask_rank_before_finite_validation"
	}
	clear(session.scheduled)
	session.queue = session.queue[:0]
	session.enqueue(initial)
}

// NewJointSession evaluates one complete four-mask distribution before tests.
// A nil model or input representation decline preserves deterministic paths.
// The model is never retained; each later explicit feedback must supply its pins.
func (prepared *PreparedPlan) NewJointSession(ctx context.Context, model *jointdecision.Model, cases []TestCase, seed string) (*Session, error) {
	if prepared == nil || len(prepared.plan.Decisions) != 2 {
		return nil, errors.New("joint session requires exactly two decisions")
	}
	if seed != "" && (model == nil || len(seed) > 512 || !utf8.ValidString(seed)) {
		return nil, errors.New("joint seed requires model and bounded UTF-8")
	}
	session, err := prepared.NewSession(ctx, nil, cases, "")
	if err != nil || model == nil {
		return session, err
	}
	selection := &session.result.Selection
	selection.ModelVariant, selection.MetadataSHA256, selection.WeightsSHA256 = model.Variant(), model.MetadataSHA256(), model.WeightsSHA256()
	text, err := prepared.JointInput()
	r := jointReceipt(text)
	if err != nil {
		for i, choice := range prepared.plan.Decisions {
			r.PartSHA[i] = hash([]byte(choice.Intent))
		}
		r.Declined, r.Error = true, err.Error()
		selection.Joint = &r
		return session, nil
	}
	if err = ctx.Err(); err != nil {
		session.initialized, session.initialError = false, err
		return session, err
	}
	var workspace jointdecision.Workspace
	var prediction jointdecision.Prediction
	start := time.Now()
	err = model.PredictInto(text, &workspace, &prediction)
	r.PredictNS = time.Since(start).Nanoseconds()
	r.Calls = 1
	selection.ModelCalls = 1
	r.Proposed, r.Probabilities = prediction.Mask, prediction.Probabilities
	selection.Joint = &r
	if err != nil {
		session.initialized, session.initialError = false, err
		return session, err
	}
	if err = ctx.Err(); err != nil {
		session.initialized, session.initialError = false, err
		return session, err
	}
	session.applyJointPrediction(model, r, seed)
	return session, nil
}
func (session *Session) rescoreJoint(ctx context.Context, probabilities [4]float32) error {
	return session.rescoreMaskDistribution(ctx, probabilities[:])
}
func (session *Session) rescoreMaskDistribution(ctx context.Context, probabilities []float32) error {
	if len(probabilities) != session.result.DeclaredCombinations || len(probabilities) > 8 {
		return errors.New("full bounded joint probability space required")
	}
	var weights [8]float64
	queue := make(searchHeap, len(session.queue), len(probabilities))
	var total float64
	for i, p := range probabilities {
		if p < 0 || math.IsNaN(float64(p)) || math.IsInf(float64(p), 0) {
			return errors.New("finite nonnegative joint probabilities required")
		}
		total += float64(p)
		weights[i] = math.Log(math.Max(float64(p), 1e-12))
	}
	if total == 0 {
		return errors.New("positive joint probability mass required")
	}
	for i, node := range session.queue {
		queue[i] = searchNode{node.mask, weights[node.mask]}
	}
	heap.Init(&queue)
	if err := ctx.Err(); err != nil {
		return err
	}
	session.jointLogWeights, session.queue = weights, queue
	return nil
}
func (session *Session) jointFeedbackPrefix(r FeedbackReceipt) string {
	prefix := feedbackPrefix(r, session.result.DeclaredCombinations-session.attempted)
	var labels []string
	for _, choice := range session.prepared.plan.Decisions {
		labels = append(labels, session.result.Selection.Choices[choice.ID])
	}
	return prefix + " selected=" + strings.Join(labels, ",")
}
