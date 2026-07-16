package parser

import (
	"strconv"
	"strings"
	"testing"
)

// genSource builds a Python source string of roughly the requested number of
// lines by repeating a realistic function template. A placeholder token is used
// for the numeric suffix so that the literal percent signs in the template are
// left untouched.
func genSource(lines int) string {
	var b strings.Builder
	const block = `def handler_ID(request, context=None):
    total = 0
    items = [x * 2 for x in range(100) if x % 3 == 0]
    for i, value in enumerate(items):
        if value > threshold and value is not None:
            total += value * factor
        elif value < 0:
            total -= abs(value)
        else:
            continue
    result = {"id": ID, "total": total, "items": items}
    return result

`
	n := 0
	for n < lines {
		b.WriteString(strings.ReplaceAll(block, "ID", strconv.Itoa(n)))
		n += 13 // the block spans 13 lines
	}
	return b.String()
}

func TestParseLargeFile(t *testing.T) {
	src := genSource(10000)
	mod, errs := Parse("big.py", src)
	if len(errs) != 0 {
		t.Fatalf("expected clean parse of large file, got %d errors, first: %v", len(errs), errs[0])
	}
	if len(mod.Body) == 0 {
		t.Fatal("expected functions in the parsed module")
	}
	lineCount := strings.Count(src, "\n")
	if lineCount < 10000 {
		t.Fatalf("expected at least 10000 lines, generated %d", lineCount)
	}
}

func BenchmarkParse10kLines(b *testing.B) {
	src := genSource(10000)
	b.ReportAllocs()
	b.SetBytes(int64(len(src)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, errs := Parse("big.py", src)
		if len(errs) != 0 {
			b.Fatalf("unexpected errors: %v", errs)
		}
	}
}
