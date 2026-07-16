// Package config reads pypls settings from a project's pyproject.toml file. It
// looks for a [tool.pypls] table and never requires one: a project with no
// configuration is checked with sensible defaults. Following the drop-in rule,
// pypls reads existing standards rather than asking for a new config file.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// fileName is the standard Python project configuration file.
const fileName = "pyproject.toml"

// Config holds the settings pypls understands. Zero values are the defaults.
type Config struct {
	// Python is the interpreter the project targets, for example python3.12.
	Python string
	// Venv is the project's virtual environment directory.
	Venv string
	// Strict makes warnings count as failures for the exit status.
	Strict bool
	// Exclude lists glob patterns for files and directories to skip.
	Exclude []string

	// Loaded is true when a [tool.pypls] table was found and read.
	Loaded bool
	// Path is the pyproject.toml the settings came from, if any.
	Path string
}

// pyproject mirrors the parts of pyproject.toml that pypls reads. Unknown keys
// elsewhere in the file are ignored.
type pyproject struct {
	Tool struct {
		Pypls struct {
			Python  string   `toml:"python"`
			Venv    string   `toml:"venv"`
			Strict  bool     `toml:"strict"`
			Exclude []string `toml:"exclude"`
		} `toml:"pypls"`
	} `toml:"tool"`
}

// Find searches startDir and its parents for a pyproject.toml and returns its
// path. The boolean is false when none is found.
func Find(startDir string) (string, bool) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(dir, fileName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// Load finds the nearest pyproject.toml starting at startDir and reads its
// settings. When no file is found it returns the default config with Loaded
// false and no error.
func Load(startDir string) (Config, error) {
	path, ok := Find(startDir)
	if !ok {
		return Config{}, nil
	}
	return LoadFile(path)
}

// LoadFile reads settings from a specific pyproject.toml.
func LoadFile(path string) (Config, error) {
	var raw pyproject
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return Config{}, err
	}
	p := raw.Tool.Pypls
	return Config{
		Python:  p.Python,
		Venv:    p.Venv,
		Strict:  p.Strict,
		Exclude: p.Exclude,
		Loaded:  true,
		Path:    path,
	}, nil
}

// Excluded reports whether a file path is excluded by the configured patterns.
// A pattern matches when it globs the whole path, globs the file's base name, or
// equals one of the path's directory segments.
func (c Config) Excluded(path string) bool {
	if len(c.Exclude) == 0 {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	base := filepath.Base(clean)
	segments := strings.Split(clean, "/")

	for _, pattern := range c.Exclude {
		pattern = strings.TrimSuffix(pattern, "/")
		if pattern == "" {
			continue
		}
		if ok, _ := filepath.Match(pattern, clean); ok {
			return true
		}
		if ok, _ := filepath.Match(pattern, base); ok {
			return true
		}
		for _, seg := range segments {
			if seg == pattern {
				return true
			}
		}
	}
	return false
}
