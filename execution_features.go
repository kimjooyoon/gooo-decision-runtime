package decision

import "errors"

// ExecutionFeatureVersion preserves v2 and appends an observed output mismatch.
const ExecutionFeatureVersion = "source_intent_condition_output_channels_v3"
const ExecutionFeatureDim = FeatureDim + 64

// OutputFeedback is one committed integer counterexample. The input, expected
// and actual values are exact. It does not specify which choice caused failure.
type OutputFeedback struct {
	Present       bool
	ChoiceCount   uint8
	Choice        uint8
	CandidateMask uint16
	Input         int64
	Expected      int64
	Actual        int64
}

// ExecutionFeaturesInto keeps v2 [0:256] unchanged and places output feedback in
// [256:320]. Integer values use exact two's-complement bytes, never whole-value
// floating-point conversion. Errors preserve the destination.
func ExecutionFeaturesInto(source [SplitContextDim]byte, roles [BranchReturnRoleDim]byte,
	intent string, condition ConditionFeedback, feedback OutputFeedback, output *[ExecutionFeatureDim]float32) error {
	if output == nil {
		return errors.New("execution feature destination required")
	}
	if err := validateOutputFeedback(feedback); err != nil {
		return err
	}
	var prefix [FeatureDim]float32
	if err := ConditionBranchFeaturesInto(source, roles, intent, condition, &prefix); err != nil {
		return err
	}
	var candidate [ExecutionFeatureDim]float32
	copy(candidate[:], prefix[:])
	outputFeedbackFields(feedback, &candidate)
	*output = candidate
	return nil
}

func validateOutputFeedback(f OutputFeedback) error {
	if !f.Present {
		if f != (OutputFeedback{}) {
			return errors.New("absent output feedback must have zero fields")
		}
		return nil
	}
	if f.ChoiceCount == 0 || f.ChoiceCount > 16 || f.Choice >= f.ChoiceCount || uint32(f.CandidateMask)>>f.ChoiceCount != 0 {
		return errors.New("output feedback must name a declared candidate mask")
	}
	if f.Expected == f.Actual {
		return errors.New("output feedback requires an actual mismatch")
	}
	return nil
}

// outputFeedbackFields only receives validated feedback from package code.
func outputFeedbackFields(f OutputFeedback, output *[ExecutionFeatureDim]float32) {
	if !f.Present {
		return
	}
	fields := output[FeatureDim:]
	flag := func(i int, value bool) {
		if value {
			fields[i] = 1.0 / 8
		}
	}
	flag(0, true)
	flag(1, f.CandidateMask>>f.Choice&1 == 0)
	flag(2, f.CandidateMask>>f.Choice&1 != 0)
	for i, value := range [3]int64{f.Input, f.Expected, f.Actual} {
		flag(3+i*3, value < 0)
		flag(4+i*3, value == 0)
		flag(5+i*3, value > 0)
		bits := uint64(value)
		for j := range 8 {
			fields[14+i*8+j] = float32(byte(bits>>uint(56-j*8))) / 2048
		}
	}
	flag(12, f.Actual < f.Expected)
	flag(13, f.Actual > f.Expected)
	for i := range 16 {
		flag(38+i, f.CandidateMask>>i&1 != 0)
	}
	fields[54], fields[55] = float32(f.ChoiceCount)/128, float32(f.Choice)/128
}
