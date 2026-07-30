package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kanukuntla-r/forge/internal/skill"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage forge's Claude Code skill",
	Long: `Manage forge's Claude Code skill file.

Currently supports install; future subcommands may add list, update, and uninstall.`,
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install forge's skill to Claude Code",
	Long: `Install forge's skill file to ~/.claude/skills/forge.skill.md.

This teaches Claude Code to reach for forge when scaffolding projects.
If a file already exists, it is backed up with a timestamp suffix.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := skill.Install()
		if err != nil {
			return err
		}
		printSkillInstallSuccess(cmd.OutOrStdout(), result)
		return nil
	},
}

func init() {
	skillCmd.AddCommand(skillInstallCmd)
	rootCmd.AddCommand(skillCmd)
}

func printSkillInstallSuccess(w io.Writer, result *skill.InstallResult) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "✓ Installed forge skill to %s\n", tildePath(result.InstalledPath))

	if result.Created {
		fmt.Fprintln(w, "  (Created directory ~/.claude/skills/)")
	}
	if result.BackupPath != "" {
		fmt.Fprintf(w, "  Backup created: %s\n", tildePath(result.BackupPath))
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "To use the skill:")
	fmt.Fprintln(w, "  1. Restart Claude Code (or open a new conversation)")
	fmt.Fprintln(w, "  2. Claude Code will now recognize forge as a scaffolding tool")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "The skill teaches Claude Code to:")
	fmt.Fprintln(w, "  - Use forge new when starting matching projects")
	fmt.Fprintln(w, "  - Use forge visualize when exploring codebases")
	fmt.Fprintln(w, "  - Use forge add to extend existing forge projects")
	fmt.Fprintln(w)
}

// tildePath shortens a path under the user's home directory to a "~/..."
// form for display, so users can paste it directly into another command.
func tildePath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if rel, ok := strings.CutPrefix(p, home); ok {
		return "~" + rel
	}
	return p
}
