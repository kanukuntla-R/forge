package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// visualizeCmd implements `forge visualize [path]`.
// It opens the Understand-Anything dashboard for the project at [path] (or cwd).
var visualizeCmd = &cobra.Command{
	Use:   "visualize [path]",
	Short: "Open the Understand-Anything dashboard for a forge project",
	Long: `Open the Understand-Anything dashboard for the project at [path] (or cwd).

Requires .understand-anything/knowledge-graph.json to exist. forge writes
that file at scaffold time, so any forge-created project is visualizable.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("forge visualize: not implemented yet")
	},
}

func init() {
	rootCmd.AddCommand(visualizeCmd)
}
