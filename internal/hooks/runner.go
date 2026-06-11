package hooks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/kanukuntla-r/forge/internal/manifest"
	"github.com/kanukuntla-r/forge/internal/render"
)

// HookResult records the outcome of a single post-create hook.
type HookResult struct {
	Name    string
	Success bool
	// Output is non-empty only when capture=true and Success=false.
	Output string
}

// Run executes each hook in order in workDir.
// Each hook's Shell field is template-rendered with tmplCtx before execution.
// When capture is false, each hook's stdout and stderr stream live to the
// provided writers. When capture is true, output is buffered per hook;
// on failure the buffer is placed in HookResult.Output; on success it is
// discarded.
// Returns the partial results slice alongside the first required-hook error.
func Run(ctx context.Context, hs []manifest.Hook, workDir string, tmplCtx map[string]any, stdout, stderr io.Writer, capture bool) ([]HookResult, error) {
	results := make([]HookResult, 0, len(hs))

	for i, h := range hs {
		if !capture {
			fmt.Fprintf(stdout, "Running hook (%d/%d): %s\n", i+1, len(hs), h.Name)
		}

		renderedShell, err := render.Render(fmt.Sprintf("hook[%d].shell", i), h.Shell, tmplCtx)
		if err != nil {
			results = append(results, HookResult{Name: h.Name, Success: false})
			return results, fmt.Errorf("rendering hook %q: %w", h.Name, err)
		}

		cmd := exec.CommandContext(ctx, "sh", "-c", renderedShell)
		cmd.Dir = workDir

		var buf bytes.Buffer
		if capture {
			cmd.Stdout = &buf
			cmd.Stderr = &buf
		} else {
			cmd.Stdout = stdout
			cmd.Stderr = stderr
		}

		runErr := cmd.Run()
		res := HookResult{Name: h.Name, Success: runErr == nil}
		if runErr != nil && capture {
			res.Output = buf.String()
		}
		results = append(results, res)

		if runErr != nil {
			if h.Optional {
				if !capture {
					fmt.Fprintf(stderr, "✗ %s (optional hook failed, continuing): %v\n", h.Name, runErr)
				}
				continue
			}
			return results, fmt.Errorf("hook %q failed: %w", h.Name, runErr)
		}
		if !capture {
			fmt.Fprintf(stdout, "✓ %s\n", h.Name)
		}
	}
	return results, nil
}
