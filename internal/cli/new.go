package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	cterm "github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/kanukuntla-r/forge/internal/blueprint"
	"github.com/kanukuntla-r/forge/internal/graph"
	"github.com/kanukuntla-r/forge/internal/hooks"
	"github.com/kanukuntla-r/forge/internal/project"
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
		noHooks, _ := cmd.Flags().GetBool("no-hooks")

		blueprintName := args[0]
		projectName := ""
		if len(args) >= 2 {
			projectName = args[1]
		}

		return runNew(cmd.Context(), cmd.OutOrStdout(), blueprintName, projectName, varFlags, runNewOptions{
			verbose:     verbose,
			interactive: cterm.IsTerminal(os.Stdout.Fd()) && !yes,
			useJSON:     useJSON,
			noHooks:     noHooks,
			stdin:       os.Stdin,
			stderr:      cmd.ErrOrStderr(),
		})
	},
}

func init() {
	newCmd.Flags().StringArray("var", nil, "Set a variable (KEY=VALUE, repeatable)")
	newCmd.Flags().BoolP("verbose", "v", false, "Print the list of files written")
	newCmd.Flags().Bool("yes", false, "Accept all defaults; disable interactive prompts")
	newCmd.Flags().Bool("json", false, "Output structured JSON; optionally read variables from stdin as JSON")
	newCmd.Flags().Bool("no-hooks", false, "skip post-create hooks (e.g. pnpm install)")
	rootCmd.AddCommand(newCmd)
}

// runNewOptions holds flag-derived and test-injectable settings for runNew.
type runNewOptions struct {
	verbose      bool
	interactive  bool
	useJSON      bool
	noHooks      bool
	stdin        io.Reader  // source for --json reads; nil defaults to os.Stdin
	stderr       io.Writer  // hook stderr; nil defaults to os.Stderr
	nameFormOpts []render.FormOption
}

// newResult is the JSON envelope emitted when --json is set.
type newResult struct {
	Blueprint    string         `json:"blueprint"`
	Path         string         `json:"path"`
	FilesCreated []string       `json:"files_created"`
	GraphPath    string         `json:"graph_path,omitempty"`
	MarkerPath   string         `json:"marker_path"`
	HooksRun     []hookResult   `json:"hooks_run,omitempty"`
	Variables    map[string]any `json:"variables"`
	Error        string         `json:"error,omitempty"`
	Phase        string         `json:"phase,omitempty"`
}

type hookResult struct {
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
}

