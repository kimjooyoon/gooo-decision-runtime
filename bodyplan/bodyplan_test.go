package bodyplan

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime"
)

var evaluateBenchmarkSink Value

func BenchmarkEvaluateArithmeticPlan(b *testing.B) {
	program, err := Compile(arithmeticPlan(), nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value, evalErr := program.Evaluate(int64(i%128 - 64))
		if evalErr != nil {
			b.Fatal(evalErr)
		}
		evaluateBenchmarkSink = value
	}
	runtime.KeepAlive(program)
}

func TestCompileRendersTypedMultiNodeBodyAndEvaluatesBranches(t *testing.T) {
	program, err := Compile(arithmeticPlan(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(program.GoSource(), "func Adjusted(input int64) int64") || !strings.Contains(program.GoSource(), "var x = (input + int64(2))") {
		t.Fatalf("unexpected Go source:\n%s", program.GoSource())
	}
	if !strings.Contains(program.GoooSource(), "package bodyplan\nnamespace bodyplan") || !strings.Contains(program.GoooSource(), `entity Integer id "bodyplan://entity/integer"`) || !strings.Contains(program.GoooSource(), `activity Adjusted(Integer) -> Integer computes "let x = (input + 2)`) {
		t.Fatalf("unexpected Gooo source:\n%s", program.GoooSource())
	}
	for input, want := range map[int64]int64{-3: -2, 1: 8} {
		value, evalErr := program.Evaluate(input)
		if evalErr != nil {
			t.Fatalf("Evaluate(%d): %v", input, evalErr)
		}
		if value.Type != decision.TypeInt || value.Int != want {
			t.Errorf("Evaluate(%d) = %+v, want Int(%d)", input, value, want)
		}
	}
}

func TestCompileResolvesTypedHoleAndRequiresExactChoices(t *testing.T) {
	plan := comparisonHolePlan()
	holes, err := plan.Holes()
	if err != nil {
		t.Fatal(err)
	}
	if len(holes) != 1 || holes[0].ID != "cmp-1" || holes[0].LeftType != decision.TypeInt || holes[0].RightType != decision.TypeInt || holes[0].ResultType != decision.TypeBool || holes[0].Fallback != "less_than" {
		t.Fatalf("unexpected holes: %+v", holes)
	}
	for name, choices := range map[string]map[string]string{
		"missing":     nil,
		"extra":       {"cmp-1": "less_equal", "unknown": "equal"},
		"unsupported": {"cmp-1": "equal"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, compileErr := Compile(plan, choices); compileErr == nil {
				t.Fatalf("Compile accepted choices %v", choices)
			}
		})
	}
	program, err := Compile(plan, map[string]string{"cmp-1": "less_equal"})
	if err != nil {
		t.Fatal(err)
	}
	value, err := program.Evaluate(5)
	if err != nil || value.Type != decision.TypeBool || !value.Bool {
		t.Fatalf("Evaluate(5) = %+v, %v, want true Boolean", value, err)
	}
	if !strings.Contains(program.GoooSource(), `activity Compare(Integer) -> Boolean computes "return (input <= 5)"`) {
		t.Fatalf("resolved operation missing from Gooo source:\n%s", program.GoooSource())
	}
}

func TestCompileAndEvaluateUseGoInt64Overflow(t *testing.T) {
	plan := Plan{
		Schema: Schema, ID: "overflow", Name: "Wrap", ResultType: decision.TypeInt,
		Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: 1}, {Kind: ExprBinary, Operation: "add", Left: 0, Right: 1}},
		Statements:  []Stmt{{Kind: StmtReturn, Expr: 2}}, Root: []int{0},
	}
	program, err := Compile(plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(program.GoSource(), "input + int64(1)") {
		t.Fatalf("Go literal was not explicitly typed:\n%s", program.GoSource())
	}
	value, err := program.Evaluate(math.MaxInt64)
	if err != nil || value.Int != math.MinInt64 {
		t.Fatalf("overflow evaluation = %+v, %v, want MinInt64", value, err)
	}
	minimum := Plan{
		Schema: Schema, ID: "minimum-literal", Name: "Minimum", ResultType: decision.TypeInt,
		Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: math.MinInt64}, {Kind: ExprBinary, Operation: "add", Left: 0, Right: 1}},
		Statements:  []Stmt{{Kind: StmtReturn, Expr: 2}}, Root: []int{0},
	}
	minimumProgram, err := Compile(minimum, nil)
	if err != nil {
		t.Fatalf("Compile MinInt64 literal: %v", err)
	}
	if !strings.Contains(minimumProgram.GoSource(), "int64(-9223372036854775808)") {
		t.Fatalf("MinInt64 literal was not preserved as a typed constant:\n%s", minimumProgram.GoSource())
	}
}

