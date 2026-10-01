package pathplan

import (
	"errors"
	"slices"

	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

const SourceFeatureSchema = "gooo/typed-source-features/v1"

// This scratch arena does not escape SourceFeatures. Borrowed strings/branch
// slices are read-only; fallback swaps modify only copied node/slice headers.
type sourceFeatureArena struct {
	resultType           decision.ValueType
	expressionCount      int
	statementCount       int
	rootCount            int
	expressions          [128]bodyplan.Expr
	statements           [128]bodyplan.Stmt
	roots                [128]int
	expressionReachable  [128]bool
	statementReachable   [128]bool
	declarations         [128]int
	declarationCount     int
	ambiguousDeclaration bool
}

// SourceFeatures projects immutable, fallback-normalized typed source facts.
// No intent, tests, model, seed or output values are read. Scope ambiguity and
// unreachable targets decline the optional representation, not compiler paths.
func (prepared *PreparedPlan) SourceFeatures(id string) ([decision.SplitContextDim]byte, error) {
	var fields [decision.SplitContextDim]byte
	if prepared == nil || prepared.fallback == nil {
		return fields, errors.New("prepared source feature snapshot required")
	}
	choiceIndex := -1
	for i, choice := range prepared.plan.Decisions {
		if choice.ID == id {
			choiceIndex = i
			break
		}
	}
	if choiceIndex < 0 {
		return fields, errors.New("source feature decision is undeclared")
	}
	var arena sourceFeatureArena
	arena.normalize(prepared.plan)
	for _, index := range arena.roots[:arena.rootCount] {
		arena.markStatement(index)
	}
	if arena.ambiguousDeclaration {
		return fields, errors.New("SEMANTIC_DECLARATION_POSITION_AMBIGUOUS")
	}
	for i := range arena.declarationCount {
		name := arena.statements[arena.declarations[i]].Name
		if name == "input" {
			return fields, errors.New("SEMANTIC_NAME_SHADOWS_INPUT")
		}
		for j := 0; j < i; j++ {
			if arena.statements[arena.declarations[j]].Name == name {
				return fields, errors.New("SEMANTIC_NAME_HAS_MULTIPLE_DECLARATIONS")
			}
		}
	}
	choice := prepared.plan.Decisions[choiceIndex]
	if err := arena.targetFields(choice, &fields); err != nil {
		return [decision.SplitContextDim]byte{}, err
	}
	arena.graphFields(&fields)
	for i, option := range choice.Options {
		facts := arena.optionFields(choice, option)
		copy(fields[44+i*10:54+i*10], facts[:])
	}
	return fields, nil
}

func (a *sourceFeatureArena) normalize(plan Plan) {
	a.resultType = plan.Base.ResultType
	a.expressionCount = copy(a.expressions[:], plan.Base.Expressions)
	a.statementCount = copy(a.statements[:], plan.Base.Statements)
	a.rootCount = copy(a.roots[:], plan.Base.Root)
	for _, choice := range plan.Decisions {
		for _, option := range choice.Options {
			if option.Label != choice.Fallback {
				continue
			}
			switch choice.Kind {
			case LocalReference:
				a.expressions[choice.Target].Name = option.Name
			case AssignmentTarget:
				a.statements[choice.Target].Name = option.Name
			case OperandOrder:
				if option.Reverse {
					e := &a.expressions[choice.Target]
					e.Left, e.Right = e.Right, e.Left
				}
			case BranchLayout:
				if option.Reverse {
					s := &a.statements[choice.Target]
					s.Then, s.Else = s.Else, s.Then
				}
			case RootOrder:
				a.rootCount = copy(a.roots[:], option.Order)
			}
		}
	}
}

func (a *sourceFeatureArena) markExpression(index int) {
	if a.expressionReachable[index] {
		return
	}
	a.expressionReachable[index] = true
	e := a.expressions[index]
	if e.Kind == bodyplan.ExprBinary || e.Kind == bodyplan.ExprHole {
		a.markExpression(e.Left)
		a.markExpression(e.Right)
	}
}

func (a *sourceFeatureArena) markStatement(index int) {
	if a.statementReachable[index] {
		kind := a.statements[index].Kind
		if kind == bodyplan.StmtLet || kind == bodyplan.StmtIf {
			a.ambiguousDeclaration = true
		}
		return
	}
	a.statementReachable[index] = true
	s := a.statements[index]
	if s.Kind == bodyplan.StmtLet {
		a.declarations[a.declarationCount] = index
		a.declarationCount++
	}
	a.markExpression(s.Expr)
	if s.Kind == bodyplan.StmtIf {
		for _, i := range s.Then {
			a.markStatement(i)
		}
		for _, i := range s.Else {
			a.markStatement(i)
		}
	}
}

func sourceFlag(value bool) byte {
	if value {
		return 128
	}
	return 0
}
func sourceCount(value int) byte { return byte(min(value, 16) * 8) }

func expressionOperation(e bodyplan.Expr) string {
	if e.Kind == bodyplan.ExprHole {
		return e.Fallback
	}
	return e.Operation
}

func (a *sourceFeatureArena) targetFields(choice Choice, fields *[decision.SplitContextDim]byte) error {
	for i, kind := range [5]string{LocalReference, AssignmentTarget, OperandOrder, BranchLayout, RootOrder} {
		fields[i] = sourceFlag(choice.Kind == kind)
	}
	fields[5], fields[6] = sourceFlag(a.resultType == decision.TypeInt), sourceFlag(a.resultType == decision.TypeBool)
	index := -1
	switch choice.Kind {
	case LocalReference, OperandOrder:
		if !a.expressionReachable[choice.Target] {
			return errors.New("TARGET_NOT_REACHABLE_IN_SOURCE_BODY")
		}
		index = choice.Target
	case AssignmentTarget, BranchLayout:
		if !a.statementReachable[choice.Target] {
			return errors.New("TARGET_NOT_REACHABLE_IN_SOURCE_BODY")
		}
		s := a.statements[choice.Target]
		index = s.Expr
		for i, kind := range [4]string{bodyplan.StmtLet, bodyplan.StmtAssign, bodyplan.StmtIf, bodyplan.StmtReturn} {
			fields[21+i] = sourceFlag(s.Kind == kind)
		}
	}
	if index < 0 {
		return nil
	}
	e := a.expressions[index]
	for i, kind := range [6]string{bodyplan.ExprInput, bodyplan.ExprInt, bodyplan.ExprBool, bodyplan.ExprLocal, bodyplan.ExprBinary, bodyplan.ExprHole} {
		fields[7+i] = sourceFlag(e.Kind == kind)
	}
	op := expressionOperation(e)
	for i, kind := range [8]string{"add", "subtract", "multiply", "less_than", "less_equal", "equal", "and", "or"} {
		fields[13+i] = sourceFlag(op == kind)
	}
	if e.Kind == bodyplan.ExprBinary || e.Kind == bodyplan.ExprHole {
		left, right := a.expressions[e.Left], a.expressions[e.Right]
		for i, kind := range [4]string{bodyplan.ExprInput, bodyplan.ExprLocal, bodyplan.ExprInt, bodyplan.ExprBool} {
			fields[25+i*2], fields[26+i*2] = sourceFlag(left.Kind == kind), sourceFlag(right.Kind == kind)
		}
	}
	fields[33] = sourceFlag(op == "add" || op == "multiply" || op == "equal" || op == "and" || op == "or")
	fields[34] = sourceFlag(op == "subtract" || op == "less_than" || op == "less_equal")
	return nil
}

func (a *sourceFeatureArena) graphFields(fields *[decision.SplitContextDim]byte) {
	var counts [9]int
	counts[2] = a.rootCount
	for i, e := range a.expressions[:a.expressionCount] {
		if !a.expressionReachable[i] {
			continue
		}
		counts[0]++
		if e.Kind == bodyplan.ExprLocal {
			counts[6]++
		}
		if e.Kind == bodyplan.ExprBinary || e.Kind == bodyplan.ExprHole {
			counts[7] += 2
		}
	}
	for i, s := range a.statements[:a.statementCount] {
		if !a.statementReachable[i] {
			continue
		}
		counts[1]++
		switch s.Kind {
		case bodyplan.StmtLet:
			counts[3]++
		case bodyplan.StmtAssign:
			counts[4]++
			for j := range a.declarationCount {
				if a.reads(s.Expr, a.statements[a.declarations[j]].Name) {
					counts[8]++
				}
			}
		case bodyplan.StmtIf:
			counts[5]++
		}
	}
	for i, count := range counts {
		fields[35+i] = sourceCount(count)
	}
}

// Direct local reads matter for assignment ordering. Following a copied local's
// initial definition here would invent a dependency on later writes to its input.
func (a *sourceFeatureArena) reads(index int, name string) bool {
	var visited [128]bool
	return a.readsNode(index, name, &visited)
}

func (a *sourceFeatureArena) readsNode(index int, name string, visited *[128]bool) bool {
	if visited[index] {
		return false
	}
	visited[index] = true
	e := a.expressions[index]
	if e.Kind == bodyplan.ExprLocal {
		return e.Name == name
	}
	if e.Kind == bodyplan.ExprBinary || e.Kind == bodyplan.ExprHole {
		return a.readsNode(e.Left, name, visited) || a.readsNode(e.Right, name, visited)
	}
	return false
}

func (a *sourceFeatureArena) declaration(name string) (int, int) {
	for i := range a.declarationCount {
		index := a.declarations[i]
		if a.statements[index].Name == name {
			return index, i
		}
	}
	return -1, -1
}

func (a *sourceFeatureArena) declarationProperty(index int, operation string, visited *[128]bool) bool {
	if visited[index] {
		return false
	}
	visited[index] = true
	e := a.expressions[index]
	if operation == "input" && e.Kind == bodyplan.ExprInput {
		return true
	}
	if expressionOperation(e) == operation {
		return true
	}
	if e.Kind == bodyplan.ExprBinary || e.Kind == bodyplan.ExprHole {
		return a.declarationProperty(e.Left, operation, visited) || a.declarationProperty(e.Right, operation, visited)
	}
	if e.Kind == bodyplan.ExprLocal {
		if d, _ := a.declaration(e.Name); d >= 0 {
			return a.declarationProperty(a.statements[d].Expr, operation, visited)
		}
	}
	return false
}

func (a *sourceFeatureArena) nameFacts(choice Choice, option Option) [10]byte {
	var fields [10]byte
	var name string
	if choice.Kind == AssignmentTarget {
		name = a.statements[choice.Target].Name
	} else {
		name = a.expressions[choice.Target].Name
	}
	fields[0], fields[9] = sourceFlag(option.Name == name), sourceFlag(option.Label == choice.Fallback)
	d, rank := a.declaration(option.Name)
	fields[1], fields[2], fields[3] = sourceFlag(d >= 0), sourceFlag(rank == 0), sourceFlag(rank >= 0 && rank == a.declarationCount-1)
	if d >= 0 {
		var visited [128]bool
		fields[4] = sourceFlag(a.declarationProperty(a.statements[d].Expr, "input", &visited))
		clear(visited[:])
		fields[5] = sourceFlag(a.declarationProperty(a.statements[d].Expr, "multiply", &visited))
	}
	reads := 0
	for i, e := range a.expressions[:a.expressionCount] {
		if a.expressionReachable[i] && e.Kind == bodyplan.ExprLocal && e.Name == option.Name {
			reads++
		}
	}
	fields[7] = sourceCount(reads)
	for i, s := range a.statements[:a.statementCount] {
		if a.statementReachable[i] {
			if s.Kind == bodyplan.StmtAssign && s.Name == option.Name {
				fields[6] = 128
			}
			if s.Kind == bodyplan.StmtReturn && a.reads(s.Expr, option.Name) {
				fields[8] = 128
			}
		}
	}
	return fields
}

func fallbackReverse(choice Choice) bool {
	for _, option := range choice.Options {
		if option.Label == choice.Fallback {
			return option.Reverse
		}
	}
	return false
}

func (a *sourceFeatureArena) operandFacts(choice Choice, option Option) [10]byte {
	var fields [10]byte
	e := a.expressions[choice.Target]
	reverse := option.Reverse != fallbackReverse(choice)
	if reverse {
		e.Left, e.Right = e.Right, e.Left
	}
	left, right := a.expressions[e.Left], a.expressions[e.Right]
	fields[0], fields[9] = sourceFlag(reverse), sourceFlag(!reverse)
	for i, kind := range [3]string{bodyplan.ExprInput, bodyplan.ExprLocal, bodyplan.ExprInt} {
		fields[1+i*2], fields[2+i*2] = sourceFlag(left.Kind == kind), sourceFlag(right.Kind == kind)
	}
	fields[7], fields[8] = sourceFlag(expressionOperation(left) == "add"), sourceFlag(expressionOperation(left) == "multiply")
	return fields
}

func (a *sourceFeatureArena) armOperation(sequence []int) string {
	for _, index := range sequence {
		s := a.statements[index]
		if s.Kind == bodyplan.StmtIf {
			if op := a.armOperation(s.Then); op != "" {
				return op
			}
			if op := a.armOperation(s.Else); op != "" {
				return op
			}
		}
		if op := expressionOperation(a.expressions[s.Expr]); op == "add" || op == "multiply" || op == "subtract" {
			return op
		}
	}
	return ""
}

func (a *sourceFeatureArena) branchFacts(choice Choice, option Option) [10]byte {
	var fields [10]byte
	s := a.statements[choice.Target]
	reverse := option.Reverse != fallbackReverse(choice)
	if reverse {
		s.Then, s.Else = s.Else, s.Then
	}
	then, otherwise := a.armOperation(s.Then), a.armOperation(s.Else)
	fields[0], fields[9] = sourceFlag(reverse), sourceFlag(!reverse)
	for i, op := range [3]string{"add", "multiply", "subtract"} {
		fields[1+i], fields[4+i] = sourceFlag(then == op), sourceFlag(otherwise == op)
	}
	e := a.expressions[s.Expr]
	if e.Kind == bodyplan.ExprBinary || e.Kind == bodyplan.ExprHole {
		fields[7] = sourceFlag(a.expressions[e.Left].Kind == bodyplan.ExprInput)
	}
	fields[8] = sourceFlag(expressionOperation(e) == "equal")
	return fields
}

func (a *sourceFeatureArena) rootFacts(option Option) [10]byte {
	var fields [10]byte
	fields[0] = sourceFlag(slices.Equal(option.Order, a.roots[:a.rootCount]))
	var writes [2]int
	count := 0
	for _, index := range option.Order {
		if a.statements[index].Kind == bodyplan.StmtAssign && count < 2 {
			writes[count] = index
			count++
		}
	}
	if a.declarationCount == 0 {
		return fields
	}
	first, last := a.statements[a.declarations[0]].Name, a.statements[a.declarations[a.declarationCount-1]].Name
	for i := range count {
		s := a.statements[writes[i]]
		start := 1 + i*4
		fields[start], fields[start+1] = sourceFlag(s.Name == first), sourceFlag(s.Name == last)
		fields[start+2], fields[start+3] = sourceFlag(a.reads(s.Expr, first)), sourceFlag(a.reads(s.Expr, last))
	}
	if count == 2 {
		firstWrite, lastWrite := a.statements[writes[0]], a.statements[writes[1]]
		fields[9] = sourceFlag(a.reads(firstWrite.Expr, lastWrite.Name) || a.reads(lastWrite.Expr, firstWrite.Name))
	}
	return fields
}

func (a *sourceFeatureArena) optionFields(choice Choice, option Option) [10]byte {
	switch choice.Kind {
	case LocalReference, AssignmentTarget:
		return a.nameFacts(choice, option)
	case OperandOrder:
		return a.operandFacts(choice, option)
	case BranchLayout:
		return a.branchFacts(choice, option)
	case RootOrder:
		return a.rootFacts(option)
	}
	return [10]byte{}
}
