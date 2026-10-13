package criteriainput

import "github.com/kimjooyoon/gooo-decision-runtime/jointdecision"

// Each predicate has six independently normalized channels: node kind [0:16],
// operator [16:48], input-role flow [48:80], ordered structure [80:208], literal
// spelling [208:240], and source return fingerprints [240:256]. All prepared
// nodes participate; this is a source representation, not path execution.
func (p *Prepared) graphFeatures(graph Graph, output *[PredicateFeatureDim]float32) {
	var hashes [jointdecision.RecordGraphNodeLimit]uint64
	var valueRoles, controlRoles [jointdecision.RecordGraphNodeLimit]uint8
	for i, node := range p.nodes[graph.Start : graph.Start+graph.Count] {
		literal := string(p.literals[node.LiteralStart : node.LiteralStart+uint32(node.LiteralBytes)])
		hash := featureHashWord(featureHashStart, uint64(node.Kind))
		for _, code := range [3]uint8{node.Operator, node.Input, node.InputType} {
			hash = featureHashWord(hash, uint64(code))
		}
		hash = featureHashText(hash, literal)
		for edgeIndex, edge := range [4]uint16{node.Parents[0], node.Parents[1], node.Condition, node.Guard} {
			var parent uint64
			if edge != 0 {
				parent = hashes[edge-1]
				if edgeIndex < 2 {
					valueRoles[i] |= valueRoles[edge-1]
					controlRoles[i] |= controlRoles[edge-1]
				} else {
					controlRoles[i] |= valueRoles[edge-1] | controlRoles[edge-1]
				}
			}
			hash = featureHashWord(hash, parent)
		}
		if node.Input != 0 {
			valueRoles[i] |= 1 << (node.Input - 1)
		}
		hashes[i] = hash
		output[node.Kind-1]++
		if node.Operator != 0 {
			output[16+node.Operator-1]++
		}
		output[48+valueRoles[i]]++
		output[52+controlRoles[i]]++
		output[80+hash%128]++
		if node.LiteralBytes != 0 {
			output[208+featureHashText(featureHashStart, literal)%32]++
		}
		if node.KindName() == "return" {
			output[56+valueRoles[i]]++
			output[60+controlRoles[i]]++
			output[240+hash%16]++
		}
	}
	for _, channel := range [6][2]int{{0, 16}, {16, 48}, {48, 80}, {80, 208}, {208, 240}, {240, 256}} {
		featureNormalize(output[channel[0]:channel[1]])
	}
}