func TestCompileSupportsBooleanResult(t *testing.T) {
	plan := Plan{
		Schema: Schema, ID: "negative-check", Name: "IsNegative", ResultType: decision.TypeBool,
		Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: 0}, {Kind: ExprBinary, Operation: "less_than", Left: 0, Right: 1}},
		Statements:  []Stmt{{Kind: StmtReturn, Expr: 2}}, Root: []int{0},
	}
	program, err := Compile(plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	value, err := program.Evaluate(-1)
	if err != nil || value.Type != decision.TypeBool || !value.Bool {
		t.Fatalf("Evaluate(-1) = %+v, %v, want true Boolean", value, err)
	}
	if !strings.Contains(program.GoSource(), "func IsNegative(input int64) bool") || !strings.Contains(program.GoooSource(), "-> Boolean computes") {
		t.Fatalf("Boolean result type missing from generated source:\n%s\n%s", program.GoSource(), program.GoooSource())
	}
}

func TestCompileAllowsSameNameInSeparateReturningBranchScopes(t *testing.T) {
	plan := Plan{
		Schema: Schema, ID: "branch-local-scopes", Name: "BranchLocal", ResultType: decision.TypeInt,
		Expressions: []Expr{
			{Kind: ExprInput, Name: "input"},
			{Kind: ExprInt, Int: 0},
			{Kind: ExprBinary, Operation: "less_than", Left: 0, Right: 1},
			{Kind: ExprInt, Int: 10},
			{Kind: ExprLocal, Name: "answer"},
			{Kind: ExprInt, Int: 1},
		},
		Statements: []Stmt{
			{Kind: StmtIf, Expr: 2, Then: []int{1, 2}, Else: []int{3, 4}},
			{Kind: StmtLet, Name: "answer", Expr: 3},
			{Kind: StmtReturn, Expr: 4},
			{Kind: StmtLet, Name: "answer", Expr: 5},
			{Kind: StmtReturn, Expr: 4},
		},
		Root: []int{0},
	}
	program, err := Compile(plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	for input, want := range map[int64]int64{-1: 10, 1: 1} {
		got, evalErr := program.Evaluate(input)
		if evalErr != nil || got.Type != decision.TypeInt || got.Int != want {
			t.Errorf("Evaluate(%d) = %+v, %v, want Int(%d)", input, got, evalErr, want)
		}
	}
}

func TestSlotEvaluationResolvesSharedLocalByLexicalScopeAndType(t *testing.T) {
	plan := Plan{
		Schema: Schema, ID: "branch-slot-types", Name: "BranchSlotTypes", ResultType: decision.TypeBool,
		Expressions: []Expr{
			{Kind: ExprInput, Name: "input"},
			{Kind: ExprInt, Int: 0},
			{Kind: ExprBool, Bool: true},
			{Kind: ExprLocal, Name: "answer"},
			{Kind: ExprBinary, Operation: "less_than", Left: 3, Right: 1},
			{Kind: ExprBinary, Operation: "equal", Left: 3, Right: 2},
			{Kind: ExprBinary, Operation: "less_than", Left: 0, Right: 1},
		},
		Statements: []Stmt{
			{Kind: StmtIf, Expr: 6, Then: []int{1, 2}, Else: []int{3, 4}},
			{Kind: StmtLet, Name: "answer", Expr: 0},
			{Kind: StmtReturn, Expr: 4},
			{Kind: StmtLet, Name: "answer", Expr: 2},
			{Kind: StmtReturn, Expr: 5},
		},
		Root: []int{0},
	}
	program, err := Compile(plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	thenScope, elseScope := program.branchScopes[0][0], program.branchScopes[0][1]
	exprCount := len(program.plan.Expressions)
	thenSlot := program.expressionSlots[int(thenScope)*exprCount+3]
	elseSlot := program.expressionSlots[int(elseScope)*exprCount+3]
	if thenSlot == 0 || elseSlot == 0 || thenSlot == elseSlot {
		t.Fatalf("shared local did not resolve to two distinct branch slots: then=%d else=%d", thenSlot, elseSlot)
	}
	for _, input := range []int64{-4, 4} {
		got, evalErr := program.Evaluate(input)
		if evalErr != nil || got.Type != decision.TypeBool || !got.Bool {
			t.Errorf("Evaluate(%d) = %+v, %v; want true Boolean", input, got, evalErr)
		}
	}
}

func TestSlotEvaluationBranchAssignmentsUpdateOuterLocal(t *testing.T) {
	plan := Plan{
		Schema: Schema, ID: "branch-assign-outer", Name: "BranchAssignOuter", ResultType: decision.TypeInt,
		Expressions: []Expr{
			{Kind: ExprInput, Name: "input"},
			{Kind: ExprInt, Int: 0},
			{Kind: ExprBinary, Operation: "less_than", Left: 0, Right: 1},
			{Kind: ExprLocal, Name: "answer"},
			{Kind: ExprInt, Int: 10},
			{Kind: ExprInt, Int: -10},
		},
		Statements: []Stmt{
			{Kind: StmtLet, Name: "answer", Expr: 0},
			{Kind: StmtIf, Expr: 2, Then: []int{2}, Else: []int{3}},
			{Kind: StmtAssign, Name: "answer", Expr: 4},
			{Kind: StmtAssign, Name: "answer", Expr: 5},
			{Kind: StmtReturn, Expr: 3},
		},
		Root: []int{0, 1, 4},
	}
	program, err := Compile(plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	for input, want := range map[int64]int64{-1: 10, 1: -10} {
		got, evalErr := program.Evaluate(input)
		if evalErr != nil || got.Type != decision.TypeInt || got.Int != want {
			t.Errorf("Evaluate(%d) = %+v, %v; want Int(%d)", input, got, evalErr, want)
		}
	}
}

func TestEvaluateBooleanOperationsAndTypedEquality(t *testing.T) {
	for _, test := range []struct {
		name      string
		operation string
		left      int
		right     int
		input     int64
		want      bool
	}{
		{name: "integer equality true", operation: "equal", left: 0, right: 1, input: 5, want: true},
		{name: "integer equality false", operation: "equal", left: 0, right: 1, input: 4, want: false},
		{name: "and", operation: "and", left: 2, right: 3, input: -1, want: true},
		{name: "or", operation: "or", left: 2, right: 3, input: -1, want: true},
		{name: "Boolean equality", operation: "equal", left: 2, right: 3, input: -1, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := operationPlan(test.operation, test.left, test.right)
			program, err := Compile(plan, nil)
			if err != nil {
				t.Fatal(err)
			}
			value, err := program.Evaluate(test.input)
			if err != nil || value.Type != decision.TypeBool || value.Bool != test.want {
				t.Fatalf("Evaluate(%d) = %+v, %v, want Bool(%t)", test.input, value, err, test.want)
			}
		})
	}
}

func TestEvaluateIsSafeForConcurrentCallers(t *testing.T) {
	program, err := Compile(arithmeticPlan(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := range 32 {
		group.Add(1)
		go func(value int64) {
			defer group.Done()
			got, evalErr := program.Evaluate(value)
			if evalErr != nil {
				t.Errorf("Evaluate(%d): %v", value, evalErr)
				return
			}
			want := (value+2)*3 - 1
			if value < 0 {
				want = (value+2)*3 + 1
			}
			if got.Type != decision.TypeInt || got.Int != want {
				t.Errorf("Evaluate(%d) = %+v, want Int(%d)", value, got, want)
			}
		}(int64(i - 16))
	}
	group.Wait()
}

func TestCompileRejectsInvalidScopesAndAssignments(t *testing.T) {
	t.Run("let cannot shadow visible local", func(t *testing.T) {
		plan := arithmeticPlan()
		plan.Statements[3] = Stmt{Kind: StmtLet, Name: "x", Expr: 9}
		if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "shadows") {
			t.Fatalf("Compile error = %v, want shadow rejection", err)
		}
	})
	t.Run("input is immutable", func(t *testing.T) {
		plan := Plan{
			Schema: Schema, ID: "input-assignment", Name: "Bad", ResultType: decision.TypeInt,
			Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: 1}},
			Statements:  []Stmt{{Kind: StmtAssign, Name: "input", Expr: 1}, {Kind: StmtReturn, Expr: 0}}, Root: []int{0, 1},
		}
		if _, err := Compile(plan, nil); err == nil {
			t.Fatal("Compile accepted assignment to input")
		}
	})
	t.Run("locals do not escape branch", func(t *testing.T) {
		plan := Plan{
			Schema: Schema, ID: "branch-scope", Name: "Bad", ResultType: decision.TypeInt,
			Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: 0}, {Kind: ExprBinary, Operation: "less_than", Left: 0, Right: 1}, {Kind: ExprInt, Int: 2}, {Kind: ExprLocal, Name: "branch_value"}},
			Statements:  []Stmt{{Kind: StmtIf, Expr: 2, Then: []int{1}}, {Kind: StmtLet, Name: "branch_value", Expr: 3}, {Kind: StmtReturn, Expr: 4}}, Root: []int{0, 2},
		}
		if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "not in scope") {
			t.Fatalf("Compile error = %v, want branch scope rejection", err)
		}
	})
}

