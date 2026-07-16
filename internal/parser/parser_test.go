package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Go-Python-Toolchain/pypls/internal/ast"
)

func mustParse(t *testing.T, src string) *ast.Module {
	t.Helper()
	mod, errs := Parse("test.py", src)
	if len(errs) != 0 {
		t.Fatalf("unexpected parse errors for source:\n%s\nerrors: %v", src, errs)
	}
	return mod
}

func TestCorpusParsesClean(t *testing.T) {
	files, err := filepath.Glob("testdata/*.py")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no corpus files found")
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		mod, errs := Parse(f, string(src))
		if len(errs) != 0 {
			t.Errorf("%s: expected clean parse, got errors: %v", f, errs)
		}
		if mod == nil || len(mod.Body) == 0 {
			t.Errorf("%s: expected a non-empty module", f)
		}
	}
}

func TestAssignment(t *testing.T) {
	mod := mustParse(t, "x = 1\n")
	as, ok := mod.Body[0].(*ast.Assign)
	if !ok {
		t.Fatalf("expected Assign, got %T", mod.Body[0])
	}
	if name, ok := as.Targets[0].(*ast.Name); !ok || name.Id != "x" {
		t.Fatalf("expected target x, got %#v", as.Targets[0])
	}
	if num, ok := as.Value.(*ast.Number); !ok || num.Value != "1" {
		t.Fatalf("expected value 1, got %#v", as.Value)
	}
}

func TestOperatorPrecedence(t *testing.T) {
	// 1 + 2 * 3 must parse as 1 + (2 * 3).
	mod := mustParse(t, "x = 1 + 2 * 3\n")
	as := mod.Body[0].(*ast.Assign)
	bin, ok := as.Value.(*ast.BinOp)
	if !ok {
		t.Fatalf("expected BinOp, got %T", as.Value)
	}
	if bin.Op.String() != "+" {
		t.Fatalf("expected top operator +, got %s", bin.Op)
	}
	right, ok := bin.Right.(*ast.BinOp)
	if !ok || right.Op.String() != "*" {
		t.Fatalf("expected right side to be multiplication, got %#v", bin.Right)
	}
}

func TestPowerRightAssociative(t *testing.T) {
	// 2 ** 3 ** 2 must parse as 2 ** (3 ** 2).
	mod := mustParse(t, "x = 2 ** 3 ** 2\n")
	as := mod.Body[0].(*ast.Assign)
	bin := as.Value.(*ast.BinOp)
	if bin.Op.String() != "**" {
		t.Fatalf("expected top **, got %s", bin.Op)
	}
	if _, ok := bin.Right.(*ast.BinOp); !ok {
		t.Fatalf("expected right-associative nesting, got %#v", bin.Right)
	}
}

func TestChainedComparison(t *testing.T) {
	mod := mustParse(t, "r = a < b <= c\n")
	as := mod.Body[0].(*ast.Assign)
	cmp, ok := as.Value.(*ast.Compare)
	if !ok {
		t.Fatalf("expected Compare, got %T", as.Value)
	}
	if len(cmp.Ops) != 2 || cmp.Ops[0] != ast.CmpLt || cmp.Ops[1] != ast.CmpLte {
		t.Fatalf("unexpected comparison ops: %v", cmp.Ops)
	}
}

func TestFunctionWithParams(t *testing.T) {
	mod := mustParse(t, "def f(a, b=2, *c, d, **e):\n    return a\n")
	fn, ok := mod.Body[0].(*ast.FunctionDef)
	if !ok {
		t.Fatalf("expected FunctionDef, got %T", mod.Body[0])
	}
	kinds := []ast.ParamKind{}
	for _, prm := range fn.Params.Params {
		kinds = append(kinds, prm.Kind)
	}
	want := []ast.ParamKind{ast.ParamNormal, ast.ParamNormal, ast.ParamVararg, ast.ParamNormal, ast.ParamKwarg}
	if len(kinds) != len(want) {
		t.Fatalf("expected %d params, got %d", len(want), len(kinds))
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("param %d kind: got %v, want %v", i, kinds[i], want[i])
		}
	}
	if fn.Params.Params[1].Default == nil {
		t.Fatal("expected b to have a default")
	}
}

