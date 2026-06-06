package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// addCmd implements `forge add <extension> [args...]`.
// It adds a component (api route, page, agent, etc.) to an existing forge project.
var addCmd = &cobra.Command{
	Use:   "add <extension> [args...]",
	Short: "Add a component to an existing forge project",
	Long: `Add a component to an existing forge project.

Reads .forge/project.json to determine which blueprint the project was
created from, then renders the named extension into the project tree.

Examples:
  forge add api-route users
  forge add page dashboard
  forge add component UserCard`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("forge add: not implemented yet (extension=%s)", args[0])
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
