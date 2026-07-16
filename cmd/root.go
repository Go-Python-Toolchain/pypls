package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pypls",
	Short: "A fast, incremental type checker and language server for Python",
	Long: `pypls is a persistent daemon that provides fast, incremental type
checking and IDE feedback for Python projects.

It runs entirely on your machine and never sends your code anywhere. pypls
sits alongside your existing tools and does not ask you to change your code,
your project layout, or your primary commands.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command and exits non-zero on failure.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "pypls:", err)
		os.Exit(1)
	}
}
