package decision

import "errors"

// ConditionBranchFeatureVersion adds static branch-return roles to the twenty
// reserved v1 cells. It requires explicitly trained weights for this input ABI.
const ConditionBranchFeatureVersion = "source_intent_condition_branch_roles_v2"

const BranchReturnRoleDim = 20

// ConditionBranchFeaturesInto preserves all 236 populated v1 cells exactly.
// Roles occupy [236:256], independently of intent and observed conditions.
// Each arm has nine flags (0/128) and one saturated count (multiples of 8).
// Errors preserve the caller-owned destination, including invalid role bytes.
func ConditionBranchFeaturesInto(source [SplitContextDim]byte, roles [BranchReturnRoleDim]byte,
	intent string, feedback ConditionFeedback, output *[FeatureDim]float32) error {
	if output == nil {
		return errors.New("condition branch feature destination required")
	}
	for i, value := range roles {
		if value > 128 || (i%10 == 9 && value%8 != 0) || (i%10 != 9 && value != 0 && value != 128) {
			return errors.New("bounded branch return flags and counts required")
		}
	}
	var candidate [FeatureDim]float32
	if err := ConditionFeaturesInto(source, intent, feedback, &candidate); err != nil {
		return err
	}
	for i, value := range roles {
		candidate[FeatureDim-BranchReturnRoleDim+i] = float32(value) / 128
	}
	*output = candidate
	return nil
}
