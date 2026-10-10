package pathplan

import (
	"context"
	"encoding/binary"
	"math"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

// NewInteractionRequirementContractSession reads the explicit 528-cell source
// representation, outputs and Boolean requirements in one initial prediction.
// Nil retains the original deterministic receipt. Unsupported representations
// decline with zero predictions and retain ordinary finite search.
func (p *PreparedPlan) NewInteractionRequirementContractSession(ctx context.Context, model *contractdecision.InteractionRequirementModel, cases []TestCase) (*ContractSession, error) {
	s, err := p.newContractSession(ctx, cases, "", "", "", nil)
	if err != nil || model == nil {
		return s, err
	}
	r := &s.ranking
	r.Schema, r.SHA = "gooo/interaction-requirement-path-ranking/v1", ""
	r.ModelFingerprint = model.Fingerprint()
	r.SourceFeatures, r.CaseFeatures = contractdecision.OrderedSourceFeatureVersion, model.CaseFeatureVersion()
	r.ConditionFeatures, r.ConditionCount = model.ConditionFeatureVersion(), len(p.plan.ConditionCases)
	input, err := p.InitialContractInput(s.core.cases)
	if err != nil {
		return nil, err
	}
	r.ConditionFeatureSHA, err = requirementConditionSHA(ctx, input)
	if err != nil {
		return nil, err
	}
	var source [16][contractdecision.OrderedFeatureDim]float32
	for i, choice := range p.plan.Decisions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := input.OrderedSourceFeaturesInto(choice.ID, &source[i]); err != nil {
			r.Declined, r.Error = true, err.Error()
			return s.finish()
		}
		var raw [contractdecision.OrderedFeatureDim * 4]byte
		for j, value := range source[i] {
			binary.LittleEndian.PutUint32(raw[j*4:], math.Float32bits(value))
		}
		r.FeatureSHA[i] = hash(raw[:])
	}
	var workspace contractdecision.InteractionRequirementWorkspace
	var prediction contractdecision.ChoicePrediction
	start := time.Now()
	err = model.PredictChoicesInto(source[:r.ChoiceCount], input, input, &workspace, &prediction)
	r.Calls, r.PredictNS = 1, time.Since(start).Nanoseconds()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.applyInteractionRequirement(prediction, p)
	return s.finish()
}

func (s *ContractSession) applyInteractionRequirement(prediction contractdecision.ChoicePrediction, p *PreparedPlan) {
	r, core := &s.ranking, s.core
	r.Logits, r.Proposed = prediction.Logits, 0
	for i := range r.ChoiceCount {
		if r.Logits[i][1] > r.Logits[i][0] {
			r.Proposed |= 1 << i
		}
	}
	core.ranked, core.logWeights = true, conditionWeights(r.Logits)
	core.queue = core.queue[:0]
	clear(core.scheduled)
	core.enqueue(r.Proposed)
	core.result.Selection.ModelVariant = "interaction_requirement_contract_fp32"
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
}
