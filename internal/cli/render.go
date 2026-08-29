package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/kanukuntla-r/forge/internal/renderer"
)

var renderCmd = &cobra.Command{
	Use:   "render [component-path]",
	Short: "Render React component(s) to PNG screenshot(s)",
	Long: `Renders a single component, or (with no argument) every component
detected in the current project, to PNG via esbuild + Playwright.

Requires Node.js 18+ and Playwright's Chromium browser installed
(npx playwright install chromium). With no argument, auto-analyzes the
project if .forge/analysis.json is missing and renders every Next.js
component found under components/. Output defaults to
.forge/renders/<ComponentName>.png (single) or .forge/renders/ (batch).`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")
		viewport, _ := cmd.Flags().GetString("viewport")
		verbose, _ := cmd.Flags().GetBool("verbose")
		if len(args) == 0 {
			parallel, _ := cmd.Flags().GetInt("parallel")
			strict, _ := cmd.Flags().GetBool("strict")
			refresh, _ := cmd.Flags().GetBool("refresh")
			return runRenderBatch(cmd.OutOrStdout(), output, viewport, parallel, strict, refresh, verbose)
		}
		return runRender(cmd.OutOrStdout(), args[0], output, viewport, verbose)
	},
}

func init() {
	renderCmd.Flags().String("output", "", "Output path: PNG file (single component) or directory (batch mode)")
	renderCmd.Flags().String("viewport", "800x600", "Viewport size as WxH")
	renderCmd.Flags().Bool("verbose", false, "Show each pipeline stage")
	renderCmd.Flags().Int("parallel", 4, "Batch mode: components to render concurrently")
	renderCmd.Flags().Bool("strict", false, "Batch mode: exit non-zero if any component fails to render")
	renderCmd.Flags().Bool("refresh", false, "Batch mode: re-run the analyzer even if .forge/analysis.json exists")
	rootCmd.AddCommand(renderCmd)
}

func runRender(out io.Writer, componentPath, output, viewport string, verbose bool) error {
	outPath, err := renderer.Render(componentPath, renderer.Options{
		Output:   output,
		Viewport: viewport,
		Verbose:  verbose,
		Stdout:   out,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Rendered %s\n", outPath)
	return nil
}

func runRenderBatch(out io.Writer, output, viewport string, parallel int, strict, refresh, verbose bool) error {
	result, err := renderer.RenderBatch(renderer.BatchOptions{
		ProjectRoot: ".",
		OutputDir:   output,
		Parallelism: parallel,
		Refresh:     refresh,
		Viewport:    viewport,
		Verbose:     verbose,
		Stdout:      out,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Rendered %d/%d component(s) (%d failed) in %dms\n",
		result.Summary.Succeeded, result.Summary.Total, result.Summary.Failed, result.Summary.TotalTimeMS)
	if strict && result.Summary.Failed > 0 {
		return fmt.Errorf("forge render: %d of %d component(s) failed to render", result.Summary.Failed, result.Summary.Total)
	}
	return nil
}
