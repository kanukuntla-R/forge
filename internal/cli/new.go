package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/kanukuntla-r/forge/internal/blueprint"
	"github.com/kanukuntla-r/forge/internal/render"
)

var newCmd = &cobra.Command{
	Use:   "new <blueprint> <name>",
	Short: "Scaffold a new project from a blueprint",
	Long: `Scaffold a new project from a blueprint.

Examples:
  forge new hackathon-app my-idea
  forge new hackathon-app my-idea --var with_ai=false
  forge new hackathon-app my-idea --var package_manager=npm --verbose`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		varFlags, _ := cmd.Flags().GetStringArray("var")
		verbose, _ := cmd.Flags().GetBool("verbose")
		return runNew(cmd.Context(), cmd.OutOrStdout(), args[0], args[1], varFlags, verbose)
	},
}

func init() {
	newCmd.Flags().StringArray("var", nil, "Set a variable (KEY=VALUE, repeatable)")
	newCmd.Flags().BoolP("verbose", "v", false, "Print the list of files written")
	rootCmd.AddCommand(newCmd)
}

// runNew contains the real logic for `forge new`. RunE is a thin wrapper so
// this function is directly testable without constructing a cobra.Command.
func runNew(ctx context.Context, out io.Writer, blueprintName, projectName string, varFlags []string, verbose bool) error {
	_ = ctx // reserved for post-create hooks in M4

	r, err := blueprint.NewRegistry()
	if err != nil {
		return fmt.Errorf("loading blueprints: %w", err)
	}

	bp, err := r.Find(blueprintName)
	if err != nil {
		return err // Find already produces a user-friendly message
	}

	values, err := render.Resolve(bp.Manifest, nil, varFlags, false)
	if err != nil {
		return fmt.Errorf("resolving variables: %w", err)
	}
	values["name"] = projectName

	tmplCtx := render.ToTemplateContext(values)

	defaultPath := bp.Manifest.Target.DefaultPath
	if defaultPath == "" {
		defaultPath = "./{{ .Name }}"
	}
	targetPath, err := render.Render("target_path", defaultPath, tmplCtx)
	if err != nil {
		return fmt.Errorf("rendering target path: %w", err)
	}

	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("target %q already exists; refusing to overwrite", targetPath)
	}

	written, err := render.WriteBlueprint(bp, tmplCtx, targetPath)
	if err != nil {
		return fmt.Errorf("writing blueprint: %w", err)
	}

	fmt.Fprintf(out, "Created %s project at %s\n", blueprintName, targetPath)
	fmt.Fprintf(out, "%d files written\n", len(written))
	if verbose {
		for _, f := range written {
			fmt.Fprintf(out, "  %s\n", f)
		}
	}
	return nil
}
