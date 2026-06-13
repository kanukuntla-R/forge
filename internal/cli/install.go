package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kanukuntla-r/forge/internal/blueprint"
	"github.com/kanukuntla-r/forge/internal/manifest"
)

var installNameFlag string
var installJSON bool

var installCmd = &cobra.Command{
	Use:   "install <git-url>",
	Short: "Install a blueprint from a git repository",
	Long: `Install a blueprint from a git repository.

Clones the repo into ~/.config/forge/blueprints/<name>/ and validates
its manifest.yaml before accepting the install.

Examples:
  forge install https://github.com/some-user/go-cli-blueprint
  forge install git@github.com:some-user/go-cli-blueprint.git
  forge install https://github.com/some-user/repo --name custom-name`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]
		name := installNameFlag
		if name == "" {
			name = deriveName(url)
		}
		if name == "" {
			return fmt.Errorf("could not derive blueprint name from %q; use --name to specify", url)
		}
		blueprintsDir, err := blueprint.ConfigBlueprintsDir()
		if err != nil {
			return fmt.Errorf("determining config directory: %w", err)
		}
		return runInstall(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), url, name, blueprintsDir, installJSON)
	},
}

func init() {
	installCmd.Flags().StringVar(&installNameFlag, "name", "", "Override blueprint name (default: derived from URL)")
	installCmd.Flags().BoolVar(&installJSON, "json", false, "Output structured JSON")
	rootCmd.AddCommand(installCmd)
}

type installResult struct {
	Blueprint        string `json:"blueprint,omitempty"`
	Path             string `json:"path,omitempty"`
	SourceURL        string `json:"source_url,omitempty"`
	BlueprintVersion string `json:"blueprint_version,omitempty"`
	Error            string `json:"error,omitempty"`
	Phase            string `json:"phase,omitempty"`
}

func runInstall(_ context.Context, out, stderr io.Writer, url, name, blueprintsDir string, asJSON bool) error {
	target := filepath.Join(blueprintsDir, name)

	emitErr := func(err error, phase string) {
		if !asJSON {
			return
		}
		res := installResult{Error: err.Error(), Phase: phase}
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(out, string(data))
	}

	if info, statErr := os.Stat(target); statErr == nil && info.IsDir() {
		err := fmt.Errorf("blueprint %q already installed at %q; remove it first", name, target)
		emitErr(err, "check")
		return err
	}

	if err := os.MkdirAll(blueprintsDir, 0o755); err != nil {
		err = fmt.Errorf("creating blueprints directory: %w", err)
		emitErr(err, "setup")
		return err
	}

	cmd := exec.Command("git", "clone", "--depth=1", url, target)
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		err = fmt.Errorf("git clone failed: %w", err)
		emitErr(err, "clone")
		return err
	}

	m, err := manifest.ParseFile(filepath.Join(target, "manifest.yaml"))
	if err != nil {
		os.RemoveAll(target)
		err = fmt.Errorf("blueprint at %q is invalid: %w", target, err)
		emitErr(err, "validate")
		return err
	}

	if asJSON {
		res := installResult{
			Blueprint:        name,
			Path:             target,
			SourceURL:        url,
			BlueprintVersion: m.Version,
		}
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(out, string(data))
	} else {
		fmt.Fprintf(out, "Installed blueprint %q from %s\n", name, url)
	}
	return nil
}

// deriveName extracts a blueprint name from a git URL.
func deriveName(url string) string {
	url = strings.TrimRight(url, "/")
	last := url
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		last = url[i+1:]
	}
	return strings.TrimSuffix(last, ".git")
}
