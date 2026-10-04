package jointdecision

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

const RecordOriginSharedFeatureVersion = "triple_record_field_flow_v2_shared_v1"
const RecordOriginInputMaxBytes = 4096
const RecordOriginPartMaxBytes = 1024
const recordOriginPrefix = "gooo;record-flow3-v2|"

// Origins contains counts of unique source-derived ancestors for each ordered
// expression. The compiler owns their provenance; this SDK never executes cases.
// Slots: input same/other/scalar, literal, expression, copy same/other/scalar,
// write same/other/scalar, prior choice same/other, join, guard, read.
type RecordOriginChoice struct {
	RecordChoice
	Origins [2][16]uint16 `json:"origins"`
}

func EncodeRecordOriginThree(choices [3]RecordOriginChoice) (string, error) {
	var buffer [RecordOriginInputMaxBytes]byte
	n := copy(buffer[:], recordOriginPrefix)
	for _, choice := range choices {
		for _, value := range [4]string{choice.Field, choice.First, choice.Second, choice.Intent} {
			if !utf8.ValidString(value) {
				return "", errors.New("origin choice contains invalid UTF-8")
			}
		}
		raw, err := json.Marshal(choice)
		if err != nil {
			return "", err
		}
		var scratch [256]float32
		if err = recordOriginFeatures(string(raw), &scratch); err != nil {
			return "", err
		}
		n += len(strconv.AppendInt(buffer[n:n], int64(len(raw)), 10))
		if n+1+len(raw) > len(buffer) {
			return "", errors.New("complete origin input exceeds byte bound")
		}
		buffer[n] = ':'
		n++
		n += copy(buffer[n:], raw)
	}
	return string(buffer[:n]), nil
}

func RecordOriginThreeParts(text string) ([3]string, error) {
	var parts [3]string
	if len(text) > RecordOriginInputMaxBytes || !strings.HasPrefix(text, recordOriginPrefix) {
		return parts, errors.New("canonical bounded record origin input required")
	}
	at := len(recordOriginPrefix)
	for i := range parts {
		start, n := at, 0
		for at < len(text) && text[at] >= '0' && text[at] <= '9' {
			if at-start == 4 {
				return [3]string{}, errors.New("origin part length exceeds extent")
			}
			n = n*10 + int(text[at]-'0')
			at++
		}
		if at == start || text[start] == '0' || at >= len(text) || text[at] != ':' ||
			n > RecordOriginPartMaxBytes || at+1+n > len(text) {
			return [3]string{}, errors.New("complete canonical origin part required")
		}
		at++
		parts[i], at = text[at:at+n], at+n
	}
	if at != len(text) {
		return [3]string{}, errors.New("origin input contains trailing bytes")
	}
	return parts, nil
}

// FeaturesIntoRecordOriginThree keeps 256 slots per part: ordered expression
// structure 64, ordered origin counts 32, intent byte ngrams 160. Three channels
// normalize independently, then each part is scaled by 1/sqrt(3).
func FeaturesIntoRecordOriginThree(text string, output *[ThreeFeatureDim]float32) error {
	if output == nil {
		return errors.New("origin feature output required")
	}
	parts, err := RecordOriginThreeParts(text)
	if err != nil {
		return err
	}
	var candidate [ThreeFeatureDim]float32
	var local [256]float32
	for i, part := range parts {
		if err = recordOriginFeatures(part, &local); err != nil {
			return err
		}
		for j, value := range local {
			candidate[i*256+j] = value * float32(1/math.Sqrt(3))
		}
	}
	*output = candidate
	return nil
}

func recordOriginFeatures(text string, output *[256]float32) error {
	if len(text) == 0 || len(text) > RecordOriginPartMaxBytes || !utf8.ValidString(text) {
		return errors.New("complete origin choice exceeds UTF-8 byte bound")
	}
	var choice RecordOriginChoice
	if err := json.Unmarshal([]byte(text), &choice); err != nil {
		return err
	}
	raw, _ := json.Marshal(choice)
	if !bytes.Equal(raw, []byte(text)) || !token.IsIdentifier(choice.Field) || choice.Intent == "" {
		return errors.New("canonical complete origin field/alternatives/intent required")
	}
	var candidate [256]float32
	for i, expression := range [2]string{choice.First, choice.Second} {
		node, err := parser.ParseExpr(expression)
		if err != nil {
			return err
		}
		recordExpression(node, choice.Field, candidate[i*32:(i+1)*32])
		if err = recordOriginCounts(choice.Origins[i], candidate[64+i*16:80+i*16]); err != nil {
			return err
		}
	}
	recordOriginIntent(choice.Intent, &candidate)
	*output = candidate
	return nil
}

func recordOriginCounts(counts [16]uint16, output []float32) error {
	total := uint32(0)
	for i, count := range counts {
		if count > 512 {
			return errors.New("origin ancestor count exceeds graph bound")
		}
		total += uint32(count)
		output[i] = float32(count)
	}
	if total == 0 || total > 512 {
		return errors.New("nonempty bounded source ancestry required")
	}
	return nil
}

func recordOriginIntent(intent string, candidate *[256]float32) {
	for _, width := range [2]int{2, 3} {
		for start := 0; start+width <= len(intent); start++ {
			candidate[96+int(recordNgramHash(intent, start, width)%160)]++
		}
	}
	active := 0
	for _, channel := range [][]float32{candidate[:64], candidate[64:96], candidate[96:]} {
		if normalizeRecord(channel) {
			active++
		}
	}
	for i := range candidate {
		candidate[i] *= float32(1 / math.Sqrt(float64(active)))
	}
}
