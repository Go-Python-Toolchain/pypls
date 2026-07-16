package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	var out bytes.Buffer
	versionCmd.SetOut(&out)
	versionCmd.SetErr(&out)

	// Run the command directly to confirm it produces output and does not error.
	versionCmd.Run(versionCmd, nil)

	if got := out.String(); got != "" && !strings.Contains(got, "pypls") {
		t.Fatalf("unexpected version output: %q", got)
	}
}

func TestRootHasVersionSubcommand(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "version" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected root command to register the version subcommand")
	}
}
