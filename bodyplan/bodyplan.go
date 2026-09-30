// Package bodyplan validates, renders, and interprets small typed expression
// bodies. Plans contain no source fragments: identifiers, operations, and
// control flow all come from closed typed fields.
package bodyplan

import (
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-decision-runtime"
)

const (
	Schema      = "gooo/typed-body-plan/v1"
	maxExprs    = 128
	maxStmts    = 128
	maxHoles    = 16
	maxDepth    = 16
	maxHoleText = 512

	maxPlanInputBytes       = 64 << 10
	maxGeneratedSourceBytes = 128 << 10
)

const (
	ExprInput  = "input"
	ExprInt    = "int"
	ExprBool   = "bool"
	ExprLocal  = "local"
	ExprBinary = "binary"
	ExprHole   = "hole"

	StmtLet    = "let"
	StmtAssign = "assign"
	StmtIf     = "if"
	StmtReturn = "return"
)

var operations = map[string]string{
	"add":        "+",
	"subtract":   "-",
	"multiply":   "*",
	"less_than":  "<",
	"less_equal": "<=",
	"equal":      "==",
	"and":        "&&",
	"or":         "||",
}

// Plan is a bounded arena of typed expressions and statements. Expression
// children refer to earlier Expressions entries; statement control flow uses
// indices into Statements.
type Plan struct {
	Schema      string             `json:"schema"`
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	ResultType  decision.ValueType `json:"result_type"`
	Expressions []Expr             `json:"expressions"`
	Statements  []Stmt             `json:"statements"`
	Root        []int              `json:"root"`
}

// Expr is one typed expression node. Hole nodes are binary operations whose
// operation is selected from Allowed by Compile.
type Expr struct {
	Kind      string   `json:"kind"`
	Name      string   `json:"name,omitempty"`
	Int       int64    `json:"int,omitempty"`
	Bool      bool     `json:"bool,omitempty"`
	Operation string   `json:"operation,omitempty"`
	Left      int      `json:"left,omitempty"`
	Right     int      `json:"right,omitempty"`
	HoleID    string   `json:"hole_id,omitempty"`
	Text      string   `json:"text,omitempty"`
	Allowed   []string `json:"allowed,omitempty"`
	Fallback  string   `json:"fallback,omitempty"`
}

// Stmt is a statement node. Then and Else contain statement arena indices and
// are meaningful only for if statements.
type Stmt struct {
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
	Expr int    `json:"expr,omitempty"`
	Then []int  `json:"then,omitempty"`
	Else []int  `json:"else,omitempty"`
}

// HoleSpec is the validated model-selection boundary for one operator hole.
// ResultType is shared by every Allowed operation for these operand types.
type HoleSpec struct {
	ID         string             `json:"id"`
	Text       string             `json:"text"`
	LeftType   decision.ValueType `json:"left_type"`
	RightType  decision.ValueType `json:"right_type"`
	ResultType decision.ValueType `json:"result_type"`
	Allowed    []string           `json:"allowed"`
	Fallback   string             `json:"fallback"`
}

// Value is the interpreter's closed runtime value representation.
type Value struct {
	Type decision.ValueType `json:"type"`
	Int  int64              `json:"int,omitempty"`
	Bool bool               `json:"bool,omitempty"`
}

// Program is an immutable compiled plan. Its internal slices and maps are
// private copies, so concurrent Evaluate calls do not share mutable state.
type Program struct {
	plan            Plan
	choices         map[string]string
	expressionSlots []uint8
	statementSlots  []uint8
	branchScopes    [][2]uint16
	slotCount       int
	goSource        string
	goooSource      string
	goooBody        string
}

// Holes validates the plan and returns unique typed operator holes in
// expression-arena order.
func (plan Plan) Holes() ([]HoleSpec, error) {
	validated, err := analyze(plan)
	if err != nil {
		return nil, err
	}
	holes := make([]HoleSpec, 0, len(validated.holes))
	for _, index := range validated.holeOrder {
		hole := validated.holes[plan.Expressions[index].HoleID]
		hole.Allowed = append([]string(nil), hole.Allowed...)
		holes = append(holes, hole)
	}
	return holes, nil
}

