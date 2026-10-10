package decision

import "errors"

// DeclaredConditionFeatureVersion describes authored requirements before any
// candidate executes. It is separate from observed-condition feedback and the
// declared integer output-case ABI.
const DeclaredConditionFeatureVersion = "declared_boolean_condition_v1"
const DeclaredConditionFeatureDim = 32

type DeclaredConditionFeatureInput struct {
	Input        int64
	Expected     bool
	TargetChoice int
	ChoiceCount  int
	Index        int
	Count        int
}

// DeclaredConditionFeaturesInto encodes every int64 bit and the target choice
// without hashing IDs or converting a whole integer to floating point. The
// caller binds the fields to source; this function does not evaluate truth.
// Invalid arguments preserve the destination.
func DeclaredConditionFeaturesInto(input DeclaredConditionFeatureInput, output *[DeclaredConditionFeatureDim]float32) error {
	if output == nil || input.ChoiceCount < 1 || input.ChoiceCount > 16 ||
		input.TargetChoice < 0 || input.TargetChoice >= input.ChoiceCount ||
		input.Count < 1 || input.Count > 128 || input.Index < 0 || input.Index >= input.Count {
		return errors.New("declared condition requires a destination, 1..16 choices and a row within 1..128 conditions")
	}
	var row [DeclaredConditionFeatureDim]float32
	row[0], row[1] = 1.0/8, 1.0/8
	if input.Expected {
		row[1], row[2] = 0, 1.0/8
	}
	if input.Input == 0 {
		row[3] = 1.0 / 8
	}
	if input.Input < 0 {
		row[4] = 1.0 / 8
	}
	for j := range 8 {
		row[5+j] = float32(byte(uint64(input.Input)>>uint(56-8*j))) / 2048
	}
	row[13], row[14], row[15] = float32(input.Index)/1024, float32(input.Count)/1024, float32(input.ChoiceCount)/128
	row[16+input.TargetChoice] = 1.0 / 8
	*output = row
	return nil
}
