package bodyplan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func TestCompileUnicodeActivityAndLocalNames(t *testing.T) {
	for _, name := range []string{"요금", "差額", "ΔValue", "Fee٣"} {
		plan := Plan{Schema: Schema, ID: "urn:gooo:activity:fee", Name: name, ResultType: decision.TypeInt,
			Expressions: []Expr{{Kind: ExprInput, Name: "input"}, {Kind: ExprInt, Int: 11},
				{Kind: ExprBinary, Operation: "subtract", Left: 0, Right: 1}, {Kind: ExprLocal, Name: "중간값"}},
			Statements: []Stmt{{Kind: StmtLet, Name: "중간값", Expr: 2}, {Kind: StmtReturn, Expr: 3}}, Root: []int{0, 1}}
		program, err := Compile(plan, nil)
		if err != nil {
			t.Fatal(name, err)
		}
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, "unicode.go", program.GoSource(), 0)
		if err != nil {
			t.Fatal(name, err)
		}
		if _, err := (&types.Config{}).Check("generated", set, []*ast.File{file}, nil); err != nil {
			t.Fatal(name, "generated source does not typecheck", err)
		}
		if !strings.Contains(program.GoSource(), "func "+name+"(") ||
			!strings.Contains(program.GoooSource(), "activity "+name+"(") ||
			!strings.Contains(program.GoooSource(), "let 중간값") {
			t.Fatal(name, "source spelling lost", program.GoooSource())
		}
	}
}

func TestUnicodeIdentifierValidationKeepsSyntaxAndByteBounds(t *testing.T) {
	for _, name := range []string{"_", "func", "1요금", "요금-1", "e\u0301", "😀", "bad\x00name"} {
		plan := Plan{Schema: Schema, ID: "invalid-name", Name: name, ResultType: decision.TypeInt,
			Expressions: []Expr{{Kind: ExprInput, Name: "input"}}, Statements: []Stmt{{Kind: StmtReturn, Expr: 0}}, Root: []int{0}}
		if _, err := Compile(plan, nil); err == nil {
			t.Fatal("invalid name accepted", name)
		}
	}
	plan := Plan{Schema: Schema, ID: "bounded-unicode", Name: strings.Repeat("요", 20<<10), ResultType: decision.TypeInt,
		Expressions: []Expr{{Kind: ExprInput, Name: "input"}}, Statements: []Stmt{{Kind: StmtReturn, Expr: 0}}, Root: []int{0}}
	if _, err := Compile(plan, nil); err == nil || !strings.Contains(err.Error(), "64 KiB") {
		t.Fatal("UTF-8 name bypassed plan byte bound", err)
	}
}
