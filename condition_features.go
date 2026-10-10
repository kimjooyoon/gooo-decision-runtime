package decision

import (
	"errors"
	"unicode/utf8"
)

// ConditionChannelFeatureVersion identifies the v1 condition training input.
// Earlier path-model loaders retain their own distinct feature contracts.
const ConditionChannelFeatureVersion = "source_intent_condition_channels_v1"

// ConditionFeedback describes one observation of a declared candidate mask.
// Choice indexes refer to the same immutable plan. Expected comes from source;
// Actual is meaningful only when Reached. This value grants no edit authority.
type ConditionFeedback struct {
	Present        bool
	ChoiceCount    uint8
	Choice         uint8
	ObservedChoice uint8
	CandidateMask  uint16
	Input          int64
	Expected       bool
	Reached        bool
	Actual         bool
}

// ConditionFeaturesInto keeps three independent caller-owned array regions:
// source [0:64], full intent [64:192], and observed condition [192:256].
// Adding feedback cannot move or rescale the original source/intent features.
// Inputs use bounded UTF-8 and fixed arrays; an error preserves the destination.
func ConditionFeaturesInto(source [SplitContextDim]byte, intent string, feedback ConditionFeedback,
	output *[FeatureDim]float32) error {
	if output == nil || len(intent) == 0 || len(intent) > InputMaxBytes || !utf8.ValidString(intent) {
		return errors.New("bounded valid intent and condition feature destination required")
	}
	if err := validateSemanticFields(source); err != nil {
		return err
	}
	if err := validateConditionFeedback(feedback); err != nil {
		return err
	}
	var candidate [FeatureDim]float32
	for i, value := range source {
		candidate[i] = float32(value)
	}
	for _, width := range [3]int{1, 2, 3} {
		for start := 0; start+width <= len(intent); start++ {
			bucket := start * 4 / len(intent)
			candidate[SplitContextDim+bucket*32+int(fullNgramHash(intent, start, width)%32)]++
		}
	}
	normalizeChannel(candidate[:SplitContextDim])
	normalizeChannel(candidate[SplitContextDim:192])
	conditionFields(feedback, candidate[192:])
	*output = candidate
	return nil
}

func validateConditionFeedback(f ConditionFeedback) error {
	if !f.Present {
		if f != (ConditionFeedback{}) {
			return errors.New("absent condition feedback must have zero fields")
		}
		return nil
	}
	if f.ChoiceCount == 0 || f.ChoiceCount > 16 || f.Choice >= f.ChoiceCount ||
		f.ObservedChoice >= f.ChoiceCount || uint32(f.CandidateMask)>>f.ChoiceCount != 0 {
		return errors.New("condition feedback must name choices within its declared candidate mask")
	}
	if !f.Reached && f.Actual {
		return errors.New("unreached conditions cannot carry a true observation")
	}
	return nil
}

func conditionFields(f ConditionFeedback, fields []float32) {
	if !f.Present {
		return
	}
	flag := func(index int, value bool) {
		if value {
			fields[index] = 1.0 / 8
		}
	}
	flag(0, true)
	flag(1, f.Choice == f.ObservedChoice)
	flag(2, f.Choice != f.ObservedChoice)
	flag(3, f.Reached)
	flag(4, !f.Reached)
	flag(5, !f.Expected)
	flag(6, f.Expected)
	flag(7, !f.Reached)
	flag(8, f.Reached && !f.Actual)
	flag(9, f.Reached && f.Actual)
	selected := f.CandidateMask>>f.Choice&1 != 0
	observed := f.CandidateMask>>f.ObservedChoice&1 != 0
	flag(10, !selected)
	flag(11, selected)
	flag(12, !observed)
	flag(13, observed)
	flag(14, f.Input < 0)
	flag(15, f.Input == 0)
	flag(16, f.Input > 0)
	// Eight exact two's-complement bytes, each divided by a power of two.
	// The full int64 is never converted through float32 or float64.
	bits := uint64(f.Input)
	for i := range 8 {
		fields[17+i] = float32(byte(bits>>uint(56-i*8))) / 2048
	}
	for i := range 16 {
		flag(25+i, f.CandidateMask>>i&1 != 0)
	}
	fields[41] = float32(f.ChoiceCount) / 128
	fields[42] = float32(f.Choice) / 128
	fields[43] = float32(f.ObservedChoice) / 128
}
