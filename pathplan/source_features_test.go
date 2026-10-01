package pathplan

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

func TestSourceFeatureSnapshotRenameLiteralAndConcurrentIsolation(t *testing.T) {
	plan := interactingPlan()
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	before, err := prepared.SourceFeatures("reference")
	if err != nil {
		t.Fatal(err)
	}
	if before[0] != 128 || before[5] != 128 || before[44] != 128 || before[54] != 0 {
		t.Fatal("source reference/name facts differ", before)
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.ReplaceAll(strings.ReplaceAll(string(raw), "first", "alpha"), "second", "omega"))
	// Label/intent vocabulary stays fixed while local display names change.
	raw = []byte(strings.ReplaceAll(strings.ReplaceAll(string(raw), "reference_alpha", "reference_first"), "reference_omega", "reference_second"))
	var renamed Plan
	if err = json.Unmarshal(raw, &renamed); err != nil {
		t.Fatal(err)
	}
	for i := range renamed.Base.Expressions {
		if renamed.Base.Expressions[i].Kind == bodyplan.ExprInt {
			renamed.Base.Expressions[i].Int += 97
		}
	}
	other, err := Prepare(renamed)
	if err != nil {
		t.Fatal(err)
	}
	after, err := other.SourceFeatures("reference")
	if err != nil || before != after {
		t.Fatal("name/literal surface leaked into structural vector", err)
	}
	plan.Base.Expressions[4].Name = "unbound"
	plan.Base.Root[0] = 3
	var wait sync.WaitGroup
	for range 8 {
		wait.Go(func() {
			for range 20 {
				current, err := prepared.SourceFeatures("reference")
				if err != nil || current != before {
					t.Error("caller/concurrent mutation changed source snapshot", err)
				}
			}
		})
	}
	wait.Wait()
	if _, err = prepared.SourceFeatures("missing"); err == nil {
		t.Fatal("unknown decision accepted")
	}
	var absent *PreparedPlan
	if _, err = absent.SourceFeatures("reference"); err == nil {
		t.Fatal("nil snapshot accepted")
	}
}

func TestSourceFeatureReversedFallbackAndUnreachableTarget(t *testing.T) {
	plan := interactingPlan()
	plan.Decisions = plan.Decisions[:1]
	plan.Decisions[0].Fallback = "reference_second"
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	fields, err := prepared.SourceFeatures("reference")
	if err != nil || fields[44] != 0 || fields[54] != 128 || fields[53] != 0 || fields[63] != 128 {
		t.Fatal("fallback source basis lost", err)
	}
	plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: bodyplan.ExprLocal, Name: "first"})
	plan.Decisions[0].Target = len(plan.Base.Expressions) - 1
	if _, err = Prepare(plan); err == nil || !strings.Contains(err.Error(), "unused") {
		t.Fatal("unused target bypassed compiler validation", err)
	}
	var arena sourceFeatureArena
	arena.normalize(plan)
	for _, i := range arena.roots[:arena.rootCount] {
		arena.markStatement(i)
	}
	if err = arena.targetFields(plan.Decisions[0], &fields); err == nil || !strings.Contains(err.Error(), "NOT_REACHABLE") {
		t.Fatal("projection reachability defense differs", err)
	}
}

