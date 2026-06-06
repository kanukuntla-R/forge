// Package cli defines the forge command-line interface.
// It exposes Execute() as the single entry point invoked by main.
package cli

import (
	"github.com/spf13/cobra"
)

// rootCmd is the top-level "forge" command.
// All subcommands (new, add, visualize, list, install) attach to this.
var rootCmd = &cobra.Command{
	Use:   "forge",
	Short: "A meta-CLI for scaffolding projects from blueprints.",
	Long: `forge scaffolds new projects from blueprints.

A blueprint is a recipe for a kind of project (a hackathon web app,
a CLI tool, an agent skill). Given a blueprint and a few variables,
forge produces a complete, ready-to-edit project on disk.

forge is callable interactively, via flags, or programmatically via
stdin/stdout JSON, so humans, scripts, and AI agents can all drive it.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command. Called by main.
// Any error returned here propagates up and becomes the process exit code.
func Execute() error {
	return rootCmd.Execute()
}
