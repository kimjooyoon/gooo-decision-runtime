package criteriainput

import (
	"encoding/json"
	"errors"

	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
)

// Node is a storage carrier, not a learned embedding. The vocabulary follows
// the validated source graph; kind/operator codes are one-based table indices.
// Parents, Condition and Guard address nodes within the owning Graph range.
type Node struct {
	LiteralStart uint32
	Parents      [2]uint16
	Condition    uint16
	Guard        uint16
	LiteralBytes uint16
	Input        uint8
	Kind         uint8
	Operator     uint8
	InputType    uint8
}

type Graph struct{ Start, Count uint16 }

var kinds = [...]string{"input", "literal", "read", "copy", "write", "return", "call", "call_parameter", "expression", "choice", "join", "guard", "guard_join"}
var operators = [...]string{"BOOL", "STRING", "INT", "+", "-", "!", "len", "int64", "*", "/", "%", "==", "!=", "<", "<=", ">", ">=", "&&", "||", "slice_bounds", "slice", "true", "false", "or"}
var inputTypes = [...]string{"bool", "int64", "string"}

func (p *Prepared) appendGraph(predicate Predicate) error {
	if !validDigest(predicate.GraphSHA256, true) {
		return errors.New("complete source graph digest required")
	}
	raw, err := json.Marshal(predicate.Nodes)
	if err != nil || digest(raw) != predicate.GraphSHA256 {
		return errors.New("source graph digest differs from canonical nodes")
	}
	if predicate.Inputs != [2]InputRole{{1, "original_input", "int64"}, {2, "candidate_result", "int64"}} {
		return errors.New("ordered original-input and candidate-result Integer roles required")
	}
	if err := jointdecision.ValidateSourceGraphNodes(predicate.Nodes); err != nil {
		return err
	}
	start := p.nodeCount
	var seen [2]bool
	for _, source := range predicate.Nodes {
		if source.FieldID != "" {
			return errors.New("record field graph is outside scalar source profile")
		}
		if source.Kind == "input" {
			if source.Input < 1 || source.Input > 2 || source.InputType != "int64" || seen[source.Input-1] {
				return errors.New("graph input differs from declared scalar role")
			}
			seen[source.Input-1] = true
		}
		if int(p.literalCount)+len(source.Literal) > MaxLiteralBytes {
			return errors.New("literal pool exceeds source bound")
		}
		node := Node{LiteralStart: p.literalCount, LiteralBytes: uint16(len(source.Literal)), Parents: source.Parents,
			Condition: source.Condition, Guard: source.Guard, Input: uint8(source.Input),
			Kind: code(kinds[:], source.Kind), Operator: code(operators[:], source.Operator), InputType: code(inputTypes[:], source.InputType)}
		copy(p.literals[p.literalCount:], source.Literal)
		p.literalCount += uint32(len(source.Literal))
		p.nodes[p.nodeCount] = node
		p.nodeCount++
	}
	if seen != [2]bool{true, true} {
		return errors.New("both source input roles must be present in the graph")
	}
	p.graphs[p.graphCount] = Graph{Start: start, Count: p.nodeCount - start}
	p.predicateIDs[p.graphCount] = predicate.ID
	p.graphCount++
	return nil
}

func code(vocabulary []string, value string) uint8 {
	if value == "" {
		return 0
	}
	for i, member := range vocabulary {
		if member == value {
			return uint8(i + 1)
		}
	}
	return 0
}

func name(vocabulary []string, index uint8) string {
	if index == 0 || int(index) > len(vocabulary) {
		return ""
	}
	return vocabulary[index-1]
}

func (n Node) KindName() string      { return name(kinds[:], n.Kind) }
func (n Node) OperatorName() string  { return name(operators[:], n.Operator) }
func (n Node) InputTypeName() string { return name(inputTypes[:], n.InputType) }

// Graph and Node return value copies. Literal returns an immutable string;
// readers cannot mutate the prepared arena through these accessors.
func (p *Prepared) Graph(index int) (Graph, bool) {
	if p == nil || index < 0 || index >= int(p.graphCount) {
		return Graph{}, false
	}
	return p.graphs[index], true
}

func (p *Prepared) Node(graph, index int) (Node, string, bool) {
	g, ok := p.Graph(graph)
	if !ok || index < 0 || index >= int(g.Count) {
		return Node{}, "", false
	}
	n := p.nodes[int(g.Start)+index]
	return n, string(p.literals[n.LiteralStart : n.LiteralStart+uint32(n.LiteralBytes)]), true
}
