package decision

import (
	"encoding/hex"
	"errors"
	"math"
	"strings"
	"unicode/utf8"
)

const SemanticContextIntentFeatureVersion = "semantic_context_intent_v3"
const semanticContextPrefix = "gooo;sem64="
const semanticContextSeparator = ";intent: "
const semanticContextHeaderBytes = len(semanticContextPrefix) + SplitContextDim*2 + len(semanticContextSeparator)

// SemanticContextFeaturesInto exposes the frozen source-v3 feature contract
// without loading any weights. Invalid inputs preserve the caller's array.
func SemanticContextFeaturesInto(text string, output *[FeatureDim]float32) error {
	if output == nil || len(text) == 0 || len(text) > InputMaxBytes || !utf8.ValidString(text) {
		return errors.New("bounded valid semantic feature input and output required")
	}
	return buildSemanticContextIntentFeatures(text, output)
}

// EncodeSemanticContextInput serializes caller-provided structural facts. It
// grants no source authority; native callers bind/project their source first.
func EncodeSemanticContextInput(fields [SplitContextDim]byte, intent string) (string, error) {
	if err := validateSemanticFields(fields); err != nil {
		return "", err
	}
	if len(intent) == 0 || !utf8.ValidString(intent) || len(intent)+semanticContextHeaderBytes > InputMaxBytes {
		return "", errors.New("complete semantic context/intent exceeds the valid UTF-8 input bound")
	}
	var buffer [InputMaxBytes]byte
	n := copy(buffer[:], semanticContextPrefix)
	n += hex.Encode(buffer[n:n+SplitContextDim*2], fields[:])
	n += copy(buffer[n:], semanticContextSeparator)
	n += copy(buffer[n:], intent)
	return string(buffer[:n]), nil
}

func validateSemanticFields(fields [SplitContextDim]byte) error {
	kind, result := 0, 0
	for i, value := range fields {
		if value > 128 {
			return errors.New("semantic source feature exceeds byte contract")
		}
		if i < 7 {
			if value != 0 && value != 128 {
				return errors.New("semantic kind/result flags must be canonical")
			}
			if i < 5 {
				kind += int(value)
			} else {
				result += int(value)
			}
		}
	}
	if kind != 128 || result != 128 {
		return errors.New("one semantic choice kind and result kind required")
	}
	return nil
}

func decodeSemanticContext(text string) ([SplitContextDim]byte, string, error) {
	var fields [SplitContextDim]byte
	if len(text) <= semanticContextHeaderBytes || !strings.HasPrefix(text, semanticContextPrefix) ||
		text[len(semanticContextPrefix)+SplitContextDim*2:semanticContextHeaderBytes] != semanticContextSeparator {
		return fields, "", errors.New("canonical semantic source header and nonempty intent required")
	}
	for i := range fields {
		index := len(semanticContextPrefix) + i*2
		first, ok := semanticHex(text[index])
		if !ok {
			return fields, "", errors.New("lowercase semantic hex required")
		}
		second, ok := semanticHex(text[index+1])
		if !ok {
			return fields, "", errors.New("lowercase semantic hex required")
		}
		fields[i] = first*16 + second
	}
	if err := validateSemanticFields(fields); err != nil {
		return fields, "", err
	}
	return fields, text[semanticContextHeaderBytes:], nil
}

func semanticHex(value byte) (byte, bool) {
	if value >= '0' && value <= '9' {
		return value - '0', true
	}
	if value >= 'a' && value <= 'f' {
		return value - 'a' + 10, true
	}
	return 0, false
}

func buildSemanticContextIntentFeatures(text string, output *[FeatureDim]float32) error {
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
			bucket := start * 4 / len(intent)
			candidate[SplitContextDim+bucket*48+int(fullNgramHash(intent, start, width)%48)]++
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

// The source header stays bit-identical; only subsequent observed feedback is
// appended to the natural channel. A bounded caller checks the complete result
// before prediction so an oversized attempt can retain its full byte/hash record.
func SemanticContextFeedbackInput(original, feedback string) (string, error) {
	if len(original) > InputMaxBytes || !utf8.ValidString(original) || len(feedback) == 0 || len(feedback) > InputMaxBytes || !utf8.ValidString(feedback) {
		return "", errors.New("bounded valid semantic feedback text required")
	}
	if _, _, err := decodeSemanticContext(original); err != nil {
		return "", err
	}
	return original + "\n" + feedback, nil
}