func TestCompileRejectsUnusedDuplicateAndForwardArenaReferences(t *testing.T) {
	t.Run("unused expression", func(t *testing.T) {
		plan := Plan{
			Schema: Schema, ID: "unused-expression", Name: "Bad", ResultType: decision.TypeInt,
			Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: 1}},
			Statements:  []Stmt{{Kind: StmtReturn, Expr: 0}}, Root: []int{0},
		}
		if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "unused") {
			t.Fatalf("Compile error = %v, want unused expression rejection", err)
		}
	})
	t.Run("unused statement", func(t *testing.T) {
		plan := Plan{
			Schema: Schema, ID: "unused-statement", Name: "Bad", ResultType: decision.TypeInt,
			Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: 1}},
			Statements:  []Stmt{{Kind: StmtReturn, Expr: 0}, {Kind: StmtReturn, Expr: 1}}, Root: []int{0},
		}
		if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "unused") {
			t.Fatalf("Compile error = %v, want unused statement rejection", err)
		}
	})
	t.Run("duplicate statement reference", func(t *testing.T) {
		plan := Plan{
			Schema: Schema, ID: "duplicate-statement", Name: "Bad", ResultType: decision.TypeInt,
			Expressions: []Expr{{Kind: ExprInput, Name: "input"}},
			Statements:  []Stmt{{Kind: StmtReturn, Expr: 0}}, Root: []int{0, 0},
		}
		if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "more than once") {
			t.Fatalf("Compile error = %v, want duplicate reference rejection", err)
		}
	})
	t.Run("forward expression reference", func(t *testing.T) {
		plan := Plan{
			Schema: Schema, ID: "forward-expression", Name: "Bad", ResultType: decision.TypeInt,
			Expressions: []Expr{{Kind: ExprBinary, Operation: "add", Left: 0, Right: 1}, {Kind: ExprInput, Name: "input"}},
			Statements:  []Stmt{{Kind: StmtReturn, Expr: 0}}, Root: []int{0},
		}
		if _, err := Compile(plan, nil); err == nil {
			t.Fatal("Compile accepted a forward expression reference")
		}
	})
	t.Run("statement index out of range", func(t *testing.T) {
		plan := Plan{
			Schema: Schema, ID: "bad-statement-index", Name: "Bad", ResultType: decision.TypeInt,
			Expressions: []Expr{{Kind: ExprInput, Name: "input"}},
			Statements:  []Stmt{{Kind: StmtReturn, Expr: 0}}, Root: []int{1},
		}
		if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "out of range") {
			t.Fatalf("Compile error = %v, want out-of-range statement rejection", err)
		}
	})
	t.Run("unreachable after return", func(t *testing.T) {
		plan := Plan{
			Schema: Schema, ID: "after-return", Name: "Bad", ResultType: decision.TypeInt,
			Expressions: []Expr{{Kind: ExprInput, Name: "input"}},
			Statements:  []Stmt{{Kind: StmtReturn, Expr: 0}, {Kind: StmtReturn, Expr: 0}}, Root: []int{0, 1},
		}
		if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "unreachable") {
			t.Fatalf("Compile error = %v, want unreachable statement rejection", err)
		}
	})
}

