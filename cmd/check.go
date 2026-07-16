package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Go-Python-Toolchain/pypls/internal/analyzer"
	"github.com/Go-Python-Toolchain/pypls/internal/cache"
	"github.com/spf13/cobra"
)

var checkNoCache bool

var checkCmd = &cobra.Command{
	Use:   "check [paths...]",
	Short: "Check Python files for problems",
	Long: `check parses the given Python files and reports any problems it finds.

Each path may be a file or a directory. Directories are searched for files
ending in .py. When no path is given, the current directory is used. The command
exits with a non-zero status when any error level problem is found, which makes
it convenient to run in continuous integration.

Results are cached on disk so that unchanged files are not rechecked. Pass
--no-cache to check everything from scratch.`,
	RunE: runCheck,
}

func init() {
	checkCmd.Flags().BoolVar(&checkNoCache, "no-cache", false, "check every file from scratch, ignoring the cache")
	rootCmd.AddCommand(checkCmd)
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

	files, err := collectPythonFiles(args)
	if err != nil {
		return err
	}

	pcache := openCache(cmd)
	defer pcache.Close()

	out := cmd.OutOrStdout()
	total := 0
	errorCount := 0

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
			if d.Severity == 1 {
				errorCount++
			}
		}
	}

	if total == 0 {
		fmt.Fprintf(out, "checked %d file(s), no problems found\n", len(files))
		return nil
	}

	fmt.Fprintf(out, "checked %d file(s), found %d problem(s)\n", len(files), total)
	if errorCount > 0 {
		os.Exit(1)
	}
	return nil
}

// collectPythonFiles expands the given paths into a list of Python files.
func collectPythonFiles(paths []string) ([]string, error) {
	var files []string
	seen := map[string]bool{}

	add := func(path string) {
		if !seen[path] {
			seen[path] = true
			files = append(files, path)
		}
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
