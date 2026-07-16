package lexer

import (
	"strings"
	"testing"

	"github.com/Go-Python-Toolchain/pypls/internal/token"
)

// typeSeq collects the token types produced for src, dropping the trailing EOF
// for readability in tests.
func typeSeq(t *testing.T, src string) []token.Type {
	t.Helper()
	toks, errs := New("test.py", src).Tokenize()
	if len(errs) != 0 {
		t.Fatalf("unexpected lex errors for %q: %v", src, errs)
	}
	var out []token.Type
	for _, tk := range toks {
		out = append(out, tk.Type)
	}
	return out
}

func joinTypes(types []token.Type) string {
	var parts []string
	for _, ty := range types {
		parts = append(parts, ty.String())
	}
	return strings.Join(parts, " ")
}

func TestSimpleAssignment(t *testing.T) {
	got := joinTypes(typeSeq(t, "x = 1\n"))
	want := "NAME = NUMBER NEWLINE EOF"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestIndentation(t *testing.T) {
	src := "def f():\n    return 1\n"
	got := joinTypes(typeSeq(t, src))
	want := "def NAME ( ) : NEWLINE INDENT return NUMBER NEWLINE DEDENT EOF"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNestedDedents(t *testing.T) {
	src := "if a:\n    if b:\n        c\nd\n"
	got := joinTypes(typeSeq(t, src))
	want := "if NAME : NEWLINE INDENT if NAME : NEWLINE INDENT NAME NEWLINE DEDENT DEDENT NAME NEWLINE EOF"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBlankAndCommentLinesIgnored(t *testing.T) {
	src := "x = 1\n\n# a comment\n    \ny = 2\n"
	got := joinTypes(typeSeq(t, src))
	want := "NAME = NUMBER NEWLINE NAME = NUMBER NEWLINE EOF"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestImplicitLineJoining(t *testing.T) {
	src := "x = (1 +\n     2)\n"
	got := joinTypes(typeSeq(t, src))
	want := "NAME = ( NUMBER + NUMBER ) NEWLINE EOF"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExplicitLineContinuation(t *testing.T) {
	src := "x = 1 + \\\n    2\n"
	got := joinTypes(typeSeq(t, src))
	want := "NAME = NUMBER + NUMBER NEWLINE EOF"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStringsAndPrefixes(t *testing.T) {
	toks, errs := New("test.py", `a = "hi"
b = 'x'
c = f"v={x}"
d = r"\d+"
e = """triple
line"""
`).Tokenize()
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	var strs []string
	for _, tk := range toks {
		if tk.Type == token.STRING {
			strs = append(strs, tk.Value)
		}
	}
	want := []string{`"hi"`, `'x'`, `f"v={x}"`, `r"\d+"`, "\"\"\"triple\nline\"\"\""}
	if len(strs) != len(want) {
		t.Fatalf("got %d strings %v, want %d", len(strs), strs, len(want))
	}
	for i := range want {
		if strs[i] != want[i] {
			t.Fatalf("string %d: got %q, want %q", i, strs[i], want[i])
		}
	}
}

func TestNumbers(t *testing.T) {
	toks, errs := New("test.py", "a = 0xFF\nb = 1_000\nc = 3.14\nd = 1e10\ne = 2j\nf = .5\n").Tokenize()
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	var nums []string
	for _, tk := range toks {
		if tk.Type == token.NUMBER {
			nums = append(nums, tk.Value)
		}
	}
	want := []string{"0xFF", "1_000", "3.14", "1e10", "2j", ".5"}
	if strings.Join(nums, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", nums, want)
	}
}

func TestOperators(t *testing.T) {
	got := joinTypes(typeSeq(t, "a += 1 ** 2 // 3 @ b := c\n"))
	want := "NAME += NUMBER ** NUMBER // NUMBER @ NAME := NAME NEWLINE EOF"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPositions(t *testing.T) {
	toks, _ := New("test.py", "ab = cd\n").Tokenize()
	// First token "ab" starts at line 1, column 1.
	if toks[0].Start.Line != 1 || toks[0].Start.Column != 1 {
		t.Fatalf("ab start: got %v", toks[0].Start)
	}
	// "cd" starts at column 6.
	if toks[2].Value != "cd" || toks[2].Start.Column != 6 {
		t.Fatalf("cd token: got %q at %v", toks[2].Value, toks[2].Start)
	}
}

func TestUnindentMismatchReported(t *testing.T) {
	src := "if a:\n        b\n    c\n"
	_, errs := New("test.py", src).Tokenize()
	if len(errs) == 0 {
		t.Fatal("expected an unindent error")
	}
}