func TestCompileRejectsNonterminatingPathsAndBadHoleCandidates(t *testing.T) {
	t.Run("nonterminating branch", func(t *testing.T) {
		plan := Plan{
			Schema: Schema, ID: "nonterminating", Name: "Bad", ResultType: decision.TypeInt,
			Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: 0}, {Kind: ExprBinary, Operation: "less_than", Left: 0, Right: 1}, {Kind: ExprInt, Int: 1}},
			Statements:  []Stmt{{Kind: StmtIf, Expr: 2, Then: []int{1}}, {Kind: StmtReturn, Expr: 3}}, Root: []int{0},
		}
		if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "every body path") {
			t.Fatalf("Compile error = %v, want nonterminating path rejection", err)
		}
	})
	t.Run("candidate output types differ", func(t *testing.T) {
		plan := comparisonHolePlan()
		plan.Expressions[2].Allowed = []string{"less_than", "add"}
		plan.Expressions[2].Fallback = "less_than"
		if _, err := plan.Holes(); err == nil {
			t.Fatal("Holes accepted candidates with different output types")
		}
	})
}

func TestCompileEnforcesArenaBoundsAndTextSafety(t *testing.T) {
	t.Run("expression count", func(t *testing.T) {
		plan := Plan{Schema: Schema, ID: "too-many-expressions", Name: "Bound", ResultType: decision.TypeInt}
		for range maxExprs + 1 {
			plan.Expressions = append(plan.Expressions, Expr{Kind: ExprInt, Int: 1})
		}
		if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "expression count") {
			t.Fatalf("Compile error = %v, want expression bound", err)
		}
	})
	t.Run("hole text byte limit", func(t *testing.T) {
		plan := comparisonHolePlan()
		plan.Expressions[2].Text = strings.Repeat("é", maxHoleText/2+1)
		if _, err := plan.Holes(); err == nil {
			t.Fatal("Holes accepted text exceeding the UTF-8 byte limit")
		}
	})
	t.Run("source injection identifier", func(t *testing.T) {
		plan := arithmeticPlan()
		plan.Statements[0].Name = "x; panic(1)"
		if _, err := Compile(plan, nil); err == nil {
			t.Fatal("Compile accepted a source-like identifier")
		}
	})
	t.Run("expression depth", func(t *testing.T) {
		plan := Plan{Schema: Schema, ID: "deep-expressions", Name: "Deep", ResultType: decision.TypeInt}
		plan.Expressions = append(plan.Expressions, Expr{Kind: ExprInput, Name: "input"}, Expr{Kind: ExprInt, Int: 1})
		last := 1
		for range maxDepth + 1 {
			plan.Expressions = append(plan.Expressions, Expr{Kind: ExprBinary, Operation: "add", Left: last, Right: 1})
			last = len(plan.Expressions) - 1
		}
		plan.Statements = []Stmt{{Kind: StmtReturn, Expr: last}}
		plan.Root = []int{0}
		if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "expression nesting") {
			t.Fatalf("Compile error = %v, want expression depth rejection", err)
		}
	})
	t.Run("statement depth", func(t *testing.T) {
		plan := Plan{Schema: Schema, ID: "deep-statements", Name: "Deep", ResultType: decision.TypeInt,
			Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt}, {Kind: ExprBinary, Operation: "less_than", Left: 0, Right: 1}}}
		for range maxDepth + 1 {
			plan.Statements = append(plan.Statements, Stmt{Kind: StmtIf, Expr: 2})
		}
		plan.Statements = append(plan.Statements, Stmt{Kind: StmtReturn, Expr: 0})
		for index := 0; index < maxDepth; index++ {
			plan.Statements[index].Then = []int{index + 1}
		}
		plan.Statements[maxDepth].Then = []int{maxDepth + 1}
		plan.Root = []int{0}
		if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "statement nesting") {
			t.Fatalf("Compile error = %v, want statement depth rejection", err)
		}
	})
}

