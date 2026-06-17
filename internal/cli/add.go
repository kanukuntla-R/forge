package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"

	cterm "github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/kanukuntla-r/forge/internal/blueprint"
	"github.com/kanukuntla-r/forge/internal/graph"
	"github.com/kanukuntla-r/forge/internal/manifest"
	"github.com/kanukuntla-r/forge/internal/project"
	"github.com/kanukuntla-r/forge/internal/render"
)

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
		varFlags, _ := cmd.Flags().GetStringArray("var")
		verbose, _ := cmd.Flags().GetBool("verbose")
		yes, _ := cmd.Flags().GetBool("yes")
		useJSON, _ := cmd.Flags().GetBool("json")
		noGraph, _ := cmd.Flags().GetBool("no-graph")
		toPath, _ := cmd.Flags().GetString("to")

		extensionName := args[0]
		positionalArgs := args[1:]

		return runAdd(cmd.Context(), cmd.OutOrStdout(), extensionName, positionalArgs, varFlags, runAddOptions{
			verbose:     verbose,
			interactive: cterm.IsTerminal(os.Stdout.Fd()) && !yes,
			useJSON:     useJSON,
			noGraph:     noGraph,
			toPath:      toPath,
			stdin:       os.Stdin,
			stderr:      cmd.ErrOrStderr(),
		})
	},
}

func init() {
	addCmd.Flags().StringArray("var", nil, "Set an extension arg (KEY=VALUE, repeatable)")
	addCmd.Flags().BoolP("verbose", "v", false, "Print the list of files written")
	addCmd.Flags().Bool("yes", false, "Accept all defaults; disable interactive prompts")
	addCmd.Flags().Bool("json", false, "Output structured JSON; optionally read args from stdin as JSON")
	addCmd.Flags().Bool("no-graph", false, "Skip merging the graph fragment")
	addCmd.Flags().String("to", "", "Operate on a different project root (default: cwd)")
	rootCmd.AddCommand(addCmd)
}

// runAddOptions holds flag-derived and test-injectable settings for runAdd.
type runAddOptions struct {
	verbose      bool
	interactive  bool
	useJSON      bool
	noGraph      bool
	toPath       string
	stdin        io.Reader
	stderr       io.Writer
	nameFormOpts []render.FormOption
}

// addResult is the JSON envelope emitted when --json is set.
type addResult struct {
	Extension    string           `json:"extension"`
	ProjectName  string           `json:"project_name"`
	ProjectRoot  string           `json:"project_root"`
	FilesCreated []string         `json:"files_created"`
	GraphUpdates *addGraphUpdates `json:"graph_updates,omitempty"`
	Args         map[string]any   `json:"args"`
	Error        string           `json:"error,omitempty"`
	Phase        string           `json:"phase,omitempty"`
}

type addGraphUpdates struct {
	NodesAdded   int `json:"nodes_added"`
	NodesSkipped int `json:"nodes_skipped"`
	EdgesAdded   int `json:"edges_added"`
	EdgesSkipped int `json:"edges_skipped"`
}

