package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCmd implements `forge new <blueprint> [name]`.
// It scaffolds a new project from a blueprint.
var newCmd = &cobra.Command{
	Use:   "new <blueprint> [name]",
	Short: "Scaffold a new project from a blueprint",
	Long: `Scaffold a new project from a blueprint.

Examples:
  forge new hackathon-app my-idea
  forge new hackathon-app my-idea --with-ai --no-database
  cat vars.json | forge new hackathon-app --json`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("forge new: not implemented yet (blueprint=%s)", args[0])
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}