func TestCompilePreflightsSharedDAGExpansionBeforeRecursiveWork(t *testing.T) {
	plan := Plan{
		Schema: Schema, ID: "shared-dag-expansion", Name: "SharedDAG", ResultType: decision.TypeInt,
		Expressions: []Expr{{Kind: ExprInput, Name: "input"}},
	}
	last := 0
	for range 16 {
		plan.Expressions = append(plan.Expressions, Expr{Kind: ExprBinary, Operation: "add", Left: last, Right: last})
		last = len(plan.Expressions) - 1
	}
	for index := range 127 {
		name := fmt.Sprintf("v%03d", index)
		plan.Statements = append(plan.Statements, Stmt{Kind: StmtLet, Name: name, Expr: last})
		plan.Root = append(plan.Root, index)
	}
	plan.Expressions = append(plan.Expressions, Expr{Kind: ExprLocal, Name: "v126"})
	plan.Statements = append(plan.Statements, Stmt{Kind: StmtReturn, Expr: len(plan.Expressions) - 1})
	plan.Root = append(plan.Root, len(plan.Statements)-1)

	if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "expanded Go source exceeds the 128 KiB") {
		t.Fatalf("Compile error = %v, want the shared-DAG source budget to reject before recursive work", err)
	}
}

func TestCompileAcceptsSharedDAGNearSourceBudget(t *testing.T) {
	plan := Plan{
		Schema: Schema, ID: "shared-dag-near-budget", Name: "NearBudget", ResultType: decision.TypeInt,
		Expressions: []Expr{{Kind: ExprInput, Name: "input"}},
	}
	last := 0
	for range 13 {
		plan.Expressions = append(plan.Expressions, Expr{Kind: ExprBinary, Operation: "add", Left: last, Right: last})
		last = len(plan.Expressions) - 1
	}
	plan.Statements = []Stmt{{Kind: StmtReturn, Expr: last}}
	plan.Root = []int{0}

	program, err := Compile(plan, nil)
	if err != nil {
		t.Fatalf("Compile near-budget shared DAG: %v", err)
	}
	if got := len(program.GoSource()); got < 75<<10 || got >= maxGeneratedSourceBytes {
		t.Fatalf("generated Go source length = %d, want 75 KiB <= length < %d", got, maxGeneratedSourceBytes)
	}
	if got := len(program.GoooSource()); got >= maxGeneratedSourceBytes {
		t.Fatalf("generated Gooo source length = %d, want < %d", got, maxGeneratedSourceBytes)
	}
}