// Compile validates the entire plan, requires one allowed choice for every
// hole, and creates deterministic Go and Gooo source.
func Compile(plan Plan, choices map[string]string) (*Program, error) {
	validated, err := analyze(plan)
	if err != nil {
		return nil, err
	}
	if len(choices) != len(validated.holes) {
		return nil, errors.New("choices must name every hole exactly once")
	}
	ownedChoices := make(map[string]string, len(choices))
	for id, choice := range choices {
		hole, ok := validated.holes[id]
		if !ok {
			return nil, fmt.Errorf("choice names unknown hole %q", id)
		}
		if !contains(hole.Allowed, choice) {
			return nil, fmt.Errorf("choice %q is not allowed for hole %q", choice, id)
		}
		ownedChoices[id] = choice
	}
	for id := range validated.holes {
		if _, ok := ownedChoices[id]; !ok {
			return nil, fmt.Errorf("choice for hole %q is missing", id)
		}
	}

	ownedPlan := clonePlan(plan)
	goBody, err := renderSequence(ownedPlan, ownedChoices, ownedPlan.Root, renderGo, 0)
	if err != nil {
		return nil, err
	}
	if len(goBody) > maxGeneratedSourceBytes {
		return nil, errors.New("generated Go source exceeds the 128 KiB compilation budget")
	}
	goSource, err := format.Source([]byte("package generated\n\nfunc " + ownedPlan.Name + "(input int64) " + goType(ownedPlan.ResultType) + " {\n" + goBody + "}\n"))
	if err != nil {
		return nil, fmt.Errorf("generated Go source is invalid: %w", err)
	}
	if len(goSource) > maxGeneratedSourceBytes {
		return nil, errors.New("formatted Go source exceeds the 128 KiB compilation budget")
	}
	if err := checkGoSource(goSource); err != nil {
		return nil, fmt.Errorf("generated Go source failed type checking: %w", err)
	}
	goooBody, err := renderSequence(ownedPlan, ownedChoices, ownedPlan.Root, renderGooo, -1)
	if err != nil {
		return nil, err
	}
	resultEntity := "Integer"
	if ownedPlan.ResultType == decision.TypeBool {
		resultEntity = "Boolean"
	}
	goooSource := "package bodyplan\nnamespace bodyplan\n\n" +
		"entity Integer id \"bodyplan://entity/integer\"\n" +
		"entity Boolean id \"bodyplan://entity/boolean\"\n\n" +
		"activity " + ownedPlan.Name + "(Integer) -> " + resultEntity + " computes " + strconv.Quote(strings.TrimSuffix(goooBody, "\n")) + "\n"
	if len(goooSource) > maxGeneratedSourceBytes {
		return nil, errors.New("generated Gooo source exceeds the 128 KiB compilation budget")
	}
	return &Program{
		plan:            ownedPlan,
		choices:         ownedChoices,
		expressionSlots: validated.expressionSlots,
		statementSlots:  validated.statementSlots,
		branchScopes:    validated.branchScopes,
		slotCount:       validated.slotCount,
		goSource:        string(goSource),
		goooSource:      goooSource,
		goooBody:        strings.TrimSuffix(goooBody, "\n"),
	}, nil
}

// GoSource returns package generated source for the compiled body.
func (program Program) GoSource() string { return program.goSource }

// GoooSource returns the package bodyplan declaration and JSON-quoted body.
func (program Program) GoooSource() string { return program.goooSource }

// GoooBody returns the immutable compiler-owned body without replacing a
// caller's package, declarations or stable semantic activity identity.
func (program Program) GoooBody() string { return program.goooBody }

// Evaluate executes the immutable plan with Go int64 overflow and Boolean
// short-circuit semantics.
func (program Program) Evaluate(input int64) (Value, error) {
	if program.goSource == "" || program.goooSource == "" {
		return Value{}, errors.New("program is not compiled")
	}
	var slots [maxStmts]Value
	value, returned, err := executeSequence(&program, program.plan.Root, 0, input, &slots)
	if err != nil {
		return Value{}, err
	}
	if !returned || value.Type != program.plan.ResultType {
		return Value{}, errors.New("compiled body did not return its declared result type")
	}
	return value, nil
}

type validatedPlan struct {
	holes           map[string]HoleSpec
	holeOrder       []int
	usedExpr        []bool
	seenStmt        []bool
	expressionSlots []uint8
	statementSlots  []uint8
	branchScopes    [][2]uint16
	exprCount       int
	scopeCount      int
	slotCount       int
}

type typeScope struct {
	parent *typeScope
	id     int
	values map[string]localBinding
}

type localBinding struct {
	valueType decision.ValueType
	slot      int
}

func analyze(plan Plan) (*validatedPlan, error) {
	if err := preflightPlan(plan); err != nil {
		return nil, err
	}
	if plan.Schema != Schema {
		return nil, fmt.Errorf("plan schema must be %q", Schema)
	}
	if plan.ID == "" || !utf8.ValidString(plan.ID) || len(plan.ID) > 128 || hasControl(plan.ID) {
		return nil, errors.New("plan ID must be printable UTF-8 of at most 128 bytes")
	}
	if !validIdentifier(plan.Name) {
		return nil, errors.New("activity name must be a valid Go identifier")
	}
	if plan.ResultType != decision.TypeInt && plan.ResultType != decision.TypeBool {
		return nil, errors.New("result type must be Int or Bool")
	}
	if len(plan.Expressions) == 0 || len(plan.Expressions) > maxExprs {
		return nil, fmt.Errorf("expression count must be between 1 and %d", maxExprs)
	}
	if len(plan.Statements) == 0 || len(plan.Statements) > maxStmts {
		return nil, fmt.Errorf("statement count must be between 1 and %d", maxStmts)
	}
	if len(plan.Root) == 0 || len(plan.Root) > maxStmts {
		return nil, errors.New("root must contain at least one statement")
	}

	validated := &validatedPlan{
		holes:           make(map[string]HoleSpec),
		usedExpr:        make([]bool, len(plan.Expressions)),
		seenStmt:        make([]bool, len(plan.Statements)),
		expressionSlots: make([]uint8, len(plan.Expressions)),
		statementSlots:  make([]uint8, len(plan.Statements)),
		branchScopes:    make([][2]uint16, len(plan.Statements)),
		exprCount:       len(plan.Expressions),
		scopeCount:      1,
	}
	inputCount := 0
	seenHoleIDs := make(map[string]bool)
	for index, expression := range plan.Expressions {
		if err := validateExprShape(expression, index); err != nil {
			return nil, fmt.Errorf("expression %d: %w", index, err)
		}
		if expression.Kind == ExprInput {
			inputCount++
		}
		if expression.Kind == ExprHole {
			if !validHoleID(expression.HoleID) {
				return nil, fmt.Errorf("expression %d has invalid hole ID", index)
			}
			if seenHoleIDs[expression.HoleID] {
				return nil, fmt.Errorf("hole ID %q appears in more than one expression", expression.HoleID)
			}
			seenHoleIDs[expression.HoleID] = true
			if len(validated.holeOrder) >= maxHoles {
				return nil, fmt.Errorf("hole count exceeds %d", maxHoles)
			}
			validated.holeOrder = append(validated.holeOrder, index)
		}
	}
	if inputCount != 1 {
		return nil, errors.New("plan must contain exactly one Integer input named input")
	}
	for index, statement := range plan.Statements {
		if err := validateStatementShape(statement, index); err != nil {
			return nil, err
		}
	}
	if err := validateStatementTree(plan, validated, plan.Root, 0); err != nil {
		return nil, err
	}
	for index, seen := range validated.seenStmt {
		if !seen {
			return nil, fmt.Errorf("statement %d is unused or unreachable", index)
		}
	}
	rootScope := &typeScope{id: 0, values: map[string]localBinding{"input": {valueType: decision.TypeInt, slot: -1}}}
	terminates, err := validateSequence(plan, validated, plan.Root, rootScope, 0)
	if err != nil {
		return nil, err
	}
	if !terminates {
		return nil, errors.New("every body path must return a value")
	}
	for index, used := range validated.usedExpr {
		if !used {
			return nil, fmt.Errorf("expression %d is unused", index)
		}
	}
	return validated, nil
}

