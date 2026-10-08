package jointdecision

import (
	"errors"
	"math"
)

// FeaturesIntoRecordGraphThree retains 256 slots per field: two ordered source
// channels of 96 slots and 64 intent slots. Each source channel has 32 root-op,
// 32 reachable-op and 32 hashed relation slots. All seven channels normalize
// separately. Hash collisions remain possible; this is not a lossless encoding.
func FeaturesIntoRecordGraphThree(text string, output *[ThreeFeatureDim]float32) error {
	if output == nil {
		return errors.New("record graph feature output required")
	}
	input, err := DecodeRecordGraphThree(text)
	if err != nil {
		return err
	}
	var fingerprints [RecordGraphNodeLimit]uint64
	for i, node := range input.Nodes {
		fingerprints[i] = graphFingerprint(node, input.Nodes, &fingerprints)
	}
	var candidate [ThreeFeatureDim]float32
	for i, choice := range input.Choices {
		part := candidate[i*256 : (i+1)*256]
		for side, root := range choice.Roots {
			graphSourceFeatures(input.Nodes, &fingerprints, root, choice.FieldID, part[side*96:(side+1)*96])
		}
		for _, width := range [2]int{2, 3} {
			for start := 0; start+width <= len(choice.Intent); start++ {
				part[192+int(recordNgramHash(choice.Intent, start, width)%64)]++
			}
		}
		active := 0
		for channel := range 6 {
			if normalizeRecord(part[channel*32 : (channel+1)*32]) {
				active++
			}
		}
		if normalizeRecord(part[192:]) {
			active++
		}
		scale := float32(1 / math.Sqrt(float64(3*active)))
		for j := range part {
			part[j] *= scale
		}
	}
	*output = candidate
	return nil
}

func graphSourceFeatures(nodes []RecordGraphNode, fingerprints *[RecordGraphNodeLimit]uint64,
	root uint16, field string, output []float32) {
	semantic := root
	for graphAlias(nodes[semantic-1].Kind) {
		semantic = nodes[semantic-1].Parents[0]
	}
	output[graphOperationSlot(nodes[semantic-1], field)] = 1
	graphHashedFeature(output[64:], graphHashWord(graphHashText(graphHashStart, "root"), fingerprints[root-1]))
	var seen [RecordGraphNodeLimit]bool
	var stack [RecordGraphNodeLimit]uint16
	count := 0
	push := func(id uint16) {
		if id != 0 && !seen[id-1] {
			seen[id-1], stack[count] = true, id
			count++
		}
	}
	push(root)
	for count > 0 {
		count--
		id := stack[count]
		node := nodes[id-1]
		if !graphAlias(node.Kind) {
			output[32+graphOperationSlot(node, field)]++
			graphHashedFeature(output[64:], fingerprints[id-1])
		}
		for _, edge := range [4]uint16{node.Parents[0], node.Parents[1], node.Condition, node.Guard} {
			push(edge)
		}
	}
}

func graphAlias(kind string) bool {
	switch kind {
	case "read", "copy", "write", "return", "call", "call_parameter":
		return true
	}
	return false
}

func graphOperationSlot(n RecordGraphNode, field string) int {
	switch n.Kind {
	case "input":
		if n.FieldID == "" {
			return 0
		}
		if n.FieldID == field {
			return 1
		}
		return 2
	case "literal":
		if n.Operator == "INT" {
			return 3
		}
		if n.Operator == "STRING" {
			return 4
		}
		if n.Literal == "true" {
			return 5
		}
		return 6
	case "join":
		return 27
	case "choice":
		return 28
	case "guard":
		if n.Operator == "true" {
			return 29
		}
		return 30
	case "guard_join":
		return 31
	}
	op := n.Operator
	if n.Parents[1] == 0 && (op == "+" || op == "-") {
		op = "u" + op
	}
	for i, known := range [...]string{"+", "-", "*", "/", "%", "==", "!=", "<", "<=", ">", ">=", "&&", "||", "u+", "u-", "!", "len", "int64", "slice_bounds", "slice"} {
		if op == known {
			return 7 + i
		}
	}
	panic("validated graph contains an unsupported operation")
}

const graphHashStart = uint64(14695981039346656037)

// Value fingerprints omit local names, spans and node indices. Ordered parent
// fingerprints preserve operand and conditional-arm order. Pure copy wrappers
// reuse the value identity; a newly attached execution guard remains explicit.
func graphFingerprint(n RecordGraphNode, nodes []RecordGraphNode, hashes *[RecordGraphNodeLimit]uint64) uint64 {
	value := func(id uint16) uint64 {
		if id == 0 {
			return 0
		}
		return hashes[id-1]
	}
	if graphAlias(n.Kind) {
		parent := value(n.Parents[0])
		if n.Guard == 0 || n.Guard == nodes[n.Parents[0]-1].Guard {
			return parent
		}
		hash := graphHashWord(graphHashText(graphHashStart, "guarded-value"), parent)
		return graphHashWord(hash, value(n.Guard))
	}
	hash := graphHashText(graphHashStart, n.Kind)
	for _, text := range [4]string{n.Operator, n.Literal, n.FieldID, n.InputType} {
		hash = graphHashText(hash, text)
	}
	hash = graphHashWord(hash, uint64(n.Input))
	for _, edge := range [4]uint16{n.Parents[0], n.Parents[1], n.Condition, n.Guard} {
		hash = graphHashWord(hash, value(edge))
	}
	return hash
}

func graphHashText(hash uint64, text string) uint64 {
	hash = graphHashWord(hash, uint64(len(text)))
	for i := range len(text) {
		hash = (hash ^ uint64(text[i])) * 1099511628211
	}
	return hash
}

func graphHashWord(hash, word uint64) uint64 {
	for range 8 {
		hash = (hash ^ (word & 255)) * 1099511628211
		word >>= 8
	}
	return hash
}

func graphHashedFeature(output []float32, hash uint64) {
	output[hash&15]++
	output[16+((hash>>32)&15)]++
}