func TestCompileBoundsPlanInputBeforeIdentifierValidation(t *testing.T) {
	plan := Plan{
		Schema: Schema, ID: "oversized-plan", Name: strings.Repeat("A", 20<<10), ResultType: decision.TypeInt,
		Expressions: []Expr{{Kind: ExprInput, Name: "input"}},
		Statements:  []Stmt{{Kind: StmtReturn, Expr: 0}}, Root: []int{0},
	}
	if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "plan input exceeds the 64 KiB") {
		t.Fatalf("Compile error = %v, want bounded input rejection before identifier scanning", err)
	}
}

func arithmeticPlan() Plan {
	return Plan{
		Schema: Schema, ID: "arithmetic-branch", Name: "Adjusted", ResultType: decision.TypeInt,
		Expressions: []Expr{
			{Kind: ExprInput, Name: "input"},
			{Kind: ExprInt, Int: 2},
			{Kind: ExprBinary, Operation: "add", Left: 0, Right: 1},
			{Kind: ExprLocal, Name: "x"},
			{Kind: ExprInt, Int: 3},
			{Kind: ExprBinary, Operation: "multiply", Left: 3, Right: 4},
			{Kind: ExprInt, Int: 0},
			{Kind: ExprBinary, Operation: "less_than", Left: 0, Right: 6},
			{Kind: ExprInt, Int: 1},
			{Kind: ExprBinary, Operation: "add", Left: 3, Right: 8},
			{Kind: ExprBinary, Operation: "subtract", Left: 3, Right: 8},
		},
		Statements: []Stmt{
			{Kind: StmtLet, Name: "x", Expr: 2},
			{Kind: StmtAssign, Name: "x", Expr: 5},
			{Kind: StmtIf, Expr: 7, Then: []int{3}, Else: []int{4}},
			{Kind: StmtAssign, Name: "x", Expr: 9},
			{Kind: StmtAssign, Name: "x", Expr: 10},
			{Kind: StmtReturn, Expr: 3},
		},
		Root: []int{0, 1, 2, 5},
	}
}

