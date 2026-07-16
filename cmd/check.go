package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Go-Python-Toolchain/pypls/internal/analyzer"
	"github.com/Go-Python-Toolchain/pypls/internal/cache"
	"github.com/Go-Python-Toolchain/pypls/internal/config"
	"github.com/spf13/cobra"
)

var (
	checkNoCache    bool
	checkStrictFlag bool
	checkConfigPath string
)

var checkCmd = &cobra.Command{
	Use:   "check [paths...]",
	Short: "Check Python files for problems",
	Long: `check parses the given Python files and reports any problems it finds.

Each path may be a file or a directory. Directories are searched for files
ending in .py. When no path is given, the current directory is used. The command
exits with a non-zero status when any error level problem is found, which makes
it convenient to run in continuous integration.

Settings are read from the [tool.pypls] table of the nearest pyproject.toml.
Recognized keys are python, venv, strict, and exclude. Results are cached on disk
so that unchanged files are not rechecked. Pass --no-cache to check everything
from scratch.`,
	RunE: runCheck,
}

func init() {
	checkCmd.Flags().BoolVar(&checkNoCache, "no-cache", false, "check every file from scratch, ignoring the cache")
	checkCmd.Flags().BoolVar(&checkStrictFlag, "strict", false, "treat warnings as failures for the exit status")
	checkCmd.Flags().StringVar(&checkConfigPath, "config", "", "path to a pyproject.toml to read settings from")
	rootCmd.AddCommand(checkCmd)
}

// loadConfig reads settings from the explicit config path when given, otherwise
// from the nearest pyproject.toml. A read error is reported and the defaults are
// used so a broken config never blocks a check.
func loadConfig(cmd *cobra.Command) config.Config {
	var (
		cfg config.Config
		err error
	)
	if checkConfigPath != "" {
		cfg, err = config.LoadFile(checkConfigPath)
	} else {
		cfg, err = config.Load(".")
	}
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "pypls: ignoring unreadable config: %v\n", err)
		return config.Config{}
	}
	return cfg
}

// openCache returns a cache handle, or nil when caching is disabled or the cache
// cannot be opened. A nil cache is handled transparently by the analyzer.
func openCache(cmd *cobra.Command) *cache.Cache {
	if checkNoCache {
		return nil
	}
	dir, err := cache.DefaultDir()
	if err != nil {
		return nil
	}
	c, err := cache.OpenOrReset(dir)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "pypls: cache unavailable, continuing without it: %v\n", err)
		return nil
	}
	return c
}

func runCheck(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		args = []string{"."}
	}

	cfg := loadConfig(cmd)
	strict := cfg.Strict || checkStrictFlag

	files, err := collectPythonFiles(args, cfg)
	if err != nil {
		return err
	}

	pcache := openCache(cmd)
	defer pcache.Close()

	out := cmd.OutOrStdout()
	total := 0
	failures := 0

	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		diags := analyzer.CheckCached(pcache, file, string(source))
		for _, d := range diags {
			fmt.Fprintf(out, "%s:%d:%d: %s: %s\n",
				file, d.Range.Start.Line, d.Range.Start.Column, d.Severity, d.Message)
			total++
			if d.Severity == 1 || strict {
				failures++
			}
		}
	}

	if total == 0 {
		fmt.Fprintf(out, "checked %d file(s), no problems found\n", len(files))
		return nil
	}

	fmt.Fprintf(out, "checked %d file(s), found %d problem(s)\n", len(files), total)
	if failures > 0 {
		os.Exit(1)
	}
	return nil
}

// collectPythonFiles expands the given paths into a list of Python files,
// skipping anything the config excludes.
func collectPythonFiles(paths []string, cfg config.Config) ([]string, error) {
	var files []string
	seen := map[string]bool{}

	add := func(path string) {
		if seen[path] || cfg.Excluded(path) {
			return
		}
		seen[path] = true
		files = append(files, path)
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			add(path)
			continue
		}
		err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && cfg.Excluded(p) {
				return filepath.SkipDir
			}
			if !d.IsDir() && filepath.Ext(p) == ".py" {
				add(p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}