func TestDecoratorsAndAsync(t *testing.T) {
	mod := mustParse(t, "@deco\nasync def f():\n    await g()\n")
	fn, ok := mod.Body[0].(*ast.FunctionDef)
	if !ok {
		t.Fatalf("expected FunctionDef, got %T", mod.Body[0])
	}
	if !fn.Async {
		t.Fatal("expected async function")
	}
	if len(fn.Decorators) != 1 {
		t.Fatalf("expected one decorator, got %d", len(fn.Decorators))
	}
}

func TestIfElifElse(t *testing.T) {
	mod := mustParse(t, "if a:\n    x\nelif b:\n    y\nelse:\n    z\n")
	ifs, ok := mod.Body[0].(*ast.If)
	if !ok {
		t.Fatalf("expected If, got %T", mod.Body[0])
	}
	if len(ifs.Orelse) != 1 {
		t.Fatalf("expected chained elif in orelse, got %d stmts", len(ifs.Orelse))
	}
	if _, ok := ifs.Orelse[0].(*ast.If); !ok {
		t.Fatalf("expected elif to be an If node, got %T", ifs.Orelse[0])
	}
}

func TestComprehension(t *testing.T) {
	mod := mustParse(t, "x = [i * 2 for i in range(10) if i > 0]\n")
	as := mod.Body[0].(*ast.Assign)
	lc, ok := as.Value.(*ast.ListComp)
	if !ok {
		t.Fatalf("expected ListComp, got %T", as.Value)
	}
	if len(lc.Generators) != 1 || len(lc.Generators[0].Ifs) != 1 {
		t.Fatalf("unexpected comprehension shape: %#v", lc.Generators)
	}
}

func TestDictAndSet(t *testing.T) {
	mod := mustParse(t, "d = {\"a\": 1, **rest}\ns = {1, 2, 3}\n")
	if _, ok := mod.Body[0].(*ast.Assign).Value.(*ast.Dict); !ok {
		t.Fatalf("expected Dict, got %T", mod.Body[0].(*ast.Assign).Value)
	}
	if _, ok := mod.Body[1].(*ast.Assign).Value.(*ast.Set); !ok {
		t.Fatalf("expected Set, got %T", mod.Body[1].(*ast.Assign).Value)
	}
}

func TestAnnotatedAssignment(t *testing.T) {
	mod := mustParse(t, "x: int = 5\n")
	ann, ok := mod.Body[0].(*ast.AnnAssign)
	if !ok {
		t.Fatalf("expected AnnAssign, got %T", mod.Body[0])
	}
	if ann.Annotation == nil || ann.Value == nil {
		t.Fatal("expected annotation and value to be present")
	}
}

func TestPositionsAreAccurate(t *testing.T) {
	mod := mustParse(t, "value = 123\n")
	as := mod.Body[0].(*ast.Assign)
	target := as.Targets[0]
	if target.Pos().Line != 1 || target.Pos().Column != 1 {
		t.Fatalf("target start: got %v, want 1:1", target.Pos())
	}
	// "value" ends at column 6 (one past the last rune).
	if target.End().Column != 6 {
		t.Fatalf("target end column: got %d, want 6", target.End().Column)
	}
	if as.Value.Pos().Column != 9 {
		t.Fatalf("value start column: got %d, want 9", as.Value.Pos().Column)
	}
}

func TestErrorRecovery(t *testing.T) {
	// A broken first line should not prevent parsing the second.
	mod, errs := Parse("test.py", "x = = 1\ny = 2\n")
	if len(errs) == 0 {
		t.Fatal("expected at least one error")
	}
	found := false
	for _, s := range mod.Body {
		if as, ok := s.(*ast.Assign); ok {
			if name, ok := as.Targets[0].(*ast.Name); ok && name.Id == "y" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected to recover and parse the y assignment")
	}
}

func TestNestedDataStructures(t *testing.T) {
	mustParse(t, "config = {\n    \"servers\": [\n        {\"host\": \"a\", \"port\": 80},\n        {\"host\": \"b\", \"port\": 443},\n    ],\n    \"retries\": 3,\n}\n")
}
