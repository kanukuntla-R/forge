package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupProjectWithGraph(t *testing.T) (dir, graphPath string) {
	t.Helper()
	dir = t.TempDir()
	graphDir := filepath.Join(dir, ".understand-anything")
	if err := os.MkdirAll(graphDir, 0o755); err != nil {
		t.Fatalf("creating graph dir: %v", err)
	}
	graphPath = filepath.Join(graphDir, "knowledge-graph.json")
	if err := os.WriteFile(graphPath, []byte(`{"version":"1.0"}`), 0o644); err != nil {
		t.Fatalf("creating graph file: %v", err)
	}
	return dir, graphPath
}

func TestRunVisualizeNoGraphFile(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	err := runVisualize(dir, &out)
	if err == nil {
		t.Fatal("runVisualize() error = nil, want non-nil for missing graph")
	}
	if !strings.Contains(err.Error(), "knowledge-graph.json") {
		t.Errorf("error should mention knowledge-graph.json; got: %v", err)
	}
	if !strings.Contains(err.Error(), "forge new") {
		t.Errorf("error should suggest forge new; got: %v", err)
	}
}

func TestRunVisualizeFreshGraph(t *testing.T) {
	dir, graphPath := setupProjectWithGraph(t)

	srcFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcFile, []byte("package main"), 0o644); err != nil {
		t.Fatalf("creating source file: %v", err)
	}

	now := time.Now()
	os.Chtimes(srcFile, now.Add(-10*time.Second), now.Add(-10*time.Second))
	os.Chtimes(graphPath, now.Add(-2*time.Second), now.Add(-2*time.Second))

	var out bytes.Buffer
	if err := runVisualize(dir, &out); err != nil {
		t.Fatalf("runVisualize() error = %v", err)
	}

	output := out.String()
	if strings.Contains(output, "stale") {
		t.Errorf("output should not contain staleness hint for fresh graph; got:\n%s", output)
	}
	if !strings.Contains(output, "Understand-Anything not found on PATH") {
		t.Errorf("output should contain fallback message; got:\n%s", output)
	}
}

func TestRunVisualizeStaleGraph(t *testing.T) {
	dir, graphPath := setupProjectWithGraph(t)

	srcFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcFile, []byte("package main"), 0o644); err != nil {
		t.Fatalf("creating source file: %v", err)
	}

	now := time.Now()
	os.Chtimes(graphPath, now.Add(-10*time.Second), now.Add(-10*time.Second))
	os.Chtimes(srcFile, now.Add(-2*time.Second), now.Add(-2*time.Second))

	var out bytes.Buffer
	if err := runVisualize(dir, &out); err != nil {
		t.Fatalf("runVisualize() error = %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "may be stale") {
		t.Errorf("output should contain staleness hint; got:\n%s", output)
	}
	if !strings.Contains(output, "Understand-Anything not found on PATH") {
		t.Errorf("output should contain fallback message; got:\n%s", output)
	}
}

func TestRunVisualizeSkipsHiddenAndNodeModules(t *testing.T) {
	dir, graphPath := setupProjectWithGraph(t)
	now := time.Now()

	srcFile := filepath.Join(dir, "main.go")
	os.WriteFile(srcFile, []byte("package main"), 0o644)
	os.Chtimes(srcFile, now.Add(-10*time.Second), now.Add(-10*time.Second))

	os.Chtimes(graphPath, now.Add(-5*time.Second), now.Add(-5*time.Second))

	gitFile := filepath.Join(dir, ".git", "COMMIT_EDITMSG")
	os.MkdirAll(filepath.Dir(gitFile), 0o755)
	os.WriteFile(gitFile, []byte("wip"), 0o644)
	os.Chtimes(gitFile, now, now)

	nmFile := filepath.Join(dir, "node_modules", "react", "index.js")
	os.MkdirAll(filepath.Dir(nmFile), 0o755)
	os.WriteFile(nmFile, []byte("module.exports={}"), 0o644)
	os.Chtimes(nmFile, now, now)

	var out bytes.Buffer
	if err := runVisualize(dir, &out); err != nil {
		t.Fatalf("runVisualize() error = %v", err)
	}
	if strings.Contains(out.String(), "stale") {
		t.Errorf("output should not be stale; .git and node_modules must be skipped; got:\n%s", out.String())
	}
}

func TestIsStale(t *testing.T) {
	dir := t.TempDir()

	graphDir := filepath.Join(dir, ".understand-anything")
	os.MkdirAll(graphDir, 0o755)
	graphPath := filepath.Join(graphDir, "knowledge-graph.json")
	os.WriteFile(graphPath, []byte("{}"), 0o644)

	srcPath := filepath.Join(dir, "src.go")
	os.WriteFile(srcPath, []byte("package main"), 0o644)

	now := time.Now()

	t.Run("fresh", func(t *testing.T) {
		os.Chtimes(srcPath, now.Add(-10*time.Second), now.Add(-10*time.Second))
		os.Chtimes(graphPath, now.Add(-2*time.Second), now.Add(-2*time.Second))

		stale, err := isStale(graphPath, dir)
		if err != nil {
			t.Fatalf("isStale() error: %v", err)
		}
		if stale {
			t.Error("isStale() = true, want false when source is older than graph")
		}
	})

	t.Run("stale", func(t *testing.T) {
		os.Chtimes(graphPath, now.Add(-10*time.Second), now.Add(-10*time.Second))
		os.Chtimes(srcPath, now.Add(-2*time.Second), now.Add(-2*time.Second))

		stale, err := isStale(graphPath, dir)
		if err != nil {
			t.Fatalf("isStale() error: %v", err)
		}
		if !stale {
			t.Error("isStale() = false, want true when source is newer than graph")
		}
	})

	t.Run("within threshold", func(t *testing.T) {
		// source is 1s newer than graph — under the 2s stalenessThreshold, should not report stale
		os.Chtimes(graphPath, now.Add(-1500*time.Millisecond), now.Add(-1500*time.Millisecond))
		os.Chtimes(srcPath, now.Add(-500*time.Millisecond), now.Add(-500*time.Millisecond))

		stale, err := isStale(graphPath, dir)
		if err != nil {
			t.Fatalf("isStale() error: %v", err)
		}
		if stale {
			t.Error("isStale() = true, want false for files within staleness threshold")
		}
	})
}