// preflightPlan applies shallow input and expanded-source budgets before any
// recursive scope/type validation or source rendering. Expression costs are
// memoized by arena index, then charged at each statement use so shared DAG
// children cannot hide exponential output growth.
func preflightPlan(plan Plan) error {
	if len(plan.Expressions) == 0 || len(plan.Expressions) > maxExprs {
		return fmt.Errorf("expression count must be between 1 and %d", maxExprs)
	}
	if len(plan.Statements) == 0 || len(plan.Statements) > maxStmts {
		return fmt.Errorf("statement count must be between 1 and %d", maxStmts)
	}
	if len(plan.Root) == 0 || len(plan.Root) > maxStmts {
		return errors.New("root must contain at least one statement")
	}

	inputBytes := uint64(512)
	addString := func(value string) {
		// Six bytes per source byte covers JSON control-character escaping,
		// with two more bytes for the surrounding quotes.
		inputBytes = cappedAdd(inputBytes, cappedAdd(cappedMultiply(uint64(len(value)), 6, maxPlanInputBytes), 2, maxPlanInputBytes), maxPlanInputBytes)
	}
	for _, value := range []string{plan.Schema, plan.ID, plan.Name, string(plan.ResultType)} {
		addString(value)
	}
	for _, expression := range plan.Expressions {
		inputBytes = cappedAdd(inputBytes, 192, maxPlanInputBytes)
		for _, value := range []string{expression.Kind, expression.Name, expression.Operation, expression.HoleID, expression.Text, expression.Fallback} {
			addString(value)
		}
		if len(expression.Allowed) > len(operations) {
			return errors.New("expression candidate list exceeds the supported operation count")
		}
		inputBytes = cappedAdd(inputBytes, cappedMultiply(uint64(len(expression.Allowed)), 4, maxPlanInputBytes), maxPlanInputBytes)
		for _, candidate := range expression.Allowed {
			addString(candidate)
		}
	}
	statementReferences := uint64(len(plan.Root))
	for _, statement := range plan.Statements {
		inputBytes = cappedAdd(inputBytes, 128, maxPlanInputBytes)
		addString(statement.Kind)
		addString(statement.Name)
		thenCount := uint64(len(statement.Then))
		elseCount := uint64(len(statement.Else))
		statementReferences = cappedAdd(statementReferences, cappedAdd(thenCount, elseCount, maxStmts), maxStmts)
		inputBytes = cappedAdd(inputBytes, cappedMultiply(cappedAdd(thenCount, elseCount, maxPlanInputBytes), 24, maxPlanInputBytes), maxPlanInputBytes)
	}
	inputBytes = cappedAdd(inputBytes, cappedMultiply(uint64(len(plan.Root)), 24, maxPlanInputBytes), maxPlanInputBytes)
	if statementReferences > maxStmts {
		return errors.New("statement references exceed the bounded plan size")
	}
	if inputBytes > maxPlanInputBytes {
		return errors.New("plan input exceeds the 64 KiB bounded-plan budget")
	}

	goExprBytes := make([]uint64, len(plan.Expressions))
	goooExprBytes := make([]uint64, len(plan.Expressions))
	for index, expression := range plan.Expressions {
		switch expression.Kind {
		case ExprInput:
			goExprBytes[index], goooExprBytes[index] = 5, 5
		case ExprInt:
			// The longest int64 decimal spelling is 20 bytes.
			goExprBytes[index], goooExprBytes[index] = 27, 20
		case ExprBool:
			goExprBytes[index], goooExprBytes[index] = 5, 5
		case ExprLocal:
			goExprBytes[index], goooExprBytes[index] = uint64(len(expression.Name)), uint64(len(expression.Name))
		case ExprBinary, ExprHole:
			if expression.Left < 0 || expression.Right < 0 || expression.Left >= index || expression.Right >= index {
				return fmt.Errorf("expression %d children must refer to earlier arena entries", index)
			}
			// Parentheses plus a spaced two-byte operator are the largest
			// spelling among the supported operations.
			goExprBytes[index] = cappedAdd(cappedAdd(goExprBytes[expression.Left], goExprBytes[expression.Right], maxGeneratedSourceBytes), 6, maxGeneratedSourceBytes)
			goooExprBytes[index] = cappedAdd(cappedAdd(goooExprBytes[expression.Left], goooExprBytes[expression.Right], maxGeneratedSourceBytes), 6, maxGeneratedSourceBytes)
		default:
			// Shape validation below will provide the specific error. A fixed
			// charge keeps malformed nodes within the same bounded preflight.
			goExprBytes[index], goooExprBytes[index] = 64, 64
		}
	}

	goBodyBytes := uint64(0)
	goooBodyBytes := uint64(0)
	for index, statement := range plan.Statements {
		if statement.Expr < 0 || statement.Expr >= len(plan.Expressions) {
			return fmt.Errorf("statement %d expression index %d is out of range", index, statement.Expr)
		}
		// This fixed charge covers statement syntax, braces, indentation,
		// and separators at every allowed nesting depth. Local names are
		// charged separately.
		overhead := cappedAdd(256, uint64(len(statement.Name)), maxGeneratedSourceBytes)
		goBodyBytes = cappedAdd(goBodyBytes, cappedAdd(overhead, goExprBytes[statement.Expr], maxGeneratedSourceBytes), maxGeneratedSourceBytes)
		goooBodyBytes = cappedAdd(goooBodyBytes, cappedAdd(overhead, goooExprBytes[statement.Expr], maxGeneratedSourceBytes), maxGeneratedSourceBytes)
	}
	goSourceBytes := cappedAdd(goBodyBytes, cappedAdd(256, uint64(len(plan.Name)), maxGeneratedSourceBytes), maxGeneratedSourceBytes)
	if goSourceBytes > maxGeneratedSourceBytes {
		return errors.New("expanded Go source exceeds the 128 KiB compilation budget")
	}
	// Gooo embeds the body as a quoted string. Rendered expression tokens and
	// identifiers are ASCII without quote or backslash bytes; only renderer
	// indentation tabs and line breaks need escaping. A statement tree of at
	// most 128 nodes and depth 16 can emit at most three lines per node.
	maxQuotedBodyEscapeBytes := uint64(maxStmts * 3 * (maxDepth + 2))
	quotedBodyBytes := cappedAdd(goooBodyBytes, maxQuotedBodyEscapeBytes, maxGeneratedSourceBytes)
	goooSourceBytes := cappedAdd(quotedBodyBytes, cappedAdd(256, uint64(len(plan.Name)), maxGeneratedSourceBytes), maxGeneratedSourceBytes)
	if goooSourceBytes > maxGeneratedSourceBytes {
		return errors.New("expanded Gooo source exceeds the 128 KiB compilation budget")
	}
	return nil
}