func structuralFixture(kind string) Plan {
	plan := interactingPlan()
	plan.Base.Expressions = []bodyplan.Expr{
		{Kind: "input", Name: "input"}, {Kind: "int", Int: 2},
		{Kind: "binary", Operation: "add", Left: 0, Right: 1},
		{Kind: "binary", Operation: "multiply", Left: 0, Right: 1},
		{Kind: "local", Name: "first"}, {Kind: "local", Name: "second"},
		{Kind: "binary", Operation: "add", Left: 4, Right: 5},
		{Kind: "binary", Operation: "less_equal", Left: 0, Right: 1},
		{Kind: "binary", Operation: "subtract", Left: 0, Right: 1},
		{Kind: "binary", Operation: "multiply", Left: 4, Right: 1},
	}
	plan.Base.Statements = []bodyplan.Stmt{{Kind: "let", Name: "first", Expr: 0},
		{Kind: "let", Name: "second", Expr: 3}, {Kind: "assign", Name: "first", Expr: 2},
		{Kind: "assign", Name: "second", Expr: 9}, {Kind: "return", Expr: 6},
		{Kind: "if", Expr: 7, Then: []int{2}, Else: []int{3}}}
	plan.Base.Root = []int{0, 1, 2, 3, 4}
	choice := Choice{ID: "structure", Kind: kind, Intent: "Assemble the declared structure."}
	switch kind {
	case AssignmentTarget:
		choice.Target, choice.Fallback = 2, "assign_first"
		choice.Options = []Option{{Label: "assign_first", Name: "first"}, {Label: "assign_second", Name: "second"}}
	case OperandOrder:
		choice.Target, choice.Fallback = 8, "layout_forward"
		choice.Options = []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}
		plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "binary", Operation: "add", Left: 8, Right: 6})
		plan.Base.Statements[4].Expr = 10
	case BranchLayout:
		choice.Target, choice.Fallback = 5, "layout_forward"
		choice.Options = []Option{{Label: "layout_forward"}, {Label: "layout_reverse", Reverse: true}}
		plan.Base.Root = []int{0, 1, 5, 4}
	case RootOrder:
		choice.Fallback = "schedule_forward"
		choice.Options = []Option{{Label: "schedule_forward", Order: []int{0, 1, 2, 3, 4}},
			{Label: "schedule_reverse", Order: []int{0, 1, 3, 2, 4}}}
	}
	plan.Decisions = []Choice{choice}
	return pruneStructuralFixture(plan)
}

// All compiler nodes must be used; retain and reindex this fixture's selected
// reachable graph before preparation. This is a test builder, not inference.
func pruneStructuralFixture(plan Plan) Plan {
	var expressions, statements [128]bool
	var expression func(int)
	expression = func(i int) {
		if expressions[i] {
			return
		}
		expressions[i] = true
		e := plan.Base.Expressions[i]
		if e.Kind == "binary" {
			expression(e.Left)
			expression(e.Right)
		}
	}
	var statement func(int)
	statement = func(i int) {
		if statements[i] {
			return
		}
		statements[i] = true
		s := plan.Base.Statements[i]
		expression(s.Expr)
		for _, j := range s.Then {
			statement(j)
		}
		for _, j := range s.Else {
			statement(j)
		}
	}
	for _, i := range plan.Base.Root {
		statement(i)
	}
	var exprMap, stmtMap [128]int
	var es []bodyplan.Expr
	for i, e := range plan.Base.Expressions {
		if !expressions[i] {
			continue
		}
		exprMap[i] = len(es)
		if e.Kind == "binary" {
			e.Left, e.Right = exprMap[e.Left], exprMap[e.Right]
		}
		es = append(es, e)
	}
	var ss []bodyplan.Stmt
	for i, s := range plan.Base.Statements {
		if !statements[i] {
			continue
		}
		stmtMap[i] = len(ss)
		s.Expr = exprMap[s.Expr]
		ss = append(ss, s)
	}
	remap := func(sequence []int) []int {
		result := make([]int, len(sequence))
		for i, n := range sequence {
			result[i] = stmtMap[n]
		}
		return result
	}
	for i := range ss {
		ss[i].Then, ss[i].Else = remap(ss[i].Then), remap(ss[i].Else)
	}
	plan.Base.Expressions, plan.Base.Statements, plan.Base.Root = es, ss, remap(plan.Base.Root)
	for i := range plan.Decisions {
		c := &plan.Decisions[i]
		switch c.Kind {
		case LocalReference, OperandOrder:
			c.Target = exprMap[c.Target]
		case AssignmentTarget, BranchLayout:
			c.Target = stmtMap[c.Target]
		case RootOrder:
			for j := range c.Options {
				c.Options[j].Order = remap(c.Options[j].Order)
			}
		}
	}
	return plan
}

