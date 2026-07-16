package analyzer

import (
	"testing"

	"github.com/Go-Python-Toolchain/pypls/internal/cache"
)

func TestCheckCachedMatchesDirect(t *testing.T) {
	c, err := cache.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	src := "x: int = \"bad\"\ny = 1 + 2\n"
	direct := Check("test.py", src)

	// First call populates the cache, second call reads it back.
	first := CheckCached(c, "test.py", src)
	second := CheckCached(c, "test.py", src)

	if len(direct) != len(first) || len(first) != len(second) {
		t.Fatalf("diagnostic counts differ: direct=%d first=%d second=%d",
			len(direct), len(first), len(second))
	}
	for i := range direct {
		if direct[i].Message != second[i].Message || direct[i].Range != second[i].Range {
			t.Fatalf("cached diagnostic %d differs from direct: %v vs %v", i, second[i], direct[i])
		}
	}
}

func TestCheckCachedNilCacheWorks(t *testing.T) {
	src := "x = 1 + 2\n"
	if diags := CheckCached(nil, "test.py", src); len(diags) != 0 {
		t.Fatalf("expected no diagnostics with a nil cache, got %v", diags)
	}
}

func TestEditInvalidatesCache(t *testing.T) {
	c, err := cache.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	clean := CheckCached(c, "test.py", "x = 1\n")
	if len(clean) != 0 {
		t.Fatalf("expected clean source to have no diagnostics, got %v", clean)
	}
	// Different content hashes to a different key, so the stale entry is not used.
	broken := CheckCached(c, "test.py", "x: int = \"s\"\n")
	if len(broken) == 0 {
		t.Fatal("expected the edited source to be rechecked and report a problem")
	}
}

func BenchmarkCheckCold(b *testing.B) {
	src := genSource(10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Check("big.py", src)
	}
}

func BenchmarkCheckWarm(b *testing.B) {
	src := genSource(10000)
	c, err := cache.Open(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()

	// Populate the cache once so every measured call is a hit.
	CheckCached(c, "big.py", src)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckCached(c, "big.py", src)
	}
}