func comparisonHolePlan() Plan {
	return Plan{
		Schema: Schema, ID: "comparison-hole", Name: "Compare", ResultType: decision.TypeBool,
		Expressions: []Expr{
			{Kind: ExprInput, Name: "input"},
			{Kind: ExprInt, Int: 5},
			{Kind: ExprHole, Left: 0, Right: 1, HoleID: "cmp-1", Text: "compare the input with five", Allowed: []string{"less_than", "less_equal"}, Fallback: "less_than"},
		},
		Statements: []Stmt{{Kind: StmtReturn, Expr: 2}}, Root: []int{0},
	}
}

func operationPlan(operation string, left, right int) Plan {
	expressions := []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: 5}, {Kind: ExprBinary, Operation: "less_than", Left: 0, Right: 1}}
	if operation == "equal" && left == 0 && right == 1 {
		expressions[2] = Expr{Kind: ExprBinary, Operation: operation, Left: left, Right: right}
		return Plan{Schema: Schema, ID: "integer-equality", Name: "Compare", ResultType: decision.TypeBool, Expressions: expressions, Statements: []Stmt{{Kind: StmtReturn, Expr: 2}}, Root: []int{0}}
	}
	boolean := true
	if operation == "or" {
		boolean = false
	}
	expressions = append(expressions, Expr{Kind: ExprBool, Bool: boolean})
	expressions = append(expressions, Expr{Kind: ExprBinary, Operation: operation, Left: left, Right: right})
	return Plan{Schema: Schema, ID: "boolean-operation", Name: "Check", ResultType: decision.TypeBool, Expressions: expressions, Statements: []Stmt{{Kind: StmtReturn, Expr: 4}}, Root: []int{0}}
}
