package decision

import (
	"math"
	"strings"
)

const SplitContextDim = 64

// Split feature channels share the fixed 256-value array, never hash into one
// another, and normalize separately before unit normalization of the union.
// Context is unpositioned; intent has four 48-value position buckets.
func buildSplitContextIntentFeatures(text string, output *[FeatureDim]float32) {
	clear(output[:])
	intent, context := text, ""
	if index := strings.LastIndex(text, "intent: "); index >= 0 {
		context, intent = text[:index], text[index+len("intent: "):]
	}
	for _, width := range [2]int{2, 3} {
		for start := 0; start+width <= len(context); start++ {
			output[fullNgramHash(context, start, width)%SplitContextDim]++
		}
		for start := 0; start+width <= len(intent); start++ {
			bucket := start * 4 / len(intent)
			index := SplitContextDim + bucket*48 + int(fullNgramHash(intent, start, width)%48)
			output[index]++
		}
	}
	active := 0
	if normalizeChannel(output[:SplitContextDim]) {
		active++
	}
	if normalizeChannel(output[SplitContextDim:]) {
		active++
	}
	if active > 0 {
		// A fixed presence scale keeps the other channel bit-identical when its
		// content changes; recomputing a rounded joint norm would couple them.
		scale := float32(1 / math.Sqrt(float64(active)))
		for i := range output {
			output[i] *= scale
		}
	}
}

func fullNgramHash(text string, start, width int) uint32 {
	hash := uint32(2166136261)
	for i := range width {
		value := text[start+i]
		if value >= 'A' && value <= 'Z' {
			value += 'a' - 'A'
		}
		hash = (hash ^ uint32(value)) * 16777619
	}
	return hash
}

func normalizeChannel(values []float32) bool {
	var squared float64
	for _, value := range values {
		squared += float64(value * value)
	}
	if squared == 0 {
		return false
	}
	scale := float32(1 / math.Sqrt(squared))
	for i := range values {
		values[i] *= scale
	}
	return true
}
