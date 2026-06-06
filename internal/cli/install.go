package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// installCmd implements `forge install <git-url>`.
// It clones a blueprint repository into ~/.config/forge/blueprints/.
var installCmd = &cobra.Command{
	Use:   "install <git-url>",
	Short: "Install a blueprint from a git repository",
	Long: `Install a blueprint from a git repository.

Clones the repo into ~/.config/forge/blueprints/<name>/ and validates
its manifest.yaml before accepting the install.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("forge install: not implemented yet (url=%s)", args[0])
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
