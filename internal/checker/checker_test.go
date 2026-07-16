package checker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Go-Python-Toolchain/pypls/internal/diagnostic"
	"github.com/Go-Python-Toolchain/pypls/internal/parser"
)

func check(t *testing.T, src string) []diagnostic.Diagnostic {
	t.Helper()
	mod, errs := parser.Parse("test.py", src)
	if len(errs) != 0 {
		t.Fatalf("source has syntax errors, cannot test types: %v", errs)
	}
	return Check(mod)
}

// wantClean asserts that a source produces no type diagnostics.
func wantClean(t *testing.T, src string) {
	t.Helper()
	if diags := check(t, src); len(diags) != 0 {
		t.Fatalf("expected no diagnostics for:\n%s\ngot: %v", src, diags)
	}
}

// wantOne asserts exactly one diagnostic whose message contains substr.
func wantOne(t *testing.T, src, substr string) diagnostic.Diagnostic {
	t.Helper()
	diags := check(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one diagnostic for:\n%s\ngot %d: %v", src, len(diags), diags)
	}
	if !strings.Contains(diags[0].Message, substr) {
		t.Fatalf("message %q does not contain %q", diags[0].Message, substr)
	}
	return diags[0]
}

func TestLiteralTypesViaAnnotation(t *testing.T) {
	// These only fire if the literal type is inferred correctly.
	wantOne(t, "x: str = 5\n", "int")
	wantOne(t, "x: int = 3.0\n", "float")
	wantOne(t, "x: int = 2j\n", "complex")
	wantOne(t, "x: bytes = \"s\"\n", "str")
	wantOne(t, "x: str = b\"s\"\n", "bytes")
}

func TestNumericWideningIsAllowed(t *testing.T) {
	wantClean(t, "x: int = True\n")
	wantClean(t, "x: float = 5\n")
	wantClean(t, "x: complex = 3.5\n")
	wantClean(t, "x: int = 42\n")
}

func TestNoneIsAlwaysAssignable(t *testing.T) {
	wantClean(t, "x: int = None\n")
}

func TestBuiltinConstructorInference(t *testing.T) {
	// str(5) returns str, which is not assignable to an int annotation.
	wantOne(t, "x: int = str(5)\n", "str")
	// int("5") returns int, which is fine.
	wantClean(t, "x: int = int(\"5\")\n")
}

func TestUnsupportedOperands(t *testing.T) {
	wantOne(t, "y = \"a\" + 1\n", "unsupported operand")
	wantOne(t, "y = \"a\" - \"b\"\n", "unsupported operand")
	wantOne(t, "y = [1] - [2]\n", "unsupported operand")
}

func TestValidOperandsAreClean(t *testing.T) {
	wantClean(t, "y = 1 + 2\n")
	wantClean(t, "y = 1 + 2.0\n")
	wantClean(t, "y = \"a\" + \"b\"\n")
	wantClean(t, "y = \"ab\" * 3\n")
	wantClean(t, "y = 3 * \"ab\"\n")
	wantClean(t, "y = [1] + [2]\n")
	wantClean(t, "y = {1, 2} - {2}\n")
	wantClean(t, "y = \"total: %d\" % 5\n")
	wantClean(t, "y = 7 // 2\n")
	wantClean(t, "y = 1 << 4\n")
}

func TestNoFalsePositivesOnUntypedCode(t *testing.T) {
	wantClean(t, "def add(a, b):\n    return a + b\n")
	wantClean(t, "x = get_value()\ny = x + other()\n")
	wantClean(t, "obj.attr = 1\nobj.method() + 2\n")
	wantClean(t, "z = data[0] + data[1]\n")
}

func TestAugAssignOperand(t *testing.T) {
	wantOne(t, "s = \"x\"\ns += 5\n", "unsupported operand")
	wantClean(t, "n = 0\nn += 5\n")
}

func TestParamAnnotationsInform(t *testing.T) {
	// n is annotated int, so n + "x" is invalid.
	wantOne(t, "def f(n: int):\n    return n + \"x\"\n", "unsupported operand")
	// Without the annotation there is nothing to flag.
	wantClean(t, "def f(n):\n    return n + \"x\"\n")
}

func TestMismatchRangePointsAtValue(t *testing.T) {
	d := wantOne(t, "x: int = \"hello\"\n", "str")
	if d.Severity != diagnostic.SeverityWarning {
		t.Fatalf("expected warning severity, got %s", d.Severity)
	}
	// The value "hello" starts at column 10.
	if d.Range.Start.Column != 10 {
		t.Fatalf("expected diagnostic at column 10, got %d", d.Range.Start.Column)
	}
}

func TestCorpusHasNoTypeFalsePositives(t *testing.T) {
	files, _ := filepath.Glob("../parser/testdata/*.py")
	if len(files) == 0 {
		t.Skip("no corpus files found")
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		mod, errs := parser.Parse(f, string(src))
		if len(errs) != 0 {
			continue // syntax handled elsewhere
		}
		if diags := Check(mod); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics on real code, got: %v", f, diags)
		}
	}
}
