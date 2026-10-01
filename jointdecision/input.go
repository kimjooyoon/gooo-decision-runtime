// Package jointdecision ranks four complete paths from two source-bound inputs.
package jointdecision

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime"
)

const (
	Schema         = "gooo/tiny-joint-path-model/v1"
	FeatureVersion = "paired_semantic_context_v3_joint_v1"
	FeatureDim     = 512
	HiddenDim      = 24
	LabelCount     = 4
	InputMaxBytes  = 1088
	inputPrefix    = "gooo;joint2|"
)

// Encode preserves both complete source-v3 UTF-8 inputs and their byte order.
func Encode(parts [2]string) (string, error) {
	var buffer [InputMaxBytes]byte
	n := copy(buffer[:], inputPrefix)
	var scratch [decision.FeatureDim]float32
	for _, part := range parts {
		if err := decision.SemanticContextFeaturesInto(part, &scratch); err != nil {
			return "", err
		}
		length := strconv.AppendInt(buffer[n:n], int64(len(part)), 10)
		n += len(length)
		if n+1+len(part) > len(buffer) {
			return "", errors.New("complete joint input exceeds byte bound")
		}
		buffer[n] = ':'
		n++
		n += copy(buffer[n:], part)
	}
	return string(buffer[:n]), nil
}

func readPart(text string, at int) (string, int, error) {
	start, n := at, 0
	for at < len(text) && text[at] >= '0' && text[at] <= '9' {
		if at-start >= 3 {
			return "", at, errors.New("joint byte length exceeds canonical extent")
		}
		n = n*10 + int(text[at]-'0')
		at++
	}
	if at == start || text[start] == '0' || at >= len(text) || text[at] != ':' || n > decision.InputMaxBytes || at+1+n > len(text) {
		return "", at, errors.New("canonical nonempty joint part byte length required")
	}
	at++
	return text[at : at+n], at + n, nil
}

// Parts returns immutable string views; feature validation belongs to FeaturesInto.
func Parts(text string) ([2]string, error) {
	var parts [2]string
	if len(text) > InputMaxBytes || !strings.HasPrefix(text, inputPrefix) {
		return parts, errors.New("canonical bounded joint input required")
	}
	at := len(inputPrefix)
	for i := range parts {
		var err error
		parts[i], at, err = readPart(text, at)
		if err != nil {
			return [2]string{}, err
		}
	}
	if at != len(text) {
		return [2]string{}, errors.New("joint input contains trailing bytes")
	}
	return parts, nil
}

// FeaturesInto concatenates unchanged source-v3 vectors, scaled by 1/sqrt(2).
// Both parts validate before committing any output; the valid path allocates zero.
func FeaturesInto(text string, output *[FeatureDim]float32) error {
	if output == nil {
		return errors.New("joint feature output required")
	}
	parts, err := Parts(text)
	if err != nil {
		return err
	}
	var candidate [FeatureDim]float32
	var part [decision.FeatureDim]float32
	scale := float32(1 / math.Sqrt(2))
	for i, text := range parts {
		if err = decision.SemanticContextFeaturesInto(text, &part); err != nil {
			return err
		}
		for j, v := range part {
			candidate[i*decision.FeatureDim+j] = v * scale
		}
	}
	*output = candidate
	return nil
}

// Feedback preserves each original source header and full natural intention.
// It returns a complete oversize observation; the caller may decline atomically.
func Feedback(text, feedback string) (string, error) {
	parts, err := Parts(text)
	if err != nil {
		return "", err
	}
	for i, part := range parts {
		parts[i], err = decision.SemanticContextFeedbackInput(part, feedback)
		if err != nil {
			return "", err
		}
	}
	var builder strings.Builder
	builder.WriteString(inputPrefix)
	for _, part := range parts {
		builder.WriteString(strconv.Itoa(len(part)))
		builder.WriteByte(':')
		builder.WriteString(part)
	}
	return builder.String(), nil
}
