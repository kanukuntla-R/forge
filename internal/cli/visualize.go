package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/kanukuntla-r/forge/internal/dashboard"
)

var visualizeCmd = &cobra.Command{
	Use:   "visualize [path]",
	Short: "Open the forge dashboard for an analyzed project",
	Long: `Runs analysis and opens an interactive dashboard in your browser.
The dashboard updates live as files change in the project.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		return runVisualize(cmd.OutOrStdout(), cmd.ErrOrStderr(), path)
	},
}

func init() {
	rootCmd.AddCommand(visualizeCmd)
}

func runVisualize(out, errOut io.Writer, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	analysisPath := filepath.Join(absPath, ".forge", "analysis.json")

	fmt.Fprintf(out, "Analyzing %s...\n", filepath.Base(absPath))
	if err := runAnalyze(out, errOut, absPath, false); err != nil {
		return fmt.Errorf("analyzing project: %w", err)
	}

	srv, err := dashboard.NewServer(absPath, analysisPath)
	if err != nil {
		return fmt.Errorf("starting dashboard: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			fmt.Fprintln(errOut, "\nShutting down...")
			cancel()
		case <-srv.IdleShutdownCh():
			fmt.Fprintln(out, "\nNo browser connected, shutting down.")
			cancel()
		}
	}()

	fmt.Fprintf(out, "Dashboard running at %s\n", srv.URL())
	fmt.Fprintln(out, "Press Ctrl+C to stop.")

	if err := dashboard.OpenBrowser(srv.URL()); err != nil {
		fmt.Fprintf(errOut, "forge: could not open browser: %v\n", err)
		fmt.Fprintf(errOut, "Open manually: %s\n", srv.URL())
	}

	if err := srv.Start(ctx); err != nil {
		return err
	}
	fmt.Fprintln(out, "Dashboard stopped.")
	return nil
}
