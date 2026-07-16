package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProject(t *testing.T, dir, contents string) string {
	t.Helper()
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadReadsSettings(t *testing.T) {
	dir := t.TempDir()
	writeProject(t, dir, `
[project]
name = "example"

[tool.pypls]
python = "python3.12"
venv = ".venv"
strict = true
exclude = ["build", "*.gen.py"]

[tool.other]
setting = 1
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Loaded {
		t.Fatal("expected config to be loaded")
	}
	if cfg.Python != "python3.12" || cfg.Venv != ".venv" || !cfg.Strict {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if len(cfg.Exclude) != 2 {
		t.Fatalf("expected two exclude patterns, got %v", cfg.Exclude)
	}
}

func TestLoadMissingFileIsDefault(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Loaded {
		t.Fatal("expected no config to be loaded")
	}
}

func TestMalformedFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeProject(t, dir, "[tool.pypls]\npython = \n")
	if _, err := Load(dir); err == nil {
		t.Fatal("expected an error for malformed toml")
	}
}

func TestFindSearchesUpward(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, "[tool.pypls]\nstrict = true\n")
	nested := filepath.Join(root, "src", "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	path, ok := Find(nested)
	if !ok {
		t.Fatal("expected to find pyproject.toml in a parent")
	}
	if filepath.Dir(path) != root {
		t.Fatalf("found the wrong file: %s", path)
	}
}

func TestExcluded(t *testing.T) {
	cfg := Config{Exclude: []string{"build", "*.gen.py", "vendor/"}}
	cases := map[string]bool{
		"build/main.py":      true,
		"src/build/thing.py": true,
		"app/models.gen.py":  true,
		"vendor/lib.py":      true,
		"src/app/models.py":  false,
		"tests/test_core.py": false,
		"buildinfo/keep.py":  false,
	}
	for path, want := range cases {
		if got := cfg.Excluded(path); got != want {
			t.Errorf("Excluded(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestNoExcludeMatchesNothing(t *testing.T) {
	cfg := Config{}
	if cfg.Excluded("anything.py") {
		t.Fatal("expected no exclusions with an empty config")
	}
}
