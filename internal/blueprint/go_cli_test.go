package blueprint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanukuntla-r/forge/internal/blueprint"
	"github.com/kanukuntla-r/forge/internal/manifest"
	"github.com/kanukuntla-r/forge/internal/render"
)

func TestGoCliManifest(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("go-cli")
	if err != nil {
		t.Fatalf("Find(go-cli): %v", err)
	}
	if bp.Manifest.Name != "go-cli" {
		t.Errorf("Name = %q, want go-cli", bp.Manifest.Name)
	}
	if bp.Manifest.Kind != manifest.KindApp {
		t.Errorf("Kind = %q, want app", bp.Manifest.Kind)
	}
	varNames := map[string]bool{}
	for _, v := range bp.Manifest.Variables {
		varNames[v.Name] = true
	}
	for _, want := range []string{"module_path", "description"} {
		if !varNames[want] {
			t.Errorf("manifest missing variable %q", want)
		}
	}
}

func TestGoCliScaffolds(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("go-cli")
	if err != nil {
		t.Fatalf("Find(go-cli): %v", err)
	}

	dir := filepath.Join(t.TempDir(), "output")
	ctx := map[string]any{
		"Name":        "test-gocli",
		"Description": "A test CLI",
		"ModulePath":  "github.com/testuser/test-gocli",
	}
	written, err := render.WriteBlueprint(bp, ctx, dir)
	if err != nil {
		t.Fatalf("WriteBlueprint: %v", err)
	}

	got := map[string]bool{}
	for _, f := range written {
		got[f] = true
	}

	for _, want := range []string{
		"go.mod",
		"Makefile",
		"README.md",
		".gitignore",
		"main.go",
		"cmd/root.go",
		"cmd/hello.go",
		"internal/logger/logger.go",
	} {
		if !got[want] {
			t.Errorf("expected file %q not written; written: %v", want, written)
		}
	}
}

func TestGoCliModulePathSubstitution(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("go-cli")
	if err != nil {
		t.Fatalf("Find(go-cli): %v", err)
	}

	dir := filepath.Join(t.TempDir(), "output")
	ctx := map[string]any{
		"Name":        "test-gocli",
		"Description": "A test CLI",
		"ModulePath":  "github.com/testuser/test-gocli",
	}
	if _, err := render.WriteBlueprint(bp, ctx, dir); err != nil {
		t.Fatalf("WriteBlueprint: %v", err)
	}

	gomod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}
	if !strings.Contains(string(gomod), "module github.com/testuser/test-gocli") {
		t.Errorf("go.mod missing module path; got:\n%s", gomod)
	}
	if !strings.Contains(string(gomod), "github.com/spf13/cobra") {
		t.Errorf("go.mod missing cobra dependency; got:\n%s", gomod)
	}

	mainGo, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatalf("reading main.go: %v", err)
	}
	if !strings.Contains(string(mainGo), "github.com/testuser/test-gocli/cmd") {
		t.Errorf("main.go missing module import; got:\n%s", mainGo)
	}

	gitignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	if !strings.Contains(string(gitignore), "/test-gocli") {
		t.Errorf(".gitignore missing binary name; got:\n%s", gitignore)
	}
}
