package pathplan

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

// ContractRanking binds a single pre-execution prediction to every declared
// case and source feature. A declined representation uses declared fallback.
type ContractRanking struct {
	Schema              string         `json:"schema"`
	SHA                 string         `json:"ranking_sha256"`
	PlanSHA             string         `json:"plan_sha256"`
	CaseSHA             string         `json:"finite_cases_sha256"`
	ModelFingerprint    string         `json:"model_fingerprint,omitempty"`
	SourceFeatures      string         `json:"source_feature_version,omitempty"`
	CaseFeatures        string         `json:"case_feature_version,omitempty"`
	ConditionFeatures   string         `json:"condition_feature_version,omitempty"`
	ConditionFeatureSHA string         `json:"condition_feature_sha256,omitempty"`
	ConditionCount      int            `json:"declared_condition_count,omitempty"`
	FeatureSHA          [16]string     `json:"source_feature_sha256"`
	CaseCount           int            `json:"declared_case_count"`
	ChoiceCount         int            `json:"choice_count"`
	Logits              [16][2]float32 `json:"choice_logits"`
	Proposed            uint16         `json:"proposed_mask"`
	Calls               int            `json:"local_model_predictions"`
	PredictNS           int64          `json:"predict_ns"`
	Applied             bool           `json:"ranking_applied"`
	Declined            bool           `json:"representation_declined"`
	Error               string         `json:"error,omitempty"`
}

// ContractSession reuses the finite typed frontier and checks every case and
// condition for acceptance. It retains no model and performs no later inference.
// Advance/Observe keep the existing non-blocking busy/deadline contracts.
type ContractSession struct {
	core    *Session
	ranking ContractRanking
}

// ContractProgress binds the existing search record to its initial ranking.
// Initial declared requirements never masquerade as observed failure feedback.
type ContractProgress struct {
	Schema          string `json:"schema"`
	SHA             string `json:"progress_sha256"`
	RankingSHA      string `json:"initial_ranking_sha256"`
	SessionProgress `json:"search"`
}

func (s *ContractSession) Ranking() ContractRanking {
	if s == nil {
		return ContractRanking{}
	}
	return s.ranking
}

func (s *ContractSession) Observe() (ContractProgress, error) {
	if s == nil {
		return ContractProgress{}, errors.New("contract session required")
	}
	p, err := s.core.Observe()
	if err != nil {
		return ContractProgress{}, err
	}
	return s.progress(p), nil
}

func (s *ContractSession) Advance(ctx context.Context, budget int) (ContractProgress, *bodyplan.Program, error) {
	if s == nil {
		return ContractProgress{}, nil, errors.New("contract session required")
	}
	p, body, err := s.core.Advance(ctx, budget)
	if p.Schema == "" {
		return ContractProgress{}, body, err
	}
	return s.progress(p), body, err
}

func (s *ContractSession) progress(p SessionProgress) ContractProgress {
	result := ContractProgress{Schema: "gooo/contract-path-progress/v1", RankingSHA: s.ranking.SHA, SessionProgress: p}
	raw, _ := json.Marshal(result)
	result.SHA = hash(raw)
	return result
}

// NewContractSession invokes the local model immediately during initialization,
// before any candidate body executes. Nil uses the existing deterministic
// frontier, including its fallback order. Errors never expose a half-ranked session.
func (p *PreparedPlan) NewContractSession(ctx context.Context, model *contractdecision.Model, cases []TestCase) (*ContractSession, error) {
	if model == nil {
		return p.newContractSession(ctx, cases, "", "", "", nil)
	}
	predict := func(source [][contractdecision.FeatureDim]float32, input contractdecision.CaseSource) (contractdecision.ChoicePrediction, error) {
		var workspace contractdecision.Workspace
		var result contractdecision.ChoicePrediction
		err := model.PredictChoicesInto(source, input, &workspace, &result)
		return result, err
	}
	return p.newContractSession(ctx, cases, model.Fingerprint(), model.CaseFeatureVersion(), "contract_fp32", predict)
}

type contractPredict func([][contractdecision.FeatureDim]float32, contractdecision.CaseSource) (contractdecision.ChoicePrediction, error)

func (p *PreparedPlan) newContractSession(ctx context.Context, cases []TestCase, fingerprint, caseVersion, variant string, predict contractPredict) (*ContractSession, error) {
	core, err := p.NewSession(ctx, nil, cases, "")
	if err != nil {
		return nil, err
	}
	s := &ContractSession{core: core, ranking: ContractRanking{
		Schema: "gooo/contract-path-ranking/v1", PlanSHA: p.sha, CaseSHA: core.caseSHA,
		CaseCount: len(core.cases), ChoiceCount: len(p.plan.Decisions), Proposed: core.fallbackMask,
	}}
	if predict == nil {
		return s.finish()
	}
	r := &s.ranking
	r.ModelFingerprint = fingerprint
	r.SourceFeatures = decision.RelationalFlowFeatureVersion
	r.CaseFeatures = caseVersion
	input, err := p.InitialContractInputFor(core.cases, r.CaseFeatures)
	if err != nil {
		return nil, err
	}
	var source [16][contractdecision.FeatureDim]float32
	for i, choice := range p.plan.Decisions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := input.RelationalSourceFeaturesInto(choice.ID, &source[i]); err != nil {
			r.Declined, r.Error = true, err.Error()
			return s.finish()
		}
		var raw [contractdecision.FeatureDim * 4]byte
		for j, v := range source[i] {
			binary.LittleEndian.PutUint32(raw[j*4:], math.Float32bits(v))
		}
		r.FeatureSHA[i] = hash(raw[:])
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	start := time.Now()
	prediction, err := predict(source[:r.ChoiceCount], input)
	r.Calls, r.PredictNS = 1, time.Since(start).Nanoseconds()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.Logits, r.Proposed = prediction.Logits, 0
	for i := range r.ChoiceCount {
		if r.Logits[i][1] > r.Logits[i][0] {
			r.Proposed |= 1 << i
		}
	}
	core.ranked = true
	core.logWeights = conditionWeights(r.Logits)
	core.queue = core.queue[:0]
	clear(core.scheduled)
	core.enqueue(r.Proposed)
	core.result.Selection.ModelVariant = variant
	core.result.Selection.ModelCalls = 1
	for i, choice := range p.plan.Decisions {
		label := choice.Options[r.Proposed>>i&1].Label
		core.result.InitialProposals[choice.ID] = label
		core.result.Selection.Choices[choice.ID] = label
		core.result.Selection.Receipts[i].Selected = label
		core.result.Selection.Receipts[i].Proposed = label
		core.result.Selection.Receipts[i].Mode = "declared_contract_rank_before_finite_validation"
	}
	r.Applied = true
	return s.finish()
}

func (s *ContractSession) finish() (*ContractSession, error) {
	raw, err := json.Marshal(s.ranking)
	if err != nil {
		return nil, err
	}
	s.ranking.SHA = hash(raw)
	return s, nil
}
