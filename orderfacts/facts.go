// Package orderfacts observes two sequential integer updates in a typed body.
// It is an experimental source descriptor, separate from released model ABIs.
package orderfacts

import (
	"encoding/binary"
	"errors"

	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

const Version = "gooo/two-update-order-facts/v1"
const Bytes = 48

// Signature has an eight-byte header and two twenty-byte operation records.
// Records contain opcode, two operand kinds, one reserved byte, and two exact
// little-endian int64 constants. A non-constant operand has a zero constant slot.
// Opcodes: add=1, subtract=2, multiply=3. Kinds: current local=1, input=2, int=3.
type Signature [Bytes]byte

// Alternatives validates the entire typed plan, then observes a root choice in
// declared option order. Source binding remains the compiler's responsibility.
// Other choices are left at the base structure; these are not joint-mask facts.
func Alternatives(plan pathplan.Plan, choiceID string) ([2]Signature, error) {
	var result [2]Signature
	if _, err := pathplan.Prepare(plan); err != nil {
		return result, err
	}
	for _, choice := range plan.Decisions {
		if choice.ID != choiceID || choice.Kind != pathplan.RootOrder {
			continue
		}
		for i, option := range choice.Options {
			if !Encode(plan.Base, option.Order, &result[i]) {
				return [2]Signature{}, errors.New("root choice is outside the two-update observation scope")
			}
		}
		return result, nil
	}
	return result, errors.New("declared root choice is required")
}

// Encode observes an already validated plan without allocating. It accepts only
// let v=input; v=<binary>; v=<binary>; return v, where leaves are v/input/int.
// It checks all accessed indices and the root permutation, but does not replace
// full plan validation. Failure leaves out unchanged; nil output is rejected.
func Encode(base bodyplan.Plan, order []int, out *Signature) bool {
	if out == nil || base.Schema != bodyplan.Schema || base.ResultType != decision.TypeInt ||
		len(base.Root) != 4 || len(order) != 4 || len(base.Statements) != 4 ||
		len(base.Expressions) == 0 || len(base.Expressions) > 128 {
		return false
	}
	var seen, original [4]bool
	for i, index := range order {
		root := base.Root[i]
		if index < 0 || index >= 4 || seen[index] || root < 0 || root >= 4 || original[root] {
			return false
		}
		seen[index], original[root] = true, true
		s := base.Statements[index]
		if s.Expr < 0 || s.Expr >= len(base.Expressions) || len(s.Then) != 0 || len(s.Else) != 0 {
			return false
		}
	}
	first, last := base.Statements[order[0]], base.Statements[order[3]]
	returned := base.Expressions[last.Expr]
	if first.Kind != bodyplan.StmtLet || first.Name == "" || base.Expressions[first.Expr].Kind != bodyplan.ExprInput ||
		last.Kind != bodyplan.StmtReturn || returned.Kind != bodyplan.ExprLocal || returned.Name != first.Name {
		return false
	}
	var result Signature
	result[0], result[1] = 1, 2
	for i, index := range order[1:3] {
		s := base.Statements[index]
		if s.Kind != bodyplan.StmtAssign || s.Name != first.Name || !operation(base, s.Expr, first.Name, result[8+i*20:28+i*20]) {
			return false
		}
	}
	*out = result
	return true
}

func operation(base bodyplan.Plan, index int, local string, out []byte) bool {
	e := base.Expressions[index]
	if e.Kind != bodyplan.ExprBinary || e.Left < 0 || e.Right < 0 || e.Left >= index || e.Right >= index {
		return false
	}
	var opcode byte
	switch e.Operation {
	case "add":
		opcode = 1
	case "subtract":
		opcode = 2
	case "multiply":
		opcode = 3
	default:
		return false
	}
	lk, lv := leaf(base.Expressions[e.Left], local)
	rk, rv := leaf(base.Expressions[e.Right], local)
	if lk == 0 || rk == 0 {
		return false
	}
	// int64 addition/multiplication commute, including Go's wraparound semantics.
	if opcode != 2 && (lk > rk || lk == rk && lv > rv) {
		lk, rk, lv, rv = rk, lk, rv, lv
	}
	out[0], out[1], out[2] = opcode, lk, rk
	binary.LittleEndian.PutUint64(out[4:12], uint64(lv))
	binary.LittleEndian.PutUint64(out[12:20], uint64(rv))
	return true
}

func leaf(e bodyplan.Expr, local string) (byte, int64) {
	switch e.Kind {
	case bodyplan.ExprLocal:
		if e.Name == local {
			return 1, 0
		}
	case bodyplan.ExprInput:
		return 2, 0
	case bodyplan.ExprInt:
		return 3, e.Int
	}
	return 0, 0
}
