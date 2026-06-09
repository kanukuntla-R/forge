package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	cterm "github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/kanukuntla-r/forge/internal/blueprint"
	"github.com/kanukuntla-r/forge/internal/render"
)

var newCmd = &cobra.Command{
	Use:   "new <blueprint> [name]",
	Short: "Scaffold a new project from a blueprint",
	Long: `Scaffold a new project from a blueprint.

Examples:
  forge new hackathon-app my-idea
  forge new hackathon-app my-idea --var with_ai=false
  forge new hackathon-app my-idea --var package_manager=npm --verbose
  echo '{"with_ai":false}' | forge new hackathon-app my-idea --json`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		varFlags, _ := cmd.Flags().GetStringArray("var")
		verbose, _ := cmd.Flags().GetBool("verbose")
		yes, _ := cmd.Flags().GetBool("yes")
		useJSON, _ := cmd.Flags().GetBool("json")

		blueprintName := args[0]
		projectName := ""
		if len(args) >= 2 {
			projectName = args[1]
		}

		return runNew(cmd.Context(), cmd.OutOrStdout(), blueprintName, projectName, varFlags, runNewOptions{
			verbose:     verbose,
			interactive: cterm.IsTerminal(os.Stdout.Fd()) && !yes,
			useJSON:     useJSON,
			stdin:       os.Stdin,
		})
	},
}

func init() {
	newCmd.Flags().StringArray("var", nil, "Set a variable (KEY=VALUE, repeatable)")
	newCmd.Flags().BoolP("verbose", "v", false, "Print the list of files written")
	newCmd.Flags().Bool("yes", false, "Accept all defaults; disable interactive prompts")
	newCmd.Flags().Bool("json", false, "Read variables from stdin as JSON (suppresses interactive prompts)")
	rootCmd.AddCommand(newCmd)
}

// runNewOptions holds flag-derived and test-injectable settings for runNew.
type runNewOptions struct {
	verbose      bool
	interactive  bool
	useJSON      bool
	stdin        io.Reader        // source for --json reads; nil defaults to os.Stdin
	nameFormOpts []render.FormOption
}

// namePattern matches valid project names: starts with a letter, followed by
// letters, digits, underscores, or hyphens.
var namePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// runNew contains the real logic for `forge new`. RunE is a thin wrapper so
// this function is directly testable without constructing a cobra.Command.
func runNew(ctx context.Context, out io.Writer, blueprintName, projectName string, varFlags []string, opts runNewOptions) error {
	_ = ctx // reserved for post-create hooks in M4

	r, err := blueprint.NewRegistry()
	if err != nil {
		return fmt.Errorf("loading blueprints: %w", err)
	}

	bp, err := r.Find(blueprintName)
	if err != nil {
		return err // Find already produces a user-friendly message
	}

	// Read JSON variables from stdin if --json was passed.
	var jsonVars map[string]any
	if opts.useJSON {
		opts.interactive = false // JSON mode is always non-interactive

		stdin := opts.stdin
		if stdin == nil {
			stdin = os.Stdin
		}

		// Guard: refuse to block waiting on a live terminal.
		if f, ok := stdin.(*os.File); ok && cterm.IsTerminal(f.Fd()) {
			return fmt.Errorf("--json requires JSON input on stdin; pipe a JSON object in")
		}

		data, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		if strings.TrimSpace(string(data)) == "" {
			return fmt.Errorf("--json was passed but stdin was empty; expected a JSON object")
		}
		if err := json.Unmarshal(data, &jsonVars); err != nil {
			return fmt.Errorf("invalid JSON on stdin: %w", err)
		}
	}

	if projectName == "" {
		if !opts.interactive {
			return fmt.Errorf("project name is required when stdout is not a terminal; pass it as an argument")
		}
		projectName, err = promptProjectName(opts.nameFormOpts...)
		if err != nil {
			return err
		}
	}

	values, err := render.Resolve(bp.Manifest, jsonVars, varFlags, opts.interactive)
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
	if opts.verbose {
		for _, f := range written {
			fmt.Fprintf(out, "  %s\n", f)
		}
	}
	return nil
}

// promptProjectName prompts for a project name with name validation.
// opts are applied to the huh form before running, primarily for test IO injection.
func promptProjectName(opts ...render.FormOption) (string, error) {
	var name string
	field := huh.NewInput().
		Title("Project name").
		Value(&name).
		Validate(func(s string) error {
			if !namePattern.MatchString(s) {
				return fmt.Errorf("must start with a letter and contain only letters, digits, _ or -")
			}
			return nil
		})
	form := huh.NewForm(huh.NewGroup(field))
	for _, opt := range opts {
		form = opt(form)
	}
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("prompt: %w", err)
	}
	return name, nil
}
