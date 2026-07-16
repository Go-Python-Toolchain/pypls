package cache

import (
	"os"
	"path/filepath"
	"testing"
)

type sample struct {
	Name  string
	Count int
	Tags  []string
}

func TestRoundTrip(t *testing.T) {
	c, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	want := sample{Name: "widget", Count: 3, Tags: []string{"a", "b"}}
	if err := PutValue(c, "s", "k1", want); err != nil {
		t.Fatal(err)
	}
	got, ok := GetValue[sample](c, "s", "k1")
	if !ok {
		t.Fatal("expected a hit")
	}
	if got.Name != want.Name || got.Count != want.Count || len(got.Tags) != 2 {
		t.Fatalf("round trip mismatch: got %#v", got)
	}
}

func TestMissOnUnknownKey(t *testing.T) {
	c, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if _, ok := GetValue[sample](c, "s", "absent"); ok {
		t.Fatal("expected a miss for an unknown key")
	}
}

func TestPersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()

	c1, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := PutValue(c1, "s", "k", sample{Name: "kept", Count: 7}); err != nil {
		t.Fatal(err)
	}
	if err := c1.Close(); err != nil {
		t.Fatal(err)
	}

	c2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()

	got, ok := GetValue[sample](c2, "s", "k")
	if !ok {
		t.Fatal("expected the value to survive a reopen")
	}
	if got.Name != "kept" || got.Count != 7 {
		t.Fatalf("unexpected value after reopen: %#v", got)
	}
}

func TestDecodeFailureIsAMiss(t *testing.T) {
	c, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// Store bytes that are not valid msgpack for the requested type.
	if err := c.putBytes("s", "bad", []byte{0xff, 0xff, 0xff, 0xff}); err != nil {
		t.Fatal(err)
	}
	if _, ok := GetValue[sample](c, "s", "bad"); ok {
		t.Fatal("expected a decode failure to be reported as a miss")
	}
}

func TestOpenOrResetRecovers(t *testing.T) {
	// Point the cache at a path that is a file, which makes a normal open fail.
	// OpenOrReset should remove it and open a fresh cache.
	base := t.TempDir()
	badPath := filepath.Join(base, "occupied")
	if err := os.WriteFile(badPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(badPath); err == nil {
		t.Fatal("expected a plain open to fail on a file path")
	}

	c, err := OpenOrReset(badPath)
	if err != nil {
		t.Fatalf("expected OpenOrReset to recover, got %v", err)
	}
	defer c.Close()

	if err := PutValue(c, "s", "k", sample{Name: "recovered"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := GetValue[sample](c, "s", "k"); !ok {
		t.Fatal("expected the recovered cache to work")
	}
}

func TestDefaultDirHonorsXDG(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-example")
	dir, err := DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/tmp/xdg-example", "gpt", "pypls")
	if dir != want {
		t.Fatalf("got %q, want %q", dir, want)
	}
}
