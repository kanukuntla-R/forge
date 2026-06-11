package hooks

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/kanukuntla-r/forge/internal/manifest"
	"github.com/kanukuntla-r/forge/internal/render"
)

// Run executes each hook in order in workDir.
// Each hook's Shell field is template-rendered with tmplCtx before execution.
// Output streams live to stdout/stderr.
// Returns the first non-optional failure; optional failures are reported
// via stderr but do not stop execution.
func Run(ctx context.Context, hooks []manifest.Hook, workDir string, tmplCtx map[string]any, stdout, stderr io.Writer) error {
	for i, h := range hooks {
		fmt.Fprintf(stdout, "Running hook (%d/%d): %s\n", i+1, len(hooks), h.Name)

		renderedShell, err := render.Render(fmt.Sprintf("hook[%d].shell", i), h.Shell, tmplCtx)
		if err != nil {
			return fmt.Errorf("rendering hook %q: %w", h.Name, err)
		}

		cmd := exec.CommandContext(ctx, "sh", "-c", renderedShell)
		cmd.Dir = workDir
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		if err := cmd.Run(); err != nil {
			if h.Optional {
				fmt.Fprintf(stderr, "✗ %s (optional hook failed, continuing): %v\n", h.Name, err)
				continue
			}
			return fmt.Errorf("hook %q failed: %w", h.Name, err)
		}
		fmt.Fprintf(stdout, "✓ %s\n", h.Name)
	}
	return nil
}