// emitJSON marshals result to out, setting Error and Phase when err is non-nil.
func emitJSON(out io.Writer, result newResult, err error, phase string) {
	if err != nil {
		result.Error = err.Error()
		result.Phase = phase
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Fprintln(out, string(data))
}

// namePattern matches valid project names: starts with a letter, followed by
// letters, digits, underscores, or hyphens.
var namePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// runNew contains the real logic for `forge new`. RunE is a thin wrapper so
// this function is directly testable without constructing a cobra.Command.
func runNew(ctx context.Context, out io.Writer, blueprintName, projectName string, varFlags []string, opts runNewOptions) error {
	partialResult := newResult{Blueprint: blueprintName}

	r, err := blueprint.NewRegistry()
	if err != nil {
		wrappedErr := fmt.Errorf("loading blueprints: %w", err)
		if opts.useJSON {
			emitJSON(out, partialResult, wrappedErr, "manifest")
		}
		return wrappedErr
	}

	bp, err := r.Find(blueprintName)
	if err != nil {
		if opts.useJSON {
			emitJSON(out, partialResult, err, "manifest")
		}
		return err
	}

	return runNewFromBlueprint(ctx, out, bp, projectName, varFlags, opts)
}

// runNewFromBlueprint executes scaffold phases starting from a resolved blueprint.
// Separated from runNew so tests can inject a custom blueprint without going
// through the embedded registry.
func runNewFromBlueprint(ctx context.Context, out io.Writer, bp *blueprint.Blueprint, projectName string, varFlags []string, opts runNewOptions) error {
	result := newResult{Blueprint: bp.Manifest.Name}

	// Phase: variables — read JSON stdin and resolve all vars.
	var jsonVars map[string]any
	if opts.useJSON {
		opts.interactive = false

		stdin := opts.stdin
		if stdin == nil {
			stdin = os.Stdin
		}

		// Read stdin only when it is not a live terminal; a TTY means no pipe
		// was attached, so we simply proceed with no JSON vars (use defaults).
		isTTY := false
		if f, ok := stdin.(*os.File); ok {
			isTTY = cterm.IsTerminal(f.Fd())
		}
		if !isTTY {
			data, err := io.ReadAll(stdin)
			if err != nil {
				wrappedErr := fmt.Errorf("reading stdin: %w", err)
				emitJSON(out, result, wrappedErr, "variables")
				return wrappedErr
			}
			if strings.TrimSpace(string(data)) != "" {
				if err := json.Unmarshal(data, &jsonVars); err != nil {
					wrappedErr := fmt.Errorf("invalid JSON on stdin: %w", err)
					emitJSON(out, result, wrappedErr, "variables")
					return wrappedErr
				}
			}
		}
	}

	if projectName == "" {
		if !opts.interactive {
			err := fmt.Errorf("project name is required when stdout is not a terminal; pass it as an argument")
			if opts.useJSON {
				emitJSON(out, result, err, "variables")
			}
			return err
		}
		var err error
		projectName, err = promptProjectName(opts.nameFormOpts...)
		if err != nil {
			if opts.useJSON {
				emitJSON(out, result, err, "variables")
			}
			return err
		}
	}

	values, err := render.Resolve(bp.Manifest, jsonVars, varFlags, opts.interactive)
	if err != nil {
		wrappedErr := fmt.Errorf("resolving variables: %w", err)
		if opts.useJSON {
			emitJSON(out, result, wrappedErr, "variables")
		}
		return wrappedErr
	}
	values["name"] = projectName
	result.Variables = values

	tmplCtx := render.ToTemplateContext(values)

	defaultPath := bp.Manifest.Target.DefaultPath
	if defaultPath == "" {
		defaultPath = "./{{ .Name }}"
	}
	targetPath, err := render.Render("target_path", defaultPath, tmplCtx)
	if err != nil {
		wrappedErr := fmt.Errorf("rendering target path: %w", err)
		if opts.useJSON {
			emitJSON(out, result, wrappedErr, "variables")
		}
		return wrappedErr
	}

	absPath, absErr := filepath.Abs(targetPath)
	if absErr != nil {
		absPath = targetPath
	}
	result.Path = absPath

	if _, err := os.Stat(targetPath); err == nil {
		err := fmt.Errorf("target %q already exists; refusing to overwrite", targetPath)
		if opts.useJSON {
			emitJSON(out, result, err, "scaffolding")
		}
		return err
	}

	// Phase: scaffolding
	written, err := render.WriteBlueprint(bp, tmplCtx, targetPath)
	if err != nil {
		wrappedErr := fmt.Errorf("writing blueprint: %w", err)
		if opts.useJSON {
			emitJSON(out, result, wrappedErr, "scaffolding")
		}
		return wrappedErr
	}
	if written == nil {
		written = []string{}
	}
	result.FilesCreated = written

	// Phase: graph
	g, err := graph.Render(bp, tmplCtx)
	if err != nil {
		wrappedErr := fmt.Errorf("rendering knowledge graph: %w", err)
		if opts.useJSON {
			emitJSON(out, result, wrappedErr, "graph")
		}
		return wrappedErr
	}
	graphWritten := false
	if g != nil {
		if err := graph.Write(g, targetPath); err != nil {
			wrappedErr := fmt.Errorf("writing knowledge graph: %w", err)
			if opts.useJSON {
				emitJSON(out, result, wrappedErr, "graph")
			}
			return wrappedErr
		}
		graphWritten = true
		result.GraphPath = ".understand-anything/knowledge-graph.json"
	}

	// Phase: marker
	marker := project.Project{
		Blueprint:         bp.Manifest.Name,
		BlueprintVersion:  bp.Manifest.Version,
		ForgeVersion:      "0.1.0-dev",
		CreatedAt:         time.Now().UTC().Format(time.RFC3339),
		Variables:         values,
		ExtensionsApplied: []project.ExtensionApplication{},
	}
	if err := project.Write(targetPath, marker); err != nil {
		wrappedErr := fmt.Errorf("writing project marker: %w", err)
		if opts.useJSON {
			emitJSON(out, result, wrappedErr, "marker")
		}
		return wrappedErr
	}
	result.MarkerPath = ".forge/project.json"

	// Human-readable success lines (suppressed in JSON mode).
	if !opts.useJSON {
		fmt.Fprintf(out, "Created %s project at %s\n", bp.Manifest.Name, targetPath)
		fmt.Fprintf(out, "%d files written\n", len(written))
		if opts.verbose {
			for _, f := range written {
				fmt.Fprintf(out, "  %s\n", f)
			}
		}
		if graphWritten {
			fmt.Fprintln(out, "Knowledge graph written to .understand-anything/knowledge-graph.json")
		}
		fmt.Fprintln(out, "Project marker written to .forge/project.json")
	}

	// Phase: hooks
	if !opts.noHooks && len(bp.Manifest.PostCreate) > 0 {
		errOut := opts.stderr
		if errOut == nil {
			errOut = os.Stderr
		}
		hookResults, err := hooks.Run(ctx, bp.Manifest.PostCreate, targetPath, tmplCtx, out, errOut, opts.useJSON)
		for _, hr := range hookResults {
			result.HooksRun = append(result.HooksRun, hookResult{
				Name:    hr.Name,
				Success: hr.Success,
				Output:  hr.Output,
			})
		}
		if err != nil {
			wrappedErr := fmt.Errorf("running post-create hooks: %w", err)
			if opts.useJSON {
				emitJSON(out, result, wrappedErr, "hooks")
			}
			return wrappedErr
		}
	}

	if opts.useJSON {
		emitJSON(out, result, nil, "")
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
