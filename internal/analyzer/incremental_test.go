package analyzer

import "testing"

func TestIncrementalFirstPassAnalyzesAll(t *testing.T) {
	src := "def a():\n    return 1\n\ndef b():\n    return 2\n"
	ia := NewIncrementalAnalyzer()
	ia.Analyze(src)
	if ia.LastReanalyzed() != 2 {
		t.Fatalf("expected 2 units analyzed on first pass, got %d", ia.LastReanalyzed())
	}
}

func TestIncrementalReanalyzesOnlyChangedUnit(t *testing.T) {
	ia := NewIncrementalAnalyzer()
	first := "def a():\n    return 1\n\ndef b():\n    x: int = \"bad\"\n"
	ia.Analyze(first)

	// Re-analyzing identical source should touch nothing.
	ia.Analyze(first)
	if ia.LastReanalyzed() != 0 {
		t.Fatalf("expected 0 re-analyzed on unchanged source, got %d", ia.LastReanalyzed())
	}

	// Change only function a. Function b is unchanged but shifts down one line.
	second := "def a():\n    y = 0\n    return 1\n\ndef b():\n    x: int = \"bad\"\n"
	diags := ia.Analyze(second)
	if ia.LastReanalyzed() != 1 {
		t.Fatalf("expected exactly 1 unit re-analyzed, got %d", ia.LastReanalyzed())
	}

	// The diagnostic in the unchanged function b must follow it to its new line.
	if len(diags) != 1 {
		t.Fatalf("expected one diagnostic, got %v", diags)
	}
	if diags[0].Range.Start.Line != 6 {
		t.Fatalf("expected the reused diagnostic at line 6, got %d", diags[0].Range.Start.Line)
	}
}

func TestIncrementalMatchesFreshAnalysis(t *testing.T) {
	steps := []string{
		"def a():\n    return 1\n",
		"def a():\n    return 1\n\ndef b():\n    z = 1 + \"x\"\n",
		"def a():\n    q = 0\n    return 1\n\ndef b():\n    z = 1 + \"x\"\n",
	}
	ia := NewIncrementalAnalyzer()
	for _, src := range steps {
		incr := ia.Analyze(src)
		fresh := NewIncrementalAnalyzer().Analyze(src)
		if len(incr) != len(fresh) {
			t.Fatalf("incremental and fresh differ for:\n%s\nincr=%v fresh=%v", src, incr, fresh)
		}
		for i := range incr {
			if incr[i].Message != fresh[i].Message || incr[i].Range != fresh[i].Range {
				t.Fatalf("diagnostic %d differs: incr=%v fresh=%v", i, incr[i], fresh[i])
			}
		}
	}
}
