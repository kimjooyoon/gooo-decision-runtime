package pathplan

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
)

type ThreeReceipt struct {
	Schema          string     `json:"schema"`
	Feature         string     `json:"feature_version"`
	Input           string     `json:"input,omitempty"`
	InputSHA        string     `json:"input_sha256"`
	PartSHA         [3]string  `json:"individual_input_sha256"`
	Bytes           int        `json:"input_bytes"`
	Decisions       int        `json:"declared_decisions"`
	Probabilities   [8]float32 `json:"full_mask_probabilities"`
	Proposed        uint16     `json:"proposed_mask"`
	Sampled         uint16     `json:"initial_mask"`
	PredictionValid bool       `json:"prediction_valid"`
	PredictNS       int64      `json:"predict_ns"`
	Calls           int        `json:"actual_predictions"`
	Declined        bool       `json:"representation_declined"`
	Error           string     `json:"error,omitempty"`
}

func (prepared *PreparedPlan) ThreeInput() (string, error) {
	if prepared == nil {
		return "", errors.New("prepared three-choice plan required")
	}
	if len(prepared.plan.Decisions) == 3 {
		parts := [3]string{prepared.plan.Decisions[0].Intent, prepared.plan.Decisions[1].Intent, prepared.plan.Decisions[2].Intent}
		if text, err := jointdecision.EncodeThree(parts); err == nil {
			return text, nil
		}
	}
	// Retain every original input even when the fixed ABI cannot represent it.
	var b strings.Builder
	if len(prepared.plan.Decisions) == 3 {
		b.WriteString("gooo;joint3|")
	} else {
		b.WriteString("gooo;unsupported3;count=")
		b.WriteString(strconv.Itoa(len(prepared.plan.Decisions)))
		b.WriteByte('|')
	}
	for _, choice := range prepared.plan.Decisions {
		b.WriteString(strconv.Itoa(len(choice.Intent)))
		b.WriteByte(':')
		b.WriteString(choice.Intent)
	}
	return b.String(), errors.New("complete three-choice input is unsupported")
}
func threeReceipt(text string, decisions int) ThreeReceipt {
	r := ThreeReceipt{Schema: jointdecision.ThreeSchema, Feature: jointdecision.ThreeFeatureVersion, Input: text, InputSHA: hash([]byte(text)), Bytes: len(text), Decisions: decisions}
	if parts, err := jointdecision.ThreeParts(text); err == nil {
		for i, part := range parts {
			r.PartSHA[i] = hash([]byte(part))
		}
	}
	return r
}
func sampleThree(planSHA string, model *jointdecision.ThreeModel, seed string, p [8]float32) uint16 {
	bound := planSHA + "\x00" + model.MetadataSHA256() + "\x00" + model.WeightsSHA256() + "\x00" + seed
	var encoded [64]byte
	var total float64
	for i, v := range p {
		total += float64(v)
		binary.BigEndian.PutUint64(encoded[i*8:], math.Float64bits(float64(v)))
	}
	sum := sha256.Sum256(append([]byte(bound), encoded[:]...))
	draw := float64(binary.BigEndian.Uint64(sum[:8])>>11) / (1 << 53) * total
	for i, v := range p {
		draw -= float64(v)
		if draw < 0 {
			return uint16(i)
		}
	}
	return 7
}
func (session *Session) applyThree(model *jointdecision.ThreeModel, r ThreeReceipt, seed string) {
	initial := r.Proposed
	if seed != "" {
		initial = sampleThree(session.prepared.sha, model, seed, r.Probabilities)
		session.result.Selection.SeedSHA256 = hash([]byte(seed))
	}
	r.Sampled = initial
	session.result.Selection.Three = &r
	session.joint, session.ranked = true, true
	session.jointInput = r.Input
	for mask, p := range r.Probabilities {
		session.jointLogWeights[mask] = math.Log(math.Max(float64(p), 1e-12))
	}
	for i, choice := range session.prepared.plan.Decisions {
		label := choice.Options[(initial>>i)&1].Label
		session.result.InitialProposals[choice.ID] = label
		receipt := &session.result.Selection.Receipts[i]
		receipt.Proposed, receipt.Selected, receipt.Mode = label, label, "three_choice_mask_rank_before_finite_validation"
	}
	clear(session.scheduled)
	session.queue = session.queue[:0]
	session.enqueue(initial)
}

// NewThreeSession records unsupported full input and continues deterministically
// with zero calls. Models are never retained; subsequent feedback must repin.
func (prepared *PreparedPlan) NewThreeSession(ctx context.Context, model *jointdecision.ThreeModel, cases []TestCase, seed string) (*Session, error) {
	if prepared == nil {
		return nil, errors.New("prepared three-choice plan required")
	}
	if seed != "" && (model == nil || len(seed) > 512 || !utf8.ValidString(seed)) {
		return nil, errors.New("three-choice seed requires model and bounded UTF-8")
	}
	session, err := prepared.NewSession(ctx, nil, cases, "")
	if err != nil || model == nil {
		return session, err
	}
	selection := &session.result.Selection
	selection.ModelVariant, selection.MetadataSHA256, selection.WeightsSHA256 = model.Variant(), model.MetadataSHA256(), model.WeightsSHA256()
	text, err := prepared.ThreeInput()
	r := threeReceipt(text, len(prepared.plan.Decisions))
	if err != nil {
		if len(prepared.plan.Decisions) == 3 {
			for i, choice := range prepared.plan.Decisions {
				r.PartSHA[i] = hash([]byte(choice.Intent))
			}
		}
		r.Declined, r.Error = true, err.Error()
		selection.Three = &r
		return session, nil
	}
	if err = ctx.Err(); err != nil {
		session.initialized, session.initialError = false, err
		return session, err
	}
	var workspace jointdecision.ThreeWorkspace
	var prediction jointdecision.ThreePrediction
	start := time.Now()
	err = model.PredictInto(text, &workspace, &prediction)
	r.PredictNS, r.Calls = time.Since(start).Nanoseconds(), 1
	selection.ModelCalls = 1
	r.Proposed, r.Probabilities, r.PredictionValid = prediction.Mask, prediction.Probabilities, err == nil
	selection.Three = &r
	if err != nil {
		session.initialized, session.initialError = false, err
		return session, err
	}
	if err = ctx.Err(); err != nil {
		session.initialized, session.initialError = false, err
		return session, err
	}
	session.applyThree(model, r, seed)
	return session, nil
}