func emitAddJSON(out io.Writer, result addResult, err error, phase string) {
	if err != nil {
		result.Error = err.Error()
		result.Phase = phase
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Fprintln(out, string(data))
}

// runAdd contains the real logic for `forge add`. RunE is a thin wrapper so
// this function is directly testable without constructing a cobra.Command.
func runAdd(_ context.Context, out io.Writer, extensionName string, positionalArgs []string, varFlags []string, opts runAddOptions) error {
	result := addResult{Extension: extensionName}

	// 1. Resolve project root.
	root := opts.toPath
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			err = fmt.Errorf("getting working directory: %w", err)
			if opts.useJSON {
				emitAddJSON(out, result, err, "project_detection")
			}
			return err
		}
	}
	result.ProjectRoot = root

	det, err := project.Detect(root)
	if err != nil {
		if opts.useJSON {
			emitAddJSON(out, result, err, "project_detection")
		}
		return err
	}
	if det == nil {
		err := fmt.Errorf("not inside a forge project; run forge new <blueprint> first")
		if opts.useJSON {
			emitAddJSON(out, result, err, "project_detection")
		}
		return err
	}
	result.ProjectRoot = det.Root

	// 2. Load blueprint.
	reg, err := blueprint.NewRegistry()
	if err != nil {
		err = fmt.Errorf("loading blueprints: %w", err)
		if opts.useJSON {
			emitAddJSON(out, result, err, "blueprint")
		}
		return err
	}
	bp, err := reg.Find(det.Project.Blueprint)
	if err != nil {
		if opts.useJSON {
			emitAddJSON(out, result, err, "blueprint")
		}
		return err
	}

	// 3. Find extension.
	var ext *manifest.Extension
	for i := range bp.Manifest.Extensions {
		if bp.Manifest.Extensions[i].Name == extensionName {
			ext = &bp.Manifest.Extensions[i]
			break
		}
	}
	if ext == nil {
		names := make([]string, len(bp.Manifest.Extensions))
		for i, e := range bp.Manifest.Extensions {
			names[i] = e.Name
		}
		err := fmt.Errorf("extension %q not found; available: %s", extensionName, strings.Join(names, ", "))
		if opts.useJSON {
			emitAddJSON(out, result, err, "extension_lookup")
		}
		return err
	}

	// 4. Resolve extension args.
	if len(positionalArgs) > len(ext.Args) {
		err := fmt.Errorf("too many positional arguments: extension %q has %d args but %d were provided",
			ext.Name, len(ext.Args), len(positionalArgs))
		if opts.useJSON {
			emitAddJSON(out, result, err, "args")
		}
		return err
	}

	// Map positionals to --var flags by order, then append any explicit --var flags.
	extraFlags := make([]string, 0, len(positionalArgs))
	for i, arg := range ext.Args {
		if i < len(positionalArgs) {
			extraFlags = append(extraFlags, arg.Name+"="+positionalArgs[i])
		}
	}
	allFlags := append(extraFlags, varFlags...)

	// Build a synthetic manifest so render.Resolve can handle arg resolution
	// and interactive prompts identically to blueprint variables.
	syntheticVars := make([]manifest.Variable, len(ext.Args))
	for i, a := range ext.Args {
		syntheticVars[i] = manifest.Variable{
			Name:    a.Name,
			Prompt:  a.Prompt,
			Type:    manifest.VarTypeString,
			Pattern: a.Pattern,
		}
	}
	fakeManifest := &manifest.Manifest{
		Name:      ext.Name,
		Kind:      bp.Manifest.Kind,
		Variables: syntheticVars,
	}

	// Read JSON stdin if --json mode.
	var jsonVars map[string]any
	if opts.useJSON {
		opts.interactive = false
		stdin := opts.stdin
		if stdin == nil {
			stdin = os.Stdin
		}
		isTTY := false
		if f, ok := stdin.(*os.File); ok {
			isTTY = cterm.IsTerminal(f.Fd())
		}
		if !isTTY {
			data, err := io.ReadAll(stdin)
			if err != nil {
				err = fmt.Errorf("reading stdin: %w", err)
				emitAddJSON(out, result, err, "args")
				return err
			}
			if strings.TrimSpace(string(data)) != "" {
				if err := json.Unmarshal(data, &jsonVars); err != nil {
					err = fmt.Errorf("invalid JSON on stdin: %w", err)
					emitAddJSON(out, result, err, "args")
					return err
				}
			}
		}
	}

	resolvedArgs, err := render.Resolve(fakeManifest, jsonVars, allFlags, opts.interactive)
	if err != nil {
		err = fmt.Errorf("resolving args: %w", err)
		if opts.useJSON {
			emitAddJSON(out, result, err, "args")
		}
		return err
	}
	result.Args = resolvedArgs

	tmplCtx := render.ToTemplateContext(resolvedArgs)

	// 5. Render extension template into the existing project root.
	subFS, err := fs.Sub(bp.FS, "extensions/"+ext.Name)
	if err != nil {
		err = fmt.Errorf("accessing extension %q: %w", ext.Name, err)
		if opts.useJSON {
			emitAddJSON(out, result, err, "scaffolding")
		}
		return err
	}
	subBP := &blueprint.Blueprint{
		Manifest: bp.Manifest,
		FS:       subFS,
		Source:   bp.Source,
	}
	written, err := render.WriteIntoExisting(subBP, tmplCtx, det.Root)
	if err != nil {
		err = fmt.Errorf("rendering extension template: %w", err)
		if opts.useJSON {
			emitAddJSON(out, result, err, "scaffolding")
		}
		return err
	}
	if written == nil {
		written = []string{}
	}
	result.FilesCreated = written

	// 6. Merge graph fragment (unless --no-graph or extension has none).
	if !opts.noGraph && ext.GraphFragment != "" {
		fragSrc, err := fs.ReadFile(bp.FS, ext.GraphFragment)
		if err != nil {
			err = fmt.Errorf("reading graph fragment: %w", err)
			if opts.useJSON {
				emitAddJSON(out, result, err, "graph")
			}
			return err
		}
		renderedFrag, err := render.Render("graph-fragment", string(fragSrc), tmplCtx)
		if err != nil {
			err = fmt.Errorf("rendering graph fragment: %w", err)
			if opts.useJSON {
				emitAddJSON(out, result, err, "graph")
			}
			return err
		}
		var frag graph.Graph
		if err := yaml.Unmarshal([]byte(renderedFrag), &frag); err != nil {
			err = fmt.Errorf("parsing graph fragment: %w", err)
			if opts.useJSON {
				emitAddJSON(out, result, err, "graph")
			}
			return err
		}
		existing, err := graph.Read(det.Root)
		if err != nil {
			err = fmt.Errorf("reading knowledge graph: %w", err)
			if opts.useJSON {
				emitAddJSON(out, result, err, "graph")
			}
			return err
		}
		mr := graph.Merge(existing, &frag)
		result.GraphUpdates = &addGraphUpdates{
			NodesAdded:   mr.NodesAdded,
			NodesSkipped: mr.NodesSkipped,
			EdgesAdded:   mr.EdgesAdded,
			EdgesSkipped: mr.EdgesSkipped,
		}
		if err := graph.Write(existing, det.Root); err != nil {
			err = fmt.Errorf("writing knowledge graph: %w", err)
			if opts.useJSON {
				emitAddJSON(out, result, err, "graph")
			}
			return err
		}
	}

	// 7. Update project marker.
	det.Project.ExtensionsApplied = append(det.Project.ExtensionsApplied, project.ExtensionApplication{
		Name:      ext.Name,
		AppliedAt: time.Now().UTC().Format(time.RFC3339),
		Args:      resolvedArgs,
	})
	if err := project.Write(det.Root, det.Project); err != nil {
		err = fmt.Errorf("updating project marker: %w", err)
		if opts.useJSON {
			emitAddJSON(out, result, err, "marker")
		}
		return err
	}

	// 8. Output.
	if opts.useJSON {
		emitAddJSON(out, result, nil, "")
		return nil
	}

	projectName := det.Project.Blueprint
	if name, ok := det.Project.Variables["name"].(string); ok && name != "" {
		projectName = name
	}

	fileWord := "files"
	if len(written) == 1 {
		fileWord = "file"
	}
	fmt.Fprintf(out, "Added %s to %s\n", ext.Name, projectName)
	fmt.Fprintf(out, "%d %s written\n", len(written), fileWord)
	for _, f := range written {
		fmt.Fprintf(out, "  %s\n", f)
	}
	if result.GraphUpdates != nil {
		gu := result.GraphUpdates
		fmt.Fprintf(out, "Knowledge graph updated: %d %s added, %d %s added\n",
			gu.NodesAdded, plural(gu.NodesAdded, "node", "nodes"),
			gu.EdgesAdded, plural(gu.EdgesAdded, "edge", "edges"))
		if gu.NodesSkipped > 0 || gu.EdgesSkipped > 0 {
			fmt.Fprintf(out, "  (skipped %d duplicate %s, %d duplicate %s)\n",
				gu.NodesSkipped, plural(gu.NodesSkipped, "node", "nodes"),
				gu.EdgesSkipped, plural(gu.EdgesSkipped, "edge", "edges"))
		}
	}
	fmt.Fprintln(out, "Project marker updated")
	return nil
}
