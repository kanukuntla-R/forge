package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

func TestRunAnalyzeWritesJSON(t *testing.T) {
	dir := t.TempDir()
	writeAnalyzeFile(t, dir, "main.go", "package main")
	writeAnalyzeFile(t, dir, "util.go", "package main")

	var out, errOut bytes.Buffer
	if err := runAnalyze(&out, &errOut, dir, false); err != nil {
		t.Fatalf("runAnalyze: %v", err)
	}

	outPath := filepath.Join(dir, ".forge", "analysis.json")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading analysis.json: %v", err)
	}
	var analysis analyzer.ProjectAnalysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		t.Fatalf("invalid JSON in analysis.json: %v", err)
	}
	if got := len(analysis.Files); got != 2 {
		t.Errorf("want 2 files, got %d", got)
	}
}

func TestRunAnalyzeJSONFlag(t *testing.T) {
	dir := t.TempDir()
	writeAnalyzeFile(t, dir, "app.ts", "const x = 1;")

	var out, errOut bytes.Buffer
	if err := runAnalyze(&out, &errOut, dir, true); err != nil {
		t.Fatalf("runAnalyze: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("want JSON on stdout, got nothing")
	}
	var analysis analyzer.ProjectAnalysis
	if err := json.Unmarshal(out.Bytes(), &analysis); err != nil {
		t.Fatalf("stdout not valid JSON: %v", err)
	}
	if len(analysis.Files) != 1 {
		t.Errorf("want 1 file in JSON output, got %d", len(analysis.Files))
	}

	// In --json mode, nothing should be written to disk.
	if _, err := os.Stat(filepath.Join(dir, ".forge", "analysis.json")); !os.IsNotExist(err) {
		t.Error("want no analysis.json on disk in --json mode")
	}
}

func writeAnalyzeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
