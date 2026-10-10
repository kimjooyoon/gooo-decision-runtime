package pathplan

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

// NewRequirementContractSession reads all authored output and Boolean goals
// in one initial prediction. Ordinary Gooo checks still validate each proposed
// body. Later Advance calls retain the finite frontier and make no predictions.
// Nil uses the original deterministic session and its original receipt format.
func (p *PreparedPlan) NewRequirementContractSession(ctx context.Context, model *contractdecision.RequirementModel, cases []TestCase) (*ContractSession, error) {
	if model == nil {
		return p.newContractSession(ctx, cases, "", "", "", nil)
	}
	conditionSHA := ""
	predict := func(source [][contractdecision.FeatureDim]float32, cases contractdecision.CaseSource) (contractdecision.ChoicePrediction, error) {
		var result contractdecision.ChoicePrediction
		input, ok := cases.(*ContractInput)
		if !ok {
			return result, errors.New("requirement model needs the source-bound contract input")
		}
		var err error
		conditionSHA, err = requirementConditionSHA(ctx, input)
		if err != nil {
			return result, err
		}
		var workspace contractdecision.RequirementWorkspace
		err = model.PredictChoicesInto(source, input, input, &workspace, &result)
		return result, err
	}
	session, err := p.newContractSession(ctx, cases, model.Fingerprint(), model.CaseFeatureVersion(), "requirement_conditioned_contract_fp32", predict)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r := &session.ranking
	r.Schema, r.SHA = "gooo/requirement-path-ranking/v1", ""
	r.ConditionFeatures, r.ConditionFeatureSHA = model.ConditionFeatureVersion(), conditionSHA
	r.ConditionCount = len(p.plan.ConditionCases)
	return session.finish()
}

func requirementConditionSHA(ctx context.Context, input *ContractInput) (string, error) {
	h := sha256.New()
	h.Write([]byte(decision.DeclaredConditionFeatureVersion + "\x00"))
	var cells [decision.DeclaredConditionFeatureDim * 4]byte
	binary.LittleEndian.PutUint32(cells[:4], uint32(input.ConditionCount()))
	h.Write(cells[:4])
	for i := range input.ConditionCount() {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		row, err := input.ConditionFeatures(i)
		if err != nil {
			return "", err
		}
		for j, value := range row {
			binary.LittleEndian.PutUint32(cells[j*4:], math.Float32bits(value))
		}
		h.Write(cells[:])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
