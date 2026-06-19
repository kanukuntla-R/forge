package analyzer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

// makeAnalysis builds a ProjectAnalysis with the given files and their imports.
// Each entry in files is a path; imports maps file-path → []Import.
func makeAnalysis(files []string, imports map[string][]analyzer.Import) *analyzer.ProjectAnalysis {
	a := &analyzer.ProjectAnalysis{}
	for _, f := range files {
		fi := analyzer.FileInfo{
			Path:     f,
			Language: "typescript",
		}
		if imps, ok := imports[f]; ok {
			fi.Imports = imps
		}
		a.Files = append(a.Files, fi)
	}
	return a
}

func resolve(t *testing.T, dir string, analysis *analyzer.ProjectAnalysis) {
	t.Helper()
	r, err := analyzer.NewResolver(dir, analysis)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	r.Resolve(analysis)
}

func imp(source string, external bool) analyzer.Import {
	return analyzer.Import{Source: source, Names: []string{"x"}, External: external}
}

func writeTsconfig(t *testing.T, dir string, paths map[string][]string) {
	t.Helper()
	type compilerOptions struct {
		BaseUrl string              `json:"baseUrl,omitempty"`
		Paths   map[string][]string `json:"paths"`
	}
	type cfg struct {
		CompilerOptions compilerOptions `json:"compilerOptions"`
	}
	data, _ := json.Marshal(cfg{CompilerOptions: compilerOptions{Paths: paths}})
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveRelativeImport(t *testing.T) {
	dir := t.TempDir()
	analysis := makeAnalysis(
		[]string{"app/page.tsx", "app/utils.ts"},
		map[string][]analyzer.Import{
			"app/page.tsx": {imp("./utils", false)},
		},
	)
	resolve(t, dir, analysis)
	if got := analysis.Files[0].Imports[0].Resolved; got != "app/utils.ts" {
		t.Errorf("want app/utils.ts, got %q", got)
	}
}

func TestResolveRelativeImportWithExtension(t *testing.T) {
	dir := t.TempDir()
	analysis := makeAnalysis(
		[]string{"app/page.tsx", "app/utils.ts"},
		map[string][]analyzer.Import{
			"app/page.tsx": {imp("./utils.ts", false)},
		},
	)
	resolve(t, dir, analysis)
	if got := analysis.Files[0].Imports[0].Resolved; got != "app/utils.ts" {
		t.Errorf("want app/utils.ts, got %q", got)
	}
}

func TestResolveRelativeImportToTsx(t *testing.T) {
	dir := t.TempDir()
	analysis := makeAnalysis(
		[]string{"app/page.tsx", "app/component.tsx"},
		map[string][]analyzer.Import{
			"app/page.tsx": {imp("./component", false)},
		},
	)
	resolve(t, dir, analysis)
	if got := analysis.Files[0].Imports[0].Resolved; got != "app/component.tsx" {
		t.Errorf("want app/component.tsx, got %q", got)
	}
}

func TestResolveRelativeImportToIndex(t *testing.T) {
	dir := t.TempDir()
	analysis := makeAnalysis(
		[]string{"app/page.tsx", "app/lib/index.ts"},
		map[string][]analyzer.Import{
			"app/page.tsx": {imp("./lib", false)},
		},
	)
	resolve(t, dir, analysis)
	if got := analysis.Files[0].Imports[0].Resolved; got != "app/lib/index.ts" {
		t.Errorf("want app/lib/index.ts, got %q", got)
	}
}

func TestResolveParentImport(t *testing.T) {
	dir := t.TempDir()
	analysis := makeAnalysis(
		[]string{"app/page.tsx", "lib/utils.ts"},
		map[string][]analyzer.Import{
			"app/page.tsx": {imp("../lib/utils", false)},
		},
	)
	resolve(t, dir, analysis)
	if got := analysis.Files[0].Imports[0].Resolved; got != "lib/utils.ts" {
		t.Errorf("want lib/utils.ts, got %q", got)
	}
}

func TestResolveAliasImport(t *testing.T) {
	dir := t.TempDir()
	writeTsconfig(t, dir, map[string][]string{"@/*": {"./*"}})
	analysis := makeAnalysis(
		[]string{"app/page.tsx", "components/Foo.tsx"},
		map[string][]analyzer.Import{
			"app/page.tsx": {imp("@/components/Foo", false)},
		},
	)
	resolve(t, dir, analysis)
	if got := analysis.Files[0].Imports[0].Resolved; got != "components/Foo.tsx" {
		t.Errorf("want components/Foo.tsx, got %q", got)
	}
}

func TestResolveAliasImportWithoutTsconfig(t *testing.T) {
	// No tsconfig.json — fallback alias "@/*" → "./*" should still apply.
	dir := t.TempDir()
	analysis := makeAnalysis(
		[]string{"app/page.tsx", "components/Foo.tsx"},
		map[string][]analyzer.Import{
			"app/page.tsx": {imp("@/components/Foo", false)},
		},
	)
	resolve(t, dir, analysis)
	if got := analysis.Files[0].Imports[0].Resolved; got != "components/Foo.tsx" {
		t.Errorf("want components/Foo.tsx, got %q", got)
	}
}

func TestResolveExternalImportSkipped(t *testing.T) {
	dir := t.TempDir()
	analysis := makeAnalysis(
		[]string{"app/page.tsx"},
		map[string][]analyzer.Import{
			"app/page.tsx": {imp("react", true)},
		},
	)
	resolve(t, dir, analysis)
	if got := analysis.Files[0].Imports[0].Resolved; got != "" {
		t.Errorf("want empty resolved for external import, got %q", got)
	}
}

func TestResolveUnresolvableImport(t *testing.T) {
	dir := t.TempDir()
	analysis := makeAnalysis(
		[]string{"app/page.tsx"},
		map[string][]analyzer.Import{
			"app/page.tsx": {imp("./nonexistent", false)},
		},
	)
	resolve(t, dir, analysis)
	if got := analysis.Files[0].Imports[0].Resolved; got != "" {
		t.Errorf("want empty resolved for unresolvable import, got %q", got)
	}
}

func TestResolveTsconfigInvalid(t *testing.T) {
	dir := t.TempDir()
	// Write a tsconfig with comments — standard JSON parser rejects it.
	invalid := []byte(`{ // this is a comment
  "compilerOptions": {
    "paths": { "@/*": ["./*"] }
  }
}`)
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), invalid, 0644); err != nil {
		t.Fatal(err)
	}
	analysis := makeAnalysis(
		[]string{"app/page.tsx", "lib/utils.ts"},
		map[string][]analyzer.Import{
			"app/page.tsx": {imp("@/lib/utils", false)},
		},
	)
	// Should not crash; fallback aliases handle @/ even if tsconfig is invalid.
	resolve(t, dir, analysis)
	if got := analysis.Files[0].Imports[0].Resolved; got != "lib/utils.ts" {
		t.Errorf("want lib/utils.ts via fallback alias, got %q", got)
	}
}

