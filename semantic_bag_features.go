package decision

import (
	"errors"
	"math"
	"unicode/utf8"
)

const SemanticContextBagFeatureVersion = "semantic_context_intent_bag_v4"

// SemanticContextBagFeaturesInto keeps the complete input and source header.
// V4 counts all 2/3-byte intent fragments without relative-position buckets.
// Clause order can alias; callers retain typed alternatives and finite checks.
func SemanticContextBagFeaturesInto(text string, output *[FeatureDim]float32) error {
	if output == nil || len(text) == 0 || len(text) > InputMaxBytes || !utf8.ValidString(text) {
		return errors.New("bounded valid semantic bag input and output required")
	}
	fields, intent, err := decodeSemanticContext(text)
	if err != nil {
		return err
	}
	var candidate [FeatureDim]float32
	for i, value := range fields {
		candidate[i] = float32(value)
	}
	for _, width := range [2]int{2, 3} {
		for start := 0; start+width <= len(intent); start++ {
			candidate[SplitContextDim+int(fullNgramHash(intent, start, width)%192)]++
		}
	}
	active := 0
	if normalizeChannel(candidate[:SplitContextDim]) {
		active++
	}
	if normalizeChannel(candidate[SplitContextDim:]) {
		active++
	}
	if active > 0 {
		scale := float32(1 / math.Sqrt(float64(active)))
		for i := range candidate {
			candidate[i] *= scale
		}
	}
	*output = candidate
	return nil
}
