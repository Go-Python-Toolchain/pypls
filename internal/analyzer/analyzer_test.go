package analyzer

import (
	"strconv"
	"strings"
	"testing"

	"github.com/Go-Python-Toolchain/pypls/internal/diagnostic"
)

func TestCleanSourceHasNoDiagnostics(t *testing.T) {
	src := "def f(x):\n    return x + 1\n"
	if diags := Check("test.py", src); len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}

func TestSyntaxErrorRangeIsExact(t *testing.T) {
	// The stray close paren sits at columns 9 through 10 on line 1.
	src := "value = )\n"
	diags := Check("test.py", src)
	if len(diags) == 0 {
		t.Fatal("expected a diagnostic")
	}
	d := diags[0]
	if d.Severity != diagnostic.SeverityError {
		t.Fatalf("expected error severity, got %s", d.Severity)
	}
	if d.Range.Start.Line != 1 || d.Range.Start.Column != 9 {
		t.Fatalf("start: got %v, want 1:9", d.Range.Start)
	}
	if d.Range.End.Line != 1 || d.Range.End.Column != 10 {
		t.Fatalf("end: got %v, want 1:10", d.Range.End)
	}
}

func TestMissingColonReported(t *testing.T) {
	src := "if x\n    pass\n"
	diags := Check("test.py", src)
	if len(diags) == 0 {
		t.Fatal("expected a diagnostic for the missing colon")
	}
	if !strings.Contains(diags[0].Message, "expected") {
		t.Fatalf("unexpected message: %q", diags[0].Message)
	}
}

func TestUnterminatedStringReported(t *testing.T) {
	src := "x = \"abc\n"
	diags := Check("test.py", src)
	if len(diags) == 0 {
		t.Fatal("expected a diagnostic for the unterminated string")
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "unterminated") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an unterminated string message, got %v", diags)
	}
}

func TestDiagnosticsAreSorted(t *testing.T) {
	src := "a = )\nb = ]\n"
	diags := Check("test.py", src)
	if len(diags) < 2 {
		t.Fatalf("expected at least two diagnostics, got %d", len(diags))
	}
	prev := diags[0].Range.Start
	for _, d := range diags[1:] {
		cur := d.Range.Start
		if cur.Line < prev.Line || (cur.Line == prev.Line && cur.Column < prev.Column) {
			t.Fatalf("diagnostics not sorted: %v then %v", prev, cur)
		}
		prev = cur
	}
}

func genSource(lines int) string {
	var b strings.Builder
	const block = `def handler_ID(request, context=None):
    total = 0
    items = [x * 2 for x in range(100) if x % 3 == 0]
    for i, value in enumerate(items):
        if value > threshold and value is not None:
            total += value * factor
        else:
            continue
    return {"id": ID, "total": total}

`
	n := 0
	for n < lines {
		b.WriteString(strings.ReplaceAll(block, "ID", strconv.Itoa(n)))
		n += 11
	}
	return b.String()
}

func TestCheckLargeFileClean(t *testing.T) {
	src := genSource(10000)
	if diags := Check("big.py", src); len(diags) != 0 {
		t.Fatalf("expected clean check, got %d diagnostics, first: %v", len(diags), diags[0])
	}
}

func BenchmarkCheck10kLines(b *testing.B) {
	src := genSource(10000)
	b.ReportAllocs()
	b.SetBytes(int64(len(src)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Check("big.py", src)
	}
}