func TestSourceFeatureEveryKindAndSourceRelativeAlternatives(t *testing.T) {
	for i, kind := range []string{AssignmentTarget, OperandOrder, BranchLayout, RootOrder} {
		t.Run(kind, func(t *testing.T) {
			plan := structuralFixture(kind)
			prepared, err := Prepare(plan)
			if err != nil {
				t.Fatal(err)
			}
			fields, err := prepared.SourceFeatures("structure")
			if err != nil || fields[i+1] != 128 || fields[5] != 128 || fields[35] == 0 {
				t.Fatal(fields, err)
			}
			switch kind {
			case AssignmentTarget:
				if fields[22] != 128 || fields[44] != 128 || fields[46] != 128 || fields[54] != 0 || fields[57] != 128 {
					t.Fatal(fields)
				}
			case OperandOrder:
				if fields[14] != 128 || fields[25] != 128 || fields[30] != 128 || fields[34] != 128 || fields[45] != 128 || fields[54] != 128 || fields[60] != 0 {
					t.Fatal(fields)
				}
			case BranchLayout:
				if fields[23] != 128 || fields[17] != 128 || fields[45] != 128 || fields[49] != 128 || fields[54] != 128 || fields[56] != 128 || fields[58] != 128 {
					t.Fatal(fields)
				}
			case RootOrder:
				if fields[44] != 128 || fields[54] != 0 || fields[45] != 128 || fields[50] != 128 || fields[51] != 128 || fields[53] != 128 {
					t.Fatal(fields)
				}
			}
			plan.Decisions[0].Fallback = plan.Decisions[0].Options[1].Label
			other, err := Prepare(plan)
			if err != nil {
				t.Fatal(err)
			}
			reversed, err := other.SourceFeatures("structure")
			if err != nil {
				t.Fatal(err)
			}
			if kind == OperandOrder || kind == BranchLayout {
				if reversed[44] != 128 || reversed[54] != 0 || reversed[53] != 0 || reversed[63] != 128 {
					t.Fatal("reversed source basis lost", reversed)
				}
			}
			if kind == RootOrder && (reversed[44] != 0 || reversed[54] != 128) {
				t.Fatal("absolute order source basis lost", reversed)
			}
			if n := testing.AllocsPerRun(100, func() {
				if _, e := prepared.SourceFeatures("structure"); e != nil {
					panic(e)
				}
			}); n != 0 {
				t.Fatal("source projection allocated", n)
			}
		})
	}
}

func TestSourceFeatureDuplicateNamesDeclineButDeterministicBodySurvives(t *testing.T) {
	plan := structuralFixture(BranchLayout)
	plan.Base.Statements[2] = bodyplan.Stmt{Kind: "let", Name: "inside", Expr: 2}
	plan.Base.Statements[3] = bodyplan.Stmt{Kind: "let", Name: "inside", Expr: 3}
	firstRead := len(plan.Base.Expressions)
	plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "local", Name: "inside"}, bodyplan.Expr{Kind: "local", Name: "inside"})
	firstWrite := len(plan.Base.Statements)
	plan.Base.Statements = append(plan.Base.Statements, bodyplan.Stmt{Kind: "assign", Name: "first", Expr: firstRead}, bodyplan.Stmt{Kind: "assign", Name: "first", Expr: firstRead + 1})
	plan.Base.Statements[5].Then, plan.Base.Statements[5].Else = []int{2, firstWrite}, []int{3, firstWrite + 1}
	plan = pruneStructuralFixture(plan)
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = prepared.SourceFeatures("structure"); err == nil || !strings.Contains(err.Error(), "MULTIPLE_DECLARATIONS") {
		t.Fatal("ambiguous scope represented", err)
	}
	if value, err := prepared.fallback.Evaluate(4); err != nil || value.Int != 16 {
		t.Fatal("optional projection decline changed deterministic body", value, err)
	}
}

func BenchmarkSourceFeatureProjection(b *testing.B) {
	prepared, err := Prepare(interactingPlan())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := prepared.SourceFeatures("reference"); err != nil {
			b.Fatal(err)
		}
	}
}
