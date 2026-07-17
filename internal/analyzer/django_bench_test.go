package analyzer

// This file measures analyzer.Check against real Django source. It is guarded by
// the DJANGO_SRC environment variable so that continuous integration stays
// offline and deterministic: with the variable unset every test and benchmark
// here skips. To run it, point DJANGO_SRC at a Django checkout, for example:
//
//	git clone --depth 1 --branch 4.2 https://github.com/django/django /tmp/django-4.2
//	DJANGO_SRC=/tmp/django-4.2 go test ./internal/analyzer -run TestDjango -v
//	DJANGO_SRC=/tmp/django-4.2 go test ./internal/analyzer -run xxx -bench Django -benchmem
//
// The blueprint's "resolves types for Django 4.2 source in under 150ms" target
// is about editor responsiveness, so the check that matters is a single large
// file. The tests below report the per-file time for several representative
// large files, the largest single-file time against the 150ms budget, and the
// whole-tree time for transparency.

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// djangoRepresentativeFiles are large, dependency-heavy modules that stand in
// for the kind of file an engineer edits in a Django codebase. Paths are
// relative to the Django source root.
var djangoRepresentativeFiles = []string{
	"django/db/models/query.py",
	"django/db/models/base.py",
	"django/db/models/fields/__init__.py",
	"django/forms/forms.py",
	"django/forms/models.py",
	"django/core/management/base.py",
	"django/contrib/admin/options.py",
	"django/db/models/sql/query.py",
}

// djangoSrc returns the Django source root from DJANGO_SRC, skipping the test
// when it is unset so offline runs stay green.
func djangoSrc(tb testing.TB) string {
	tb.Helper()
	root := os.Getenv("DJANGO_SRC")
	if root == "" {
		tb.Skip("DJANGO_SRC is not set; skipping Django type-resolution measurement")
	}
	if _, err := os.Stat(filepath.Join(root, "django", "__init__.py")); err != nil {
		tb.Skipf("DJANGO_SRC=%q does not look like a Django checkout: %v", root, err)
	}
	return root
}

// readSource reads a file, failing the test if it cannot be read.
func readSource(tb testing.TB, path string) string {
	tb.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}

// medianCheck runs Check on source several times and returns the median wall
// time, which is stabler than a single sample against scheduler noise.
func medianCheck(file, source string, runs int) time.Duration {
	times := make([]time.Duration, 0, runs)
	for i := 0; i < runs; i++ {
		start := time.Now()
		Check(file, source)
		times = append(times, time.Since(start))
	}
	for i := 1; i < len(times); i++ {
		for j := i; j > 0 && times[j] < times[j-1]; j-- {
			times[j], times[j-1] = times[j-1], times[j]
		}
	}
	return times[len(times)/2]
}

// TestDjangoPerFileResponsiveness checks each representative Django file and
// asserts the largest single-file check stays under the 150ms editor budget.
// It reports every per-file number so the run is self-documenting.
func TestDjangoPerFileResponsiveness(t *testing.T) {
	root := djangoSrc(t)
	const budget = 150 * time.Millisecond
	const runs = 7

	var worst time.Duration
	var worstFile string
	for _, rel := range djangoRepresentativeFiles {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err != nil {
			t.Logf("skipping %s: not present in this checkout", rel)
			continue
		}
		src := readSource(t, path)
		lines := 1
		for _, c := range src {
			if c == '\n' {
				lines++
			}
		}
		d := medianCheck(path, src, runs)
		t.Logf("%-45s %6d lines  %8.3f ms", rel, lines, float64(d.Microseconds())/1000)
		if d > worst {
			worst = d
			worstFile = rel
		}
	}
	if worstFile == "" {
		t.Skip("none of the representative Django files were present")
	}
	t.Logf("largest single-file check: %s at %.3f ms (budget %.0f ms)",
		worstFile, float64(worst.Microseconds())/1000, float64(budget.Milliseconds()))
	if worst > budget {
		t.Fatalf("largest single-file check %.3f ms exceeds the %.0f ms budget (%s)",
			float64(worst.Microseconds())/1000, float64(budget.Milliseconds()), worstFile)
	}
}

// TestDjangoFullTree checks every .py file under the Django source root and
// reports the total time and file count. This is transparency for the whole
// corpus, not the editor-responsiveness budget, so it does not assert a bound.
func TestDjangoFullTree(t *testing.T) {
	root := djangoSrc(t)

	type sample struct {
		rel string
		dur time.Duration
	}
	var files int
	var total time.Duration
	var slowest sample
	start := time.Now()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".py" {
			return nil
		}
		src := readSource(t, path)
		d := medianCheck(path, src, 1)
		files++
		total += d
		if d > slowest.dur {
			rel, _ := filepath.Rel(root, path)
			slowest = sample{rel: rel, dur: d}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking Django tree: %v", err)
	}
	wall := time.Since(start)
	if files == 0 {
		t.Skip("no Python files found under DJANGO_SRC")
	}
	t.Logf("checked %d Python files in %.1f s of check time (%.1f s wall including IO)",
		files, total.Seconds(), wall.Seconds())
	t.Logf("mean per file %.3f ms; slowest %s at %.3f ms",
		float64(total.Microseconds())/1000/float64(files),
		slowest.rel, float64(slowest.dur.Microseconds())/1000)
}

// TestDjangoIncrementalRecheck measures the incremental re-check time for a
// large file after a one-line edit, which is the latency an editor pays on each
// keystroke. It reports both the cold first analysis and the warm re-check.
func TestDjangoIncrementalRecheck(t *testing.T) {
	root := djangoSrc(t)
	path := filepath.Join(root, "django/db/models/query.py")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("representative file missing: %v", err)
	}
	src := readSource(t, path)

	ia := NewIncrementalAnalyzer()
	coldStart := time.Now()
	ia.Analyze(src)
	cold := time.Since(coldStart)

	// A minimal edit: append a comment line, which changes only the trailing
	// region and leaves every earlier top-level unit cache-eligible.
	edited := src + "\n# incremental benchmark edit\n"
	warmStart := time.Now()
	ia.Analyze(edited)
	warm := time.Since(warmStart)

	t.Logf("django/db/models/query.py: cold analyze %.3f ms, warm re-check after one-line edit %.3f ms, units rechecked %d",
		float64(cold.Microseconds())/1000, float64(warm.Microseconds())/1000, ia.LastReanalyzed())
}

// BenchmarkDjangoCheck benchmarks Check on the largest representative Django
// file so the number can be reproduced with the Go benchmark tooling.
func BenchmarkDjangoCheck(b *testing.B) {
	root := djangoSrc(b)

	var path, src string
	var maxLen int
	for _, rel := range djangoRepresentativeFiles {
		p := filepath.Join(root, rel)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if len(data) > maxLen {
			maxLen = len(data)
			path = p
			src = string(data)
		}
	}
	if src == "" {
		b.Skip("no representative Django files present")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Check(path, src)
	}
}
