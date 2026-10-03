// Package orderjudge scores complete two-update candidates against intent.
// Its feature/model contracts are separate from the frozen V3/V4 local judges.
package orderjudge

import (
	"encoding/binary"
	"errors"
	"math"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-decision-runtime/orderfacts"
)

const (
	FeatureVersion = "ordered_source32_intent128_bilinear_v1"
	IntentDim      = 128
	SourceDim      = 32
	MaxInputBytes  = 512
	ParameterCount = IntentDim * SourceDim
)

// IntentFeatures retains every input byte in positional bigram/trigram counts.
// A final one-byte suffix contributes through its adjacent ngrams. One-byte
// inputs have a unigram feature. Invalid requests leave out unchanged.
func IntentFeatures(text string, out *[IntentDim]float32) error {
	if out == nil || len(text) == 0 || len(text) > MaxInputBytes || !utf8.ValidString(text) {
		return errors.New("complete valid intent of 1..512 bytes required")
	}
	var result [IntentDim]float32
	for _, width := range []int{2, 3} {
		for start := 0; start+width <= len(text); start++ {
			h := uint32(2166136261)
			for _, b := range []byte(text[start : start+width]) {
				h = (h ^ uint32(b)) * 16777619
			}
			result[(start*4/len(text))*32+int(h%32)]++
		}
	}
	if len(text) == 1 {
		result[int(text[0])%32] = 1
	}
	normalize(result[:])
	*out = result
	return nil
}

// SourceFeatures projects one exact descriptor with bounded small constants.
// Each operation occupies 16 slots: presence; three opcodes; three left and
// three right kinds; two signed constants/16; two negative and two zero flags.
func SourceFeatures(signature orderfacts.Signature, out *[SourceDim]float32) error {
	if out == nil || signature[0] != 1 || signature[1] != 2 {
		return errors.New("v1 two-update descriptor required")
	}
	for _, b := range signature[2:8] {
		if b != 0 {
			return errors.New("reserved header bytes must be zero")
		}
	}
	var result [SourceDim]float32
	for op := range 2 {
		record, values := signature[8+20*op:28+20*op], result[16*op:16*(op+1)]
		if record[0] < 1 || record[0] > 3 || record[3] != 0 {
			return errors.New("invalid operation record")
		}
		values[0], values[int(record[0])] = 1, 1
		for side := range 2 {
			kind := int(record[1+side])
			value := int64(binary.LittleEndian.Uint64(record[4+8*side : 12+8*side]))
			if kind < 1 || kind > 3 || kind != 3 && value != 0 || value < -16 || value > 16 {
				return errors.New("operand or constant outside model feature contract")
			}
			values[3+3*side+kind] = 1
			if kind == 3 {
				values[10+side] = float32(value) / 16
				if value < 0 {
					values[12+side] = 1
				}
				if value == 0 {
					values[14+side] = 1
				}
			}
		}
	}
	normalize(result[:])
	*out = result
	return nil
}

func normalize(values []float32) {
	var sum float64
	for _, v := range values {
		sum += float64(v) * float64(v)
	}
	if sum == 0 {
		return
	}
	scale := float32(1 / math.Sqrt(sum))
	for i := range values {
		values[i] = float32(values[i] * scale)
	}
}
