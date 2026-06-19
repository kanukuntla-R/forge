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
		if f.Language == "" {
			t.Errorf("file %s: want non-empty language, got empty string", f.Path)
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
	// Use an extension that no built-in adapter handles, so the mock can win.
	reg.Register(&mockAdapter{ext: ".forge_test"})

	// .forge_test file → mock adapter (typescript adapter passes on it)
	a := reg.Dispatch("src/test.forge_test", nil)
	if a.Name() != "mock" {
		t.Errorf("want mock adapter for .forge_test file, got %q", a.Name())
	}

	// .txt file → basic fallback (neither typescript nor mock handles it)
	a = reg.Dispatch("notes.txt", nil)
	if a.Name() != "basic" {
		t.Errorf("want basic adapter for .txt file, got %q", a.Name())
	}
}

func TestWalkUsesTypescriptAdapter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.ts", "export function foo() {}")

	reg := analyzer.NewRegistry()
	result, err := analyzer.Walk(dir, reg)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if got := len(result.Analysis.Files); got != 1 {
		t.Fatalf("want 1 file, got %d", got)
	}
	if got := result.Analysis.Files[0].Language; got != "typescript" {
		t.Errorf("app.ts: want language=typescript, got %q", got)
	}
}

func TestWalkUsesTypescriptForTsx(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.tsx", `export default function App() { return <div>hi</div>; }`)

	reg := analyzer.NewRegistry()
	result, err := analyzer.Walk(dir, reg)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if got := len(result.Analysis.Files); got != 1 {
		t.Fatalf("want 1 file, got %d", got)
	}
	if got := result.Analysis.Files[0].Language; got != "typescript" {
		t.Errorf("app.tsx: want language=typescript, got %q", got)
	}
}

func TestWalkUsesTypescriptForJsAndJsx(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "util.js", "module.exports = {}")
	writeFile(t, dir, "widget.jsx", "export default () => <span/>;")

	reg := analyzer.NewRegistry()
	result, err := analyzer.Walk(dir, reg)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if got := len(result.Analysis.Files); got != 2 {
		t.Fatalf("want 2 files, got %d", got)
	}
	for _, f := range result.Analysis.Files {
		if f.Language != "typescript" {
			t.Errorf("%s: want language=typescript, got %q", f.Path, f.Language)
		}
	}
}

func TestWalkUsesBasicForOtherFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# README")

	reg := analyzer.NewRegistry()
	result, err := analyzer.Walk(dir, reg)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if got := len(result.Analysis.Files); got != 1 {
		t.Fatalf("want 1 file, got %d", got)
	}
	if got := result.Analysis.Files[0].Language; got != "basic" {
		t.Errorf("README.md: want language=basic, got %q", got)
	}
}

func TestWalkExtractsTypescriptImports(t *testing.T) {
	dir := t.TempDir()
	content := `import { foo } from "./utils"
import React from "react"
export function bar() {}`
	writeFile(t, dir, "app.ts", content)

	reg := analyzer.NewRegistry()
	result, err := analyzer.Walk(dir, reg)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if got := len(result.Analysis.Files); got != 1 {
		t.Fatalf("want 1 file, got %d", got)
	}
	f := result.Analysis.Files[0]
	if f.Language != "typescript" {
		t.Fatalf("want language=typescript, got %q", f.Language)
	}
	if got := len(f.Imports); got != 2 {
		t.Fatalf("want 2 imports, got %d", got)
	}
	// First import: named, local
	if f.Imports[0].Source != "./utils" {
		t.Errorf("import 0 source: want %q, got %q", "./utils", f.Imports[0].Source)
	}
	if f.Imports[0].External {
		t.Errorf("import 0: want external=false")
	}
	// Second import: default, external
	if f.Imports[1].Source != "react" {
		t.Errorf("import 1 source: want %q, got %q", "react", f.Imports[1].Source)
	}
	if !f.Imports[1].External {
		t.Errorf("import 1: want external=true")
	}
	if len(f.Imports[1].Names) != 1 || f.Imports[1].Names[0] != "default" {
		t.Errorf("import 1 names: want [default], got %v", f.Imports[1].Names)
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
