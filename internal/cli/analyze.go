package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [path]",
	Short: "Analyze a codebase and write analysis.json",
	Long: `Walk the codebase, extract architectural information, and write
the result to .forge/analysis.json in the analyzed directory.

For v0.2 M8.1, this is a file inventory only. Deeper analysis (imports,
exports, declarations) comes in later chunks with language-specific adapters.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		asJSON, _ := cmd.Flags().GetBool("json")
		return runAnalyze(cmd.OutOrStdout(), cmd.ErrOrStderr(), path, asJSON)
	},
}

func init() {
	analyzeCmd.Flags().Bool("json", false, "Emit the analysis JSON to stdout instead of writing to disk")
	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyze(out, errOut io.Writer, path string, asJSON bool) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	start := time.Now()
	analysis, warnings, err := analyzer.AnalyzeProject(absPath)
	if err != nil {
		return err
	}

	for _, e := range warnings {
		fmt.Fprintf(errOut, "forge: warning: %v\n", e)
	}

	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling analysis: %w", err)
	}

	if asJSON {
		fmt.Fprintln(out, string(data))
		return nil
	}

	outPath := filepath.Join(absPath, ".forge", "analysis.json")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("creating .forge directory: %w", err)
	}
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return fmt.Errorf("writing analysis: %w", err)
	}

	elapsed := time.Since(start)
	fmt.Fprintf(out, "Analyzed %d %s in %s\n",
		len(analysis.Files),
		plural(len(analysis.Files), "file", "files"),
		elapsed.Round(time.Millisecond))
	fmt.Fprintln(out, "Analysis written to .forge/analysis.json")
	return nil
}
