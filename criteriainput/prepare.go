package criteriainput

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

// Prepared owns a compact graph arena, canonical packet and immutable typed
// candidate snapshot. Each graph keeps local one-based edges; its node range
// identifies the predicate. Literal spelling is retained in one bounded pool.
type Prepared struct {
	canonical    []byte
	digest       string
	paths        *pathplan.PreparedPlan
	nodes        [MaxNodes]Node
	graphs       [MaxPredicates]Graph
	literals     [MaxLiteralBytes]byte
	nodeCount    uint16
	graphCount   uint8
	literalCount uint32
	choiceCount  uint8
	choices      [MaxChoices]sourceChoice
	predicateIDs [MaxPredicates]string
}

type sourceChoice struct {
	id, intent string
	options    [2]string
	fallback   string
}

type Summary struct {
	Schema              string `json:"schema"`
	InputSHA256         string `json:"input_sha256"`
	CandidatePlanSHA256 string `json:"candidate_plan_sha256"`
	Choices             int    `json:"choices"`
	Predicates          int    `json:"predicates"`
	Nodes               int    `json:"nodes"`
	LiteralBytes        int    `json:"literal_bytes"`
	CanonicalBytes      int    `json:"canonical_bytes"`
	NumericalEncoder    string `json:"numerical_encoder"`
	ModelPredictions    int    `json:"model_predictions"`
	CandidateExecutions int    `json:"candidate_executions"`
	PreparationScope    string `json:"preparation_scope"`
}

func Prepare(input Input) (*Prepared, error) {
	if input.Schema != Schema || !validDigest(input.SourceSHA256, true) ||
		!validDigest(input.ContractSHA256, true) || !validDigest(input.CandidatePlanSHA256, false) ||
		!validText(input.ActivityID) || input.ActivityID != input.Candidate.Base.ID {
		return nil, errors.New("source packet schema, identities or digests invalid")
	}
	if len(input.Candidate.Decisions) == 0 || len(input.Candidate.Decisions) > MaxChoices ||
		input.Candidate.Base.ResultType != decision.TypeInt || len(input.Candidate.ConditionCases) != 0 ||
		len(input.Predicates) == 0 || len(input.Predicates) > MaxPredicates {
		return nil, errors.New("scalar source requires 1..6 choices, Integer output and 1..8 predicates without finite cases")
	}
	if err := sourceBounds(input); err != nil {
		return nil, err
	}
	// Bound serialization before the caller-owned data is retained or compiled.
	raw, err := json.Marshal(input)
	if err != nil || len(raw) > MaxBytes {
		return nil, errors.New("complete source packet exceeds byte bound")
	}
	paths, err := pathplan.Prepare(input.Candidate)
	if err != nil {
		return nil, fmt.Errorf("candidate source: %w", err)
	}
	if paths.PlanSHA256() != input.CandidatePlanSHA256 {
		return nil, errors.New("candidate plan digest differs from complete typed source")
	}
	p := &Prepared{canonical: raw, paths: paths, digest: digest(raw), choiceCount: uint8(len(input.Candidate.Decisions))}
	for i, choice := range input.Candidate.Decisions {
		p.choices[i] = sourceChoice{choice.ID, choice.Intent,
			[2]string{choice.Options[0].Label, choice.Options[1].Label}, choice.Fallback}
	}
	seen := map[string]bool{}
	for _, predicate := range input.Predicates {
		if !validText(predicate.ID) || !validText(predicate.ActivityID) || seen[predicate.ID] {
			return nil, errors.New("unique complete source predicate identities required")
		}
		seen[predicate.ID] = true
		if err := p.appendGraph(predicate); err != nil {
			return nil, fmt.Errorf("predicate %q: %w", predicate.ID, err)
		}
	}
	return p, nil
}

func (p *Prepared) Summary() Summary {
	if p == nil {
		return Summary{Schema: Schema, NumericalEncoder: "UNAVAILABLE"}
	}
	return Summary{Schema: Schema, InputSHA256: p.digest, CandidatePlanSHA256: p.paths.PlanSHA256(),
		Choices: int(p.choiceCount), Predicates: int(p.graphCount), Nodes: int(p.nodeCount),
		LiteralBytes: int(p.literalCount), CanonicalBytes: len(p.canonical), NumericalEncoder: "AVAILABLE_ON_REQUEST",
		PreparationScope: "source validation and candidate compilation; no prediction or requirement evaluation"}
}

// Canonical returns caller-owned source bytes. It contains source IDs and intent
// for binding, never a claim that the source has been independently verified.
func (p *Prepared) Canonical() []byte {
	if p == nil {
		return nil
	}
	return append([]byte(nil), p.canonical...)
}

func (p *Prepared) Candidate() *pathplan.PreparedPlan {
	if p == nil {
		return nil
	}
	return p.paths
}

func validText(value string) bool {
	return value != "" && len(value) <= 1024 && utf8.ValidString(value)
}

func validDigest(value string, prefixed bool) bool {
	if prefixed {
		if !strings.HasPrefix(value, "sha256:") {
			return false
		}
		value = strings.TrimPrefix(value, "sha256:")
	}
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == sha256.Size && hex.EncodeToString(raw) == value
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