func TestResolveCustomAlias(t *testing.T) {
	dir := t.TempDir()
	writeTsconfig(t, dir, map[string][]string{
		"@features/*": {"./src/features/*"},
	})
	analysis := makeAnalysis(
		[]string{"app/page.tsx", "src/features/auth.ts"},
		map[string][]analyzer.Import{
			"app/page.tsx": {imp("@features/auth", false)},
		},
	)
	resolve(t, dir, analysis)
	if got := analysis.Files[0].Imports[0].Resolved; got != "src/features/auth.ts" {
		t.Errorf("want src/features/auth.ts, got %q", got)
	}
}

func TestResolveMultipleAliases(t *testing.T) {
	dir := t.TempDir()
	writeTsconfig(t, dir, map[string][]string{
		"@/*":       {"./*"},
		"@shared/*": {"./shared/*"},
	})
	analysis := makeAnalysis(
		[]string{"app/page.tsx", "lib/utils.ts", "shared/types.ts"},
		map[string][]analyzer.Import{
			"app/page.tsx": {
				imp("@/lib/utils", false),
				imp("@shared/types", false),
			},
		},
	)
	resolve(t, dir, analysis)
	if got := analysis.Files[0].Imports[0].Resolved; got != "lib/utils.ts" {
		t.Errorf("import 0: want lib/utils.ts, got %q", got)
	}
	if got := analysis.Files[0].Imports[1].Resolved; got != "shared/types.ts" {
		t.Errorf("import 1: want shared/types.ts, got %q", got)
	}
}
