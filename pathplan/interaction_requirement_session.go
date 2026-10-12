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
	if model == nil {
		return p.newContractSession(ctx, cases, "", "", "", nil)
	}
	return p.newInteractionRequirementSession(ctx, model, cases, orderedInteractionSource)
}

// NewCanonicalInteractionRequirementContractSession uses source-resolved
// expressions for equivalent direct-return and assignment forms. The selected
// candidates still compile and execute the original source-owned plan.
func (p *PreparedPlan) NewCanonicalInteractionRequirementContractSession(ctx context.Context, model *contractdecision.CanonicalInteractionRequirementModel, cases []TestCase) (*ContractSession, error) {
	if model == nil {
		return p.newContractSession(ctx, cases, "", "", "", nil)
	}
	return p.newInteractionRequirementSession(ctx, model, cases, canonicalInteractionSource)
}

// NewSourceRelativeRequirementContractSession loads the source-relative input
// contract into the ordinary finite frontier. It predicts once before any body
// execution. Nil and unsupported source shapes retain deterministic traversal.
func (p *PreparedPlan) NewSourceRelativeRequirementContractSession(ctx context.Context, model *contractdecision.SourceRelativeRequirementModel, cases []TestCase) (*ContractSession, error) {
	if model == nil {
		return p.newContractSession(ctx, cases, "", "", "", nil)
	}
	return p.newInteractionRequirementSession(ctx, model, cases, relativeInteractionSource)
}

type interactionSource uint8

const (
	orderedInteractionSource interactionSource = iota
	canonicalInteractionSource
	relativeInteractionSource
	relativeV2InteractionSource
	relativeV3InteractionSource
)

type interactionRequirementModel interface {
	Fingerprint() string
	SourceFeatureVersion() string
	CaseFeatureVersion() string
	ConditionFeatureVersion() string
	PredictChoicesInto([][contractdecision.OrderedFeatureDim]float32, contractdecision.CaseSource, contractdecision.ConditionSource, *contractdecision.InteractionRequirementWorkspace, *contractdecision.ChoicePrediction) error
}

func (p *PreparedPlan) newInteractionRequirementSession(ctx context.Context, model interactionRequirementModel, cases []TestCase, sourceKind interactionSource) (*ContractSession, error) {
	s, err := p.newContractSession(ctx, cases, "", "", "", nil)
	if err != nil {
		return s, err
	}
	r := &s.ranking
	r.Schema, r.SHA = "gooo/interaction-requirement-path-ranking/v1", ""
	if sourceKind == canonicalInteractionSource {
		r.Schema = "gooo/canonical-interaction-requirement-path-ranking/v1"
	} else if sourceKind == relativeInteractionSource {
		r.Schema = "gooo/source-relative-requirement-path-ranking/v1"
	} else if sourceKind == relativeV2InteractionSource {
		r.Schema = "gooo/source-relative-requirement-path-ranking/v2"
	} else if sourceKind == relativeV3InteractionSource {
		r.Schema = "gooo/source-relative-requirement-path-ranking/v3"
	}
	r.ModelFingerprint = model.Fingerprint()
	r.SourceFeatures, r.CaseFeatures = model.SourceFeatureVersion(), model.CaseFeatureVersion()
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
		features := input.OrderedSourceFeaturesInto
		if sourceKind == canonicalInteractionSource {
			features = input.CanonicalOrderedSourceFeaturesInto
		} else if sourceKind == relativeInteractionSource {
			features = input.SourceRelativeOrderedSourceFeaturesInto
		} else if sourceKind == relativeV2InteractionSource {
			features = input.SourceRelativeV2FeaturesInto
		} else if sourceKind == relativeV3InteractionSource {
			features = input.SourceRelativeV3FeaturesInto
		}
		if err := features(choice.ID, &source[i]); err != nil {
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
	if sourceKind == canonicalInteractionSource {
		s.core.result.Selection.ModelVariant = "canonical_interaction_requirement_contract_fp32"
	} else if sourceKind == relativeInteractionSource {
		s.core.result.Selection.ModelVariant = "source_relative_requirement_contract_fp32"
	} else if sourceKind == relativeV2InteractionSource {
		s.core.result.Selection.ModelVariant = "source_relative_v2_requirement_contract_fp32"
	} else if sourceKind == relativeV3InteractionSource {
		s.core.result.Selection.ModelVariant = "source_relative_v3_requirement_contract_fp32"
	}
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
