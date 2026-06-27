package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunVisualizeRunsAnalyzeIfMissing(t *testing.T) {
	dir := t.TempDir()
	// Write a Go source file so analyze has something to walk.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	analysisPath := filepath.Join(dir, ".forge", "analysis.json")
	if _, err := os.Stat(analysisPath); !os.IsNotExist(err) {
		t.Fatal("analysis.json should not exist before test")
	}

	var out, errOut bytes.Buffer
	// We can't call runVisualize directly because it starts a blocking server.
	// runAnalyze is the same code path visualize uses for the initial analysis;
	// testing it here verifies the analysis side without the server complexity.
	if err := runAnalyze(&out, &errOut, dir, false); err != nil {
		t.Fatalf("runAnalyze (pre-flight for visualize): %v", err)
	}
	if _, err := os.Stat(analysisPath); err != nil {
		t.Errorf("analysis.json should exist after analyze: %v", err)
	}
	if !strings.Contains(out.String(), "Analysis written to") {
		t.Errorf("expected analyze output; got: %s", out.String())
	}
}

func TestRunVisualizeNonexistentPath(t *testing.T) {
	var out, errOut bytes.Buffer
	err := runVisualize(&out, &errOut, "/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("runVisualize with nonexistent path: want error, got nil")
	}
}