func cappedAdd(left, right, limit uint64) uint64 {
	if left > limit || right > limit || right > limit-left {
		return limit + 1
	}
	return left + right
}

func cappedMultiply(value, multiplier, limit uint64) uint64 {
	if value > limit || multiplier > limit || value != 0 && multiplier > limit/value {
		return limit + 1
	}
	return value * multiplier
}

func validateStatementTree(plan Plan, validated *validatedPlan, sequence []int, depth int) error {
	if depth > maxDepth {
		return fmt.Errorf("statement nesting exceeds %d", maxDepth)
	}
	for _, index := range sequence {
		if index < 0 || index >= len(plan.Statements) {
			return fmt.Errorf("statement index %d is out of range", index)
		}
		if validated.seenStmt[index] {
			return fmt.Errorf("statement %d is referenced more than once", index)
		}
		validated.seenStmt[index] = true
		statement := plan.Statements[index]
		if statement.Kind == StmtIf {
			if err := validateStatementTree(plan, validated, statement.Then, depth+1); err != nil {
				return err
			}
			if err := validateStatementTree(plan, validated, statement.Else, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSequence(plan Plan, validated *validatedPlan, sequence []int, scope *typeScope, depth int) (bool, error) {
	if depth > maxDepth {
		return false, fmt.Errorf("statement nesting exceeds %d", maxDepth)
	}
	for position, index := range sequence {
		statement := plan.Statements[index]
		terminates := false
		switch statement.Kind {
		case StmtLet:
			if !validLocalName(statement.Name) {
				return false, fmt.Errorf("statement %d has invalid let name", index)
			}
			if _, found := scope.lookup(statement.Name); found {
				return false, fmt.Errorf("let %q shadows a visible name", statement.Name)
			}
			expressionType, err := typeExpression(plan, validated, statement.Expr, scope, depth)
			if err != nil {
				return false, fmt.Errorf("statement %d: %w", index, err)
			}
			if validated.slotCount >= maxStmts || validated.slotCount >= 254 {
				return false, errors.New("local slot count exceeds fixed runtime frame")
			}
			slot := validated.slotCount
			validated.slotCount++
			validated.statementSlots[index] = uint8(slot + 1)
			scope.values[statement.Name] = localBinding{valueType: expressionType, slot: slot}
		case StmtAssign:
			if !validLocalName(statement.Name) || statement.Name == "input" {
				return false, fmt.Errorf("statement %d has invalid assignment target", index)
			}
			current, found := scope.lookup(statement.Name)
			if !found {
				return false, fmt.Errorf("assignment target %q is not in scope", statement.Name)
			}
			if current.slot < 0 || current.slot >= maxStmts || current.slot >= 254 {
				return false, fmt.Errorf("assignment target %q has invalid runtime slot", statement.Name)
			}
			expressionType, err := typeExpression(plan, validated, statement.Expr, scope, depth)
			if err != nil {
				return false, fmt.Errorf("statement %d: %w", index, err)
			}
			if current.valueType != expressionType {
				return false, fmt.Errorf("assignment to %q changes its type", statement.Name)
			}
			validated.statementSlots[index] = uint8(current.slot + 1)
		case StmtIf:
			conditionType, err := typeExpression(plan, validated, statement.Expr, scope, depth)
			if err != nil {
				return false, fmt.Errorf("statement %d: %w", index, err)
			}
			if conditionType != decision.TypeBool {
				return false, fmt.Errorf("if condition %d must have type Bool", index)
			}
			thenScope := newTypeScope(validated, scope)
			elseScope := newTypeScope(validated, scope)
			validated.branchScopes[index] = [2]uint16{uint16(thenScope.id), uint16(elseScope.id)}
			thenTerminates, err := validateSequence(plan, validated, statement.Then, thenScope, depth+1)
			if err != nil {
				return false, err
			}
			elseTerminates, err := validateSequence(plan, validated, statement.Else, elseScope, depth+1)
			if err != nil {
				return false, err
			}
			terminates = thenTerminates && elseTerminates
		case StmtReturn:
			resultType, err := typeExpression(plan, validated, statement.Expr, scope, depth)
			if err != nil {
				return false, fmt.Errorf("statement %d: %w", index, err)
			}
			if resultType != plan.ResultType {
				return false, fmt.Errorf("return statement %d has type %s, want %s", index, resultType, plan.ResultType)
			}
			terminates = true
		default:
			return false, fmt.Errorf("statement %d has unknown kind %q", index, statement.Kind)
		}
		if terminates && position != len(sequence)-1 {
			return false, fmt.Errorf("statement %d makes later statements unreachable", index)
		}
		if terminates {
			return true, nil
		}
	}
	return false, nil
}

func typeExpression(plan Plan, validated *validatedPlan, index int, scope *typeScope, depth int) (decision.ValueType, error) {
	if depth > maxDepth {
		return "", fmt.Errorf("expression nesting exceeds %d", maxDepth)
	}
	if index < 0 || index >= len(plan.Expressions) {
		return "", fmt.Errorf("expression index %d is out of range", index)
	}
	validated.usedExpr[index] = true
	expression := plan.Expressions[index]
	switch expression.Kind {
	case ExprInput:
		return decision.TypeInt, nil
	case ExprInt:
		return decision.TypeInt, nil
	case ExprBool:
		return decision.TypeBool, nil
	case ExprLocal:
		binding, found := scope.lookup(expression.Name)
		if !found {
			return "", fmt.Errorf("local %q is not in scope", expression.Name)
		}
		if binding.slot < 0 || binding.slot >= maxStmts || binding.slot >= 254 {
			return "", fmt.Errorf("local %q has invalid runtime slot", expression.Name)
		}
		if err := validated.recordExpressionSlot(scope.id, index, binding.slot); err != nil {
			return "", err
		}
		return binding.valueType, nil
	case ExprBinary, ExprHole:
		if expression.Left >= index || expression.Right >= index || expression.Left < 0 || expression.Right < 0 {
			return "", errors.New("expression children must refer to earlier arena entries")
		}
		leftType, err := typeExpression(plan, validated, expression.Left, scope, depth+1)
		if err != nil {
			return "", err
		}
		rightType, err := typeExpression(plan, validated, expression.Right, scope, depth+1)
		if err != nil {
			return "", err
		}
		operation := expression.Operation
		if expression.Kind == ExprHole {
			operation = expression.Fallback
		}
		ir, err := decision.BuildTypedBinary(operation, decision.Identifier{Name: "lhs", Type: leftType}, decision.Identifier{Name: "rhs", Type: rightType})
		if err != nil {
			return "", fmt.Errorf("invalid %s operation: %w", expression.Kind, err)
		}
		if expression.Kind == ExprHole {
			var commonResult decision.ValueType
			for _, candidate := range expression.Allowed {
				candidateIR, candidateErr := decision.BuildTypedBinary(candidate, decision.Identifier{Name: "lhs", Type: leftType}, decision.Identifier{Name: "rhs", Type: rightType})
				if candidateErr != nil {
					return "", fmt.Errorf("hole %q candidate %q is ill typed: %w", expression.HoleID, candidate, candidateErr)
				}
				if commonResult == "" {
					commonResult = candidateIR.ResultType
				} else if commonResult != candidateIR.ResultType {
					return "", fmt.Errorf("hole %q candidates do not share one result type", expression.HoleID)
				}
			}
			if commonResult != ir.ResultType {
				return "", fmt.Errorf("hole %q fallback result type is inconsistent", expression.HoleID)
			}
			spec := HoleSpec{ID: expression.HoleID, Text: expression.Text, LeftType: leftType, RightType: rightType, ResultType: commonResult, Allowed: append([]string(nil), expression.Allowed...), Fallback: expression.Fallback}
			if prior, exists := validated.holes[expression.HoleID]; exists {
				if !sameHole(prior, spec) {
					return "", fmt.Errorf("hole %q has context-dependent types", expression.HoleID)
				}
			} else {
				validated.holes[expression.HoleID] = spec
			}
		}
		return ir.ResultType, nil
	default:
		return "", fmt.Errorf("unknown expression kind %q", expression.Kind)
	}
}

func validateExprShape(expression Expr, index int) error {
	priorChild := func(child int) bool { return child >= 0 && child < index }
	switch expression.Kind {
	case ExprInput:
		if expression.Name != "input" || expression.Int != 0 || expression.Bool || expression.Operation != "" || expression.Left != 0 || expression.Right != 0 || expression.HoleID != "" || expression.Text != "" || len(expression.Allowed) != 0 || expression.Fallback != "" {
			return errors.New("input must be the Integer named input with no extra fields")
		}
	case ExprInt:
		if expression.Name != "" || expression.Bool || expression.Operation != "" || expression.Left != 0 || expression.Right != 0 || expression.HoleID != "" || expression.Text != "" || len(expression.Allowed) != 0 || expression.Fallback != "" {
			return errors.New("integer literal has irrelevant fields")
		}
	case ExprBool:
		if expression.Name != "" || expression.Int != 0 || expression.Operation != "" || expression.Left != 0 || expression.Right != 0 || expression.HoleID != "" || expression.Text != "" || len(expression.Allowed) != 0 || expression.Fallback != "" {
			return errors.New("Boolean literal has irrelevant fields")
		}
	case ExprLocal:
		if !validLocalName(expression.Name) || expression.Int != 0 || expression.Bool || expression.Operation != "" || expression.Left != 0 || expression.Right != 0 || expression.HoleID != "" || expression.Text != "" || len(expression.Allowed) != 0 || expression.Fallback != "" {
			return errors.New("local expression has invalid or irrelevant fields")
		}
	case ExprBinary:
		if _, ok := operations[expression.Operation]; !ok || !priorChild(expression.Left) || !priorChild(expression.Right) || expression.Name != "" || expression.Int != 0 || expression.Bool || expression.HoleID != "" || expression.Text != "" || len(expression.Allowed) != 0 || expression.Fallback != "" {
			return errors.New("binary expression has invalid operation, children, or extra fields")
		}
	case ExprHole:
		if !validHoleID(expression.HoleID) || len(expression.Text) == 0 || len(expression.Text) > maxHoleText || !utf8.ValidString(expression.Text) || !priorChild(expression.Left) || !priorChild(expression.Right) || expression.Name != "" || expression.Int != 0 || expression.Bool || expression.Operation != "" || expression.Fallback == "" || len(expression.Allowed) == 0 || len(expression.Allowed) > len(operations) {
			return errors.New("hole has invalid ID, text, candidates, children, or extra fields")
		}
		allowedSeen := make(map[string]bool, len(expression.Allowed))
		for _, candidate := range expression.Allowed {
			if _, ok := operations[candidate]; !ok || allowedSeen[candidate] {
				return errors.New("hole candidates must be unique supported operations")
			}
			allowedSeen[candidate] = true
		}
		if !allowedSeen[expression.Fallback] {
			return errors.New("hole fallback must be one of its allowed operations")
		}
	default:
		return fmt.Errorf("unknown expression kind %q", expression.Kind)
	}
	return nil
}

func validateStatementShape(statement Stmt, index int) error {
	switch statement.Kind {
	case StmtLet, StmtAssign:
		if len(statement.Then) != 0 || len(statement.Else) != 0 {
			return fmt.Errorf("statement %d has branch lists outside an if", index)
		}
	case StmtIf:
		if statement.Name != "" {
			return fmt.Errorf("if statement %d has a name", index)
		}
	case StmtReturn:
		if statement.Name != "" || len(statement.Then) != 0 || len(statement.Else) != 0 {
			return fmt.Errorf("return statement %d has irrelevant fields", index)
		}
	default:
		return fmt.Errorf("statement %d has unknown kind %q", index, statement.Kind)
	}
	return nil
}

func sameHole(left, right HoleSpec) bool {
	if left.ID != right.ID || left.Text != right.Text || left.LeftType != right.LeftType || left.RightType != right.RightType || left.ResultType != right.ResultType || left.Fallback != right.Fallback || len(left.Allowed) != len(right.Allowed) {
		return false
	}
	for i := range left.Allowed {
		if left.Allowed[i] != right.Allowed[i] {
			return false
		}
	}
	return true
}

func newTypeScope(validated *validatedPlan, parent *typeScope) *typeScope {
	scope := &typeScope{
		parent: parent,
		id:     validated.scopeCount,
		values: make(map[string]localBinding),
	}
	validated.scopeCount++
	validated.expressionSlots = append(validated.expressionSlots, make([]uint8, validated.exprCount)...)
	return scope
}

func (validated *validatedPlan) recordExpressionSlot(scopeID, expressionIndex, slot int) error {
	if scopeID < 0 || scopeID >= validated.scopeCount || expressionIndex < 0 || expressionIndex >= validated.exprCount || slot < 0 || slot >= 254 {
		return errors.New("local expression slot is out of range")
	}
	index := scopeID*validated.exprCount + expressionIndex
	encoded := uint8(slot + 1)
	if current := validated.expressionSlots[index]; current != 0 && current != encoded {
		return errors.New("expression resolves to different slots in one lexical scope")
	}
	validated.expressionSlots[index] = encoded
	return nil
}

func (scope *typeScope) lookup(name string) (localBinding, bool) {
	for current := scope; current != nil; current = current.parent {
		if binding, ok := current.values[name]; ok {
			return binding, true
		}
	}
	return localBinding{}, false
}

func validLocalName(name string) bool {
	return validIdentifier(name) && name != "input"
}

func validIdentifier(name string) bool {
	if !token.IsIdentifier(name) || name == "_" {
		return false
	}
	for index := 0; index < len(name); index++ {
		value := name[index]
		if index == 0 {
			if !((value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || value == '_') {
				return false
			}
			continue
		}
		if !((value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || (value >= '0' && value <= '9') || value == '_') {
			return false
		}
	}
	return true
}

func validHoleID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		b := id[i]
		if !((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '-' || b == '.') {
			return false
		}
	}
	return true
}

func hasControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func clonePlan(plan Plan) Plan {
	copyPlan := plan
	copyPlan.Expressions = append([]Expr(nil), plan.Expressions...)
	for index := range copyPlan.Expressions {
		copyPlan.Expressions[index].Allowed = append([]string(nil), plan.Expressions[index].Allowed...)
	}
	copyPlan.Statements = append([]Stmt(nil), plan.Statements...)
	for index := range copyPlan.Statements {
		copyPlan.Statements[index].Then = append([]int(nil), plan.Statements[index].Then...)
		copyPlan.Statements[index].Else = append([]int(nil), plan.Statements[index].Else...)
	}
	copyPlan.Root = append([]int(nil), plan.Root...)
	return copyPlan
}

func checkGoSource(source []byte) error {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "generated.go", source, parser.AllErrors)
	if err != nil {
		return err
	}
	_, err = (&types.Config{Importer: importer.Default()}).Check("generated", fileSet, []*ast.File{file}, nil)
	return err
}

type sourceDialect int

const (
	renderGo sourceDialect = iota
	renderGooo
)

func renderSequence(plan Plan, choices map[string]string, indices []int, dialect sourceDialect, depth int) (string, error) {
	var output strings.Builder
	for _, index := range indices {
		statement := plan.Statements[index]
		indent := strings.Repeat("\t", depth+1)
		switch statement.Kind {
		case StmtLet:
			expression, err := renderExpression(plan, choices, statement.Expr, dialect)
			if err != nil {
				return "", err
			}
			declaration := "var "
			if dialect == renderGooo {
				declaration = "let "
			}
			fmt.Fprintf(&output, "%s%s%s = %s\n", indent, declaration, statement.Name, expression)
		case StmtAssign:
			expression, err := renderExpression(plan, choices, statement.Expr, dialect)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&output, "%s%s = %s\n", indent, statement.Name, expression)
		case StmtIf:
			condition, err := renderExpression(plan, choices, statement.Expr, dialect)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&output, "%sif %s {\n", indent, condition)
			thenBody, err := renderSequence(plan, choices, statement.Then, dialect, depth+1)
			if err != nil {
				return "", err
			}
			output.WriteString(thenBody)
			fmt.Fprintf(&output, "%s}", indent)
			if len(statement.Else) > 0 {
				output.WriteString(" else {\n")
				elseBody, err := renderSequence(plan, choices, statement.Else, dialect, depth+1)
				if err != nil {
					return "", err
				}
				output.WriteString(elseBody)
				fmt.Fprintf(&output, "%s}\n", indent)
			} else {
				output.WriteByte('\n')
			}
		case StmtReturn:
			expression, err := renderExpression(plan, choices, statement.Expr, dialect)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&output, "%sreturn %s\n", indent, expression)
		default:
			return "", fmt.Errorf("cannot render statement kind %q", statement.Kind)
		}
	}
	return output.String(), nil
}

func renderExpression(plan Plan, choices map[string]string, index int, dialect sourceDialect) (string, error) {
	expression := plan.Expressions[index]
	switch expression.Kind {
	case ExprInput:
		return "input", nil
	case ExprInt:
		literal := strconv.FormatInt(expression.Int, 10)
		if dialect == renderGo {
			return "int64(" + literal + ")", nil
		}
		return literal, nil
	case ExprBool:
		return strconv.FormatBool(expression.Bool), nil
	case ExprLocal:
		return expression.Name, nil
	case ExprBinary, ExprHole:
		left, err := renderExpression(plan, choices, expression.Left, dialect)
		if err != nil {
			return "", err
		}
		right, err := renderExpression(plan, choices, expression.Right, dialect)
		if err != nil {
			return "", err
		}
		operation := expression.Operation
		if expression.Kind == ExprHole {
			operation = choices[expression.HoleID]
		}
		operator, ok := operations[operation]
		if !ok {
			return "", fmt.Errorf("unsupported operation %q", operation)
		}
		return "(" + left + " " + operator + " " + right + ")", nil
	default:
		return "", fmt.Errorf("cannot render expression kind %q", expression.Kind)
	}
}

func goType(valueType decision.ValueType) string {
	if valueType == decision.TypeBool {
		return "bool"
	}
	return "int64"
}

func evalExpression(program *Program, scopeID, index int, input int64, slots *[maxStmts]Value) (Value, error) {
	expression := program.plan.Expressions[index]
	switch expression.Kind {
	case ExprInput:
		return Value{Type: decision.TypeInt, Int: input}, nil
	case ExprLocal:
		if scopeID < 0 || index < 0 || index >= len(program.plan.Expressions) {
			return Value{}, errors.New("local expression context is out of range")
		}
		tableIndex := scopeID*len(program.plan.Expressions) + index
		if tableIndex < 0 || tableIndex >= len(program.expressionSlots) || program.expressionSlots[tableIndex] == 0 {
			return Value{}, fmt.Errorf("runtime local %q has no compiled slot", expression.Name)
		}
		slot := int(program.expressionSlots[tableIndex] - 1)
		if slot >= program.slotCount || slot >= len(slots) {
			return Value{}, fmt.Errorf("runtime local %q has invalid slot", expression.Name)
		}
		return slots[slot], nil
	case ExprInt:
		return Value{Type: decision.TypeInt, Int: expression.Int}, nil
	case ExprBool:
		return Value{Type: decision.TypeBool, Bool: expression.Bool}, nil
	case ExprBinary, ExprHole:
		left, err := evalExpression(program, scopeID, expression.Left, input, slots)
		if err != nil {
			return Value{}, err
		}
		operation := expression.Operation
		if expression.Kind == ExprHole {
			operation = program.choices[expression.HoleID]
		}
		if operation == "and" && !left.Bool {
			return Value{Type: decision.TypeBool, Bool: false}, nil
		}
		if operation == "or" && left.Bool {
			return Value{Type: decision.TypeBool, Bool: true}, nil
		}
		right, err := evalExpression(program, scopeID, expression.Right, input, slots)
		if err != nil {
			return Value{}, err
		}
		switch operation {
		case "add":
			return Value{Type: decision.TypeInt, Int: left.Int + right.Int}, nil
		case "subtract":
			return Value{Type: decision.TypeInt, Int: left.Int - right.Int}, nil
		case "multiply":
			return Value{Type: decision.TypeInt, Int: left.Int * right.Int}, nil
		case "less_than":
			return Value{Type: decision.TypeBool, Bool: left.Int < right.Int}, nil
		case "less_equal":
			return Value{Type: decision.TypeBool, Bool: left.Int <= right.Int}, nil
		case "equal":
			if left.Type == decision.TypeInt {
				return Value{Type: decision.TypeBool, Bool: left.Int == right.Int}, nil
			}
			return Value{Type: decision.TypeBool, Bool: left.Bool == right.Bool}, nil
		case "and":
			return Value{Type: decision.TypeBool, Bool: left.Bool && right.Bool}, nil
		case "or":
			return Value{Type: decision.TypeBool, Bool: left.Bool || right.Bool}, nil
		default:
			return Value{}, fmt.Errorf("unsupported operation %q", operation)
		}
	default:
		return Value{}, fmt.Errorf("unsupported expression kind %q", expression.Kind)
	}
}

func executeSequence(program *Program, indices []int, scopeID int, input int64, slots *[maxStmts]Value) (Value, bool, error) {
	for _, index := range indices {
		statement := program.plan.Statements[index]
		switch statement.Kind {
		case StmtLet:
			value, err := evalExpression(program, scopeID, statement.Expr, input, slots)
			if err != nil {
				return Value{}, false, err
			}
			if index >= len(program.statementSlots) || program.statementSlots[index] == 0 {
				return Value{}, false, fmt.Errorf("runtime let %q has no compiled slot", statement.Name)
			}
			slot := int(program.statementSlots[index] - 1)
			if slot >= program.slotCount || slot >= len(slots) {
				return Value{}, false, fmt.Errorf("runtime let %q has invalid slot", statement.Name)
			}
			slots[slot] = value
		case StmtAssign:
			value, err := evalExpression(program, scopeID, statement.Expr, input, slots)
			if err != nil {
				return Value{}, false, err
			}
			if index >= len(program.statementSlots) || program.statementSlots[index] == 0 {
				return Value{}, false, fmt.Errorf("runtime assignment target %q has no compiled slot", statement.Name)
			}
			slot := int(program.statementSlots[index] - 1)
			if slot >= program.slotCount || slot >= len(slots) {
				return Value{}, false, fmt.Errorf("runtime assignment target %q has invalid slot", statement.Name)
			}
			slots[slot] = value
		case StmtIf:
			condition, err := evalExpression(program, scopeID, statement.Expr, input, slots)
			if err != nil {
				return Value{}, false, err
			}
			if index >= len(program.branchScopes) {
				return Value{}, false, errors.New("runtime if has no compiled branch scopes")
			}
			branchIndex := 1
			branch := statement.Else
			if condition.Bool {
				branchIndex = 0
				branch = statement.Then
			}
			branchScopeID := int(program.branchScopes[index][branchIndex])
			value, returned, err := executeSequence(program, branch, branchScopeID, input, slots)
			if err != nil || returned {
				return value, returned, err
			}
		case StmtReturn:
			value, err := evalExpression(program, scopeID, statement.Expr, input, slots)
			return value, true, err
		default:
			return Value{}, false, fmt.Errorf("unsupported statement kind %q", statement.Kind)
		}
	}
	return Value{}, false, nil
}
