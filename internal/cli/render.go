package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/kanukuntla-r/forge/internal/renderer"
)

var renderCmd = &cobra.Command{
	Use:   "render <component-path>",
	Short: "Render a single React component to a PNG screenshot",
	Long: `Renders a single component to a PNG using esbuild + Playwright.

Requires Node.js 18+ and Playwright's Chromium browser installed
(npx playwright install chromium). Output defaults to
.forge/renders/<ComponentName>.png.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")
		viewport, _ := cmd.Flags().GetString("viewport")
		verbose, _ := cmd.Flags().GetBool("verbose")
		return runRender(cmd.OutOrStdout(), args[0], output, viewport, verbose)
	},
}

func init() {
	renderCmd.Flags().String("output", "", "Output PNG path (default .forge/renders/<Name>.png)")
	renderCmd.Flags().String("viewport", "800x600", "Viewport size as WxH")
	renderCmd.Flags().Bool("verbose", false, "Show each pipeline stage")
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
