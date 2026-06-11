package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const staleHint = `Note: your knowledge graph was created at scaffold time and may be stale
relative to your current code. To refresh it, you can ask Claude Code or
another AI assistant to update .understand-anything/knowledge-graph.json
based on your current code. Full automated refresh is planned for M8.`

const fallbackFmt = `Understand-Anything not found on PATH.

Your knowledge graph is at: %s

To visualize it, install Understand-Anything:
  https://github.com/understand-anything/cli
`

// visualizeCmd implements `forge visualize [path]`.
// It opens the Understand-Anything dashboard for the project at [path] (or cwd).
var visualizeCmd = &cobra.Command{
	Use:   "visualize [path]",
	Short: "Open the Understand-Anything dashboard for a forge project",
	Long: `Open the Understand-Anything dashboard for the project at [path] (or cwd).

Requires .understand-anything/knowledge-graph.json to exist. forge writes
that file at scaffold time, so any forge-created project is visualizable.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := ""
		if len(args) > 0 {
			path = args[0]
		}
		return runVisualize(path, cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(visualizeCmd)
}

func runVisualize(path string, out io.Writer) error {
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
	}

	graphPath := filepath.Join(path, ".understand-anything", "knowledge-graph.json")
	if _, err := os.Stat(graphPath); err != nil {
		return fmt.Errorf("knowledge graph not found at %q; run forge new to scaffold a project with a knowledge graph", graphPath)
	}

	stale, err := isStale(graphPath, path)
	if err != nil {
		return fmt.Errorf("checking graph staleness: %w", err)
	}
	if stale {
		fmt.Fprintln(out, staleHint)
	}

	if viz := findVisualizer(); viz != "" {
		cmd := exec.Command(viz)
		cmd.Dir = path
		cmd.Stdout = out
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("understand-anything: %w", err)
		}
		return nil
	}

	absGraph, err := filepath.Abs(graphPath)
	if err != nil {
		absGraph = graphPath
	}
	fmt.Fprintf(out, fallbackFmt, absGraph)
	return nil
}

// isStale reports whether any source file in projectDir is newer than
// the knowledge graph at graphPath by more than 1 second.
// Directories and files whose names start with "." and "node_modules" are skipped.
func isStale(graphPath, projectDir string) (bool, error) {
	graphInfo, err := os.Stat(graphPath)
	if err != nil {
		return false, err
	}
	graphMtime := graphInfo.ModTime()

	stale := false
	err = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == projectDir {
			return nil
		}
		name := info.Name()
		if info.IsDir() {
			if strings.HasPrefix(name, ".") || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		if info.ModTime().After(graphMtime.Add(time.Second)) {
			stale = true
		}
		return nil
	})
	return stale, err
}

// findVisualizer returns the path to the understand-anything binary,
// or "" if it is not found on PATH.
func findVisualizer() string {
	p, err := exec.LookPath("understand-anything")
	if err != nil {
		return ""
	}
	return p
}
