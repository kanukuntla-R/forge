package analyzer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

func TestWalkSimpleDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.ts", "const x = 1;")
	writeFile(t, dir, "config.json", `{"key":"value"}`)
	writeFile(t, dir, "README.md", "# README")

	reg := analyzer.NewRegistry()
	result, err := analyzer.Walk(dir, reg)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
	if got := len(result.Analysis.Files); got != 3 {
		t.Errorf("want 3 files, got %d: %v", got, filePaths(result.Analysis.Files))
	}
	for _, f := range result.Analysis.Files {
		if f.Language != "basic" {
			t.Errorf("file %s: want language=basic, got %s", f.Path, f.Language)
		}
		if f.SizeBytes == 0 {
			t.Errorf("file %s: want non-zero size", f.Path)
		}
	}
}

func TestWalkSkipsIgnoredDirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".git/some-file", "blob")
	writeFile(t, dir, "node_modules/lib.js", "module.exports={}")
	writeFile(t, dir, ".next/build.js", "bundle")
	writeFile(t, dir, "app.ts", "const x = 1;")

	reg := analyzer.NewRegistry()
	result, err := analyzer.Walk(dir, reg)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if got := len(result.Analysis.Files); got != 1 {
		t.Errorf("want 1 file, got %d: %v", got, filePaths(result.Analysis.Files))
	}
	if result.Analysis.Files[0].Path != "app.ts" {
		t.Errorf("want app.ts, got %s", result.Analysis.Files[0].Path)
	}
}

func TestWalkSkipsIgnoredFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package-lock.json", "{}")
	writeFile(t, dir, "image.png", "\x89PNG")
	writeFile(t, dir, ".DS_Store", "blob")
	writeFile(t, dir, "index.ts", "export default {};")

	reg := analyzer.NewRegistry()
	result, err := analyzer.Walk(dir, reg)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if got := len(result.Analysis.Files); got != 1 {
		t.Errorf("want 1 file, got %d: %v", got, filePaths(result.Analysis.Files))
	}
	if result.Analysis.Files[0].Path != "index.ts" {
		t.Errorf("want index.ts, got %s", result.Analysis.Files[0].Path)
	}
}

func TestWalkContinuesOnReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; chmod 000 has no effect")
	}
	dir := t.TempDir()
	writeFile(t, dir, "good.ts", "const x = 1;")

	secret := filepath.Join(dir, "secret.ts")
	if err := os.WriteFile(secret, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(secret, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(secret, 0644) }) //nolint:errcheck

	reg := analyzer.NewRegistry()
	result, err := analyzer.Walk(dir, reg)
	if err != nil {
		t.Fatalf("Walk returned fatal error: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Error("want at least one error for the unreadable file")
	}
	if got := len(result.Analysis.Files); got != 1 {
		t.Errorf("want 1 readable file, got %d", got)
	}
	if result.Analysis.Files[0].Path != "good.ts" {
		t.Errorf("want good.ts in results, got %s", result.Analysis.Files[0].Path)
	}
}

func TestRegistryDispatch(t *testing.T) {
	reg := analyzer.NewRegistry()
	reg.Register(&mockAdapter{ext: ".ts"})

	// .ts file → mock adapter
	a := reg.Dispatch("src/app.ts", nil)
	if a.Name() != "mock" {
		t.Errorf("want mock adapter for .ts file, got %q", a.Name())
	}

	// .txt file → basic fallback
	a = reg.Dispatch("notes.txt", nil)
	if a.Name() != "basic" {
		t.Errorf("want basic adapter for .txt file, got %q", a.Name())
	}
}

func TestProjectInfoDerivedFromRootPath(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "my-test-project")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "main.go", "package main")

	reg := analyzer.NewRegistry()
	result, err := analyzer.Walk(dir, reg)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if got := result.Analysis.Project.Name; got != "my-test-project" {
		t.Errorf("want name=my-test-project, got %q", got)
	}
}

// helpers

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func filePaths(files []analyzer.FileInfo) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}

// mockAdapter detects files by extension.
type mockAdapter struct {
	ext string
}

func (m *mockAdapter) Name() string             { return "mock" }
func (m *mockAdapter) FileExtensions() []string { return []string{m.ext} }
func (m *mockAdapter) Detect(path string, _ []byte) bool {
	return filepath.Ext(path) == m.ext
}
func (m *mockAdapter) Analyze(_ string, _ []byte) (*analyzer.FileAnalysis, error) {
	return &analyzer.FileAnalysis{}, nil
}
