package criteriadecision

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime/criteriainput"
)

// FeatureSHA256 binds the exact active float32 rows read by this model. It omits
// IDs and labels; source/plan digests retain their separate metadata bindings.
// It performs no prediction, source execution or feature preparation.
func FeatureSHA256(features *criteriainput.Features) (string, error) {
	if err := validateFeatures(features); err != nil {
		return "", err
	}
	return featureDigest(features), nil
}

func featureDigest(features *criteriainput.Features) string {
	hash := sha256.New()
	hash.Write([]byte(features.Schema))
	hash.Write([]byte{features.ChoiceCount, features.PredicateCount})
	var encoded [4]byte
	appendRow := func(values []float32) {
		for _, value := range values {
			binary.LittleEndian.PutUint32(encoded[:], math.Float32bits(value))
			hash.Write(encoded[:])
		}
	}
	for _, row := range features.Choices[:features.ChoiceCount] {
		appendRow(row.Values[:])
	}
	for _, row := range features.Predicates[:features.PredicateCount] {
		appendRow(row.Values[:])
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func validDigest(value string, prefixed bool) bool {
	if prefixed {
		if !strings.HasPrefix(value, "sha256:") {
			return false
		}
		value = strings.TrimPrefix(value, "sha256:")
	}
	if len(value) != sha256.Size*2 {
		return false
	}
	raw, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(raw) == value
}
