package criteriainput

import (
	"errors"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
)

// Bound all caller-owned slices and text before marshaling or compiling.
func sourceBounds(input Input) error {
	base := input.Candidate.Base
	if len(base.Expressions) > 128 || len(base.Statements) > 128 || len(base.Root) > 128 {
		return errors.New("typed candidate arena exceeds source bound")
	}
	bytes := 0
	add := func(value string, limit int) bool {
		bytes += len(value)
		return len(value) <= limit && utf8.ValidString(value) && bytes <= MaxBytes
	}
	if !add(input.Candidate.Schema, 128) || !add(base.Name, 128) || !add(base.ID, 1024) || !add(base.Schema, 128) {
		return errors.New("candidate identity or name exceeds source bound")
	}
	for _, e := range base.Expressions {
		if len(e.Allowed) > 8 || !add(e.Kind, 32) || !add(e.Name, 128) || !add(e.Operation, 32) ||
			!add(e.HoleID, 128) || !add(e.Text, 512) || !add(e.Fallback, 32) {
			return errors.New("candidate expression exceeds source bound")
		}
		for _, value := range e.Allowed {
			if !add(value, 32) {
				return errors.New("candidate operation exceeds source bound")
			}
		}
	}
	for _, s := range base.Statements {
		if len(s.Then) > 128 || len(s.Else) > 128 || !add(s.Kind, 32) || !add(s.Name, 128) {
			return errors.New("candidate statement exceeds source bound")
		}
	}
	for _, c := range input.Candidate.Decisions {
		if len(c.Options) != 2 || !add(c.ID, 128) || !add(c.Kind, 32) || !add(c.Intent, 4096) || !add(c.Fallback, 128) {
			return errors.New("candidate choice exceeds source bound")
		}
		for _, o := range c.Options {
			if len(o.Order) > 128 || !add(o.Label, 128) || !add(o.Name, 128) {
				return errors.New("candidate option exceeds source bound")
			}
		}
	}
	literalBytes := 0
	for _, predicate := range input.Predicates {
		if !add(predicate.ID, 1024) || !add(predicate.ActivityID, 1024) || !validDigest(predicate.GraphSHA256, true) {
			return errors.New("predicate identity exceeds source bound")
		}
		if predicate.Inputs != [2]InputRole{{1, "original_input", "int64"}, {2, "candidate_result", "int64"}} {
			return errors.New("ordered scalar roles required")
		}
		if err := jointdecision.ValidateSourceGraphNodes(predicate.Nodes); err != nil {
			return err
		}
		for _, node := range predicate.Nodes {
			literalBytes += len(node.Literal)
			if node.FieldID != "" || literalBytes > MaxLiteralBytes || !add(node.Literal, 1024) {
				return errors.New("scalar predicate literals or fields exceed source bound")
			}
		}
	}
	return nil
}
