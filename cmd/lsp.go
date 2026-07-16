package cmd

import (
	"github.com/Go-Python-Toolchain/pypls/internal/lsp"
	"github.com/spf13/cobra"
)

var lspCmd = &cobra.Command{
	Use:   "lsp",
	Short: "Run the language server over stdin and stdout",
	Long: `lsp runs pypls as a Language Server Protocol server. It reads requests
from standard input and writes responses to standard output, which is how
editors launch and talk to it.

You normally do not run this by hand. Point your editor's language client at
"pypls lsp" as the command for Python files.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		server := lsp.NewServer(cmd.InOrStdin(), cmd.OutOrStdout(), version)
		return server.Run()
	},
}

func init() {
	rootCmd.AddCommand(lspCmd)
}
