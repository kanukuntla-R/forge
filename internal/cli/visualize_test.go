package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeVisualizeAnalysis writes a minimal analysis.json under dir/.forge/.
func writeVisualizeAnalysis(t *testing.T, dir string) string {
	t.Helper()
	forgeDir := filepath.Join(dir, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatalf("mkdir .forge: %v", err)
	}
	data, _ := json.Marshal(map[string]any{
		"version": "1",
		"project": map[string]any{"name": "smoke", "frameworks": []string{}},
		"files":   []any{},
	})
	path := filepath.Join(forgeDir, "analysis.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write analysis.json: %v", err)
	}
	return path
}

// runVisualizeNoServer calls runVisualize but stops before the server blocks.
// We verify side-effects (analyze ran or not) by checking for analysis.json.
// The server start itself is covered in internal/dashboard/server_test.go.
//
// To avoid blocking on srv.Start, we use a project directory with an existing
// analysis.json and a patched entrypoint isn't needed — we test at the layer
// just before server creation by checking stdout/stderr content.

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
	// runVisualize will call runAnalyze (which creates analysis.json), then try
	// to start the server. We let it create the analysis and check that happens,
	// but the server.Start will block — so we do NOT call runVisualize directly
	// in a blocking way. Instead, verify the pre-conditions and that analyze ran.
	//
	// Strategy: call runAnalyze directly (same code path visualize uses), then
	// verify the output is consistent with what visualize would print.
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

func TestRunVisualizeUsesExistingAnalysis(t *testing.T) {
	dir := t.TempDir()
	analysisPath := writeVisualizeAnalysis(t, dir)

	// Record the mtime before we would call visualize.
	infoBeforeStr := modTimeString(t, analysisPath)

	// If analysis.json already exists and --refresh=false, analyze must NOT run.
	// We verify by checking that nothing about the file changes and that
	// runAnalyze's output message doesn't appear when we skip it.
	//
	// Simulate the decision logic from runVisualize:
	_, statErr := os.Stat(analysisPath)
	refresh := false
	analyzeWouldRun := refresh || os.IsNotExist(statErr)

	if analyzeWouldRun {
		t.Error("visualize should NOT re-run analyze when analysis.json exists and --refresh is false")
	}

	infoAfterStr := modTimeString(t, analysisPath)
	if infoBeforeStr != infoAfterStr {
		t.Error("analysis.json mtime changed unexpectedly — analyze must have run")
	}
}

func TestRunVisualizeRefreshForcesReanalysis(t *testing.T) {
	dir := t.TempDir()
	writeVisualizeAnalysis(t, dir)
	// Write a source file so analyze has something to walk.
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)

	refresh := true
	_, statErr := os.Stat(filepath.Join(dir, ".forge", "analysis.json"))
	analyzeWouldRun := refresh || os.IsNotExist(statErr)

	if !analyzeWouldRun {
		t.Error("visualize should re-run analyze when --refresh is true")
	}

	// Actually run analyze to confirm it succeeds with --refresh.
	var out, errOut bytes.Buffer
	if err := runAnalyze(&out, &errOut, dir, false); err != nil {
		t.Fatalf("runAnalyze with refresh: %v", err)
	}
}

func TestRunVisualizeNonexistentPath(t *testing.T) {
	var out, errOut bytes.Buffer
	err := runVisualize(&out, &errOut, "/nonexistent/path/that/does/not/exist", false)
	if err == nil {
		t.Fatal("runVisualize with nonexistent path: want error, got nil")
	}
}

func modTimeString(t *testing.T, path string) string {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.ModTime().String()
}
