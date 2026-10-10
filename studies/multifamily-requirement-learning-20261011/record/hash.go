package record

import (
	"encoding/binary"
	"math"
)

func OrderedFeatureSHA(row [528]float32) string {
	var raw [2112]byte
	for i, v := range row {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(v))
	}
	return Hash(raw[:])
}
