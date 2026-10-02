package jointdecision

import (
	"errors"
	"math"
	"strconv"
	"strings"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

const (
	ThreeSchema         = "gooo/tiny-three-choice-path-model/v1"
	ThreeFeatureVersion = "triple_semantic_context_v3_joint_v1"
	ThreeFeatureDim     = 768
	ThreeLabelCount     = 8
	ThreeInputMaxBytes  = 1600
	threePrefix         = "gooo;joint3|"
)

func EncodeThree(parts [3]string) (string, error) {
	var buffer [ThreeInputMaxBytes]byte
	n := copy(buffer[:], threePrefix)
	var scratch [decision.FeatureDim]float32
	for _, part := range parts {
		if err := decision.SemanticContextFeaturesInto(part, &scratch); err != nil {
			return "", err
		}
		length := strconv.AppendInt(buffer[n:n], int64(len(part)), 10)
		n += len(length)
		if n+1+len(part) > len(buffer) {
			return "", errors.New("complete three-choice input exceeds byte bound")
		}
		buffer[n] = ':'
		n++
		n += copy(buffer[n:], part)
	}
	return string(buffer[:n]), nil
}

func ThreeParts(text string) ([3]string, error) {
	var parts [3]string
	if len(text) > ThreeInputMaxBytes || !strings.HasPrefix(text, threePrefix) {
		return parts, errors.New("canonical bounded three-choice input required")
	}
	at := len(threePrefix)
	for i := range parts {
		var err error
		parts[i], at, err = readPart(text, at)
		if err != nil {
			return [3]string{}, err
		}
	}
	if at != len(text) {
		return [3]string{}, errors.New("three-choice input contains trailing bytes")
	}
	return parts, nil
}

// FeaturesIntoThree validates all full parts before committing caller storage.
func FeaturesIntoThree(text string, output *[ThreeFeatureDim]float32) error {
	if output == nil {
		return errors.New("three-choice feature output required")
	}
	parts, err := ThreeParts(text)
	if err != nil {
		return err
	}
	var candidate [ThreeFeatureDim]float32
	var part [decision.FeatureDim]float32
	scale := float32(1 / math.Sqrt(3))
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

// FeedbackThree preserves all original headers and intentions. Oversize full
// observations are retained so a caller can record a zero-prediction decline.
func FeedbackThree(text, feedback string) (string, error) {
	full, _, err := FeedbackThreeWithParts(text, feedback)
	return full, err
}

// FeedbackThreeWithParts also exposes complete attempted parts for auditing
// their byte/hash identity when an overflow prevents model inference.
func FeedbackThreeWithParts(text, feedback string) (string, [3]string, error) {
	parts, err := ThreeParts(text)
	if err != nil {
		return "", [3]string{}, err
	}
	for i, part := range parts {
		parts[i], err = decision.SemanticContextFeedbackInput(part, feedback)
		if err != nil {
			return "", [3]string{}, err
		}
	}
	var b strings.Builder
	b.WriteString(threePrefix)
	for _, part := range parts {
		b.WriteString(strconv.Itoa(len(part)))
		b.WriteByte(':')
		b.WriteString(part)
	}
	return b.String(), parts, nil
}
