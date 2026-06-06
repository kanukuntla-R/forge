package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// listCmd implements `forge list`.
// It enumerates available blueprints (embedded + user-installed).
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available blueprints",
	Long: `List all blueprints available to forge: built-in (embedded in the binary),
user-installed (~/.config/forge/blueprints/), and project-local (.forge/blueprints/).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("forge list: not implemented yet")
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
