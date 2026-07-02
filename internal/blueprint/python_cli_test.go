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

func TestPythonCliManifest(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("python-cli")
	if err != nil {
		t.Fatalf("Find(python-cli): %v", err)
	}
	if bp.Manifest.Name != "python-cli" {
		t.Errorf("Name = %q, want python-cli", bp.Manifest.Name)
	}
	if bp.Manifest.Kind != manifest.KindApp {
		t.Errorf("Kind = %q, want app", bp.Manifest.Kind)
	}
	varNames := map[string]bool{}
	for _, v := range bp.Manifest.Variables {
		varNames[v.Name] = true
	}
	if !varNames["description"] {
		t.Error("manifest missing variable 'description'")
	}
}

func TestPythonCliScaffolds(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("python-cli")
	if err != nil {
		t.Fatalf("Find(python-cli): %v", err)
	}

	dir := filepath.Join(t.TempDir(), "output")
	ctx := map[string]any{
		"Name":        "test-cli",
		"Description": "A test CLI",
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
		"pyproject.toml",
		"README.md",
		".python-version",
		".gitignore",
		"src/__init__.py",
		"src/main.py",
		"src/commands/__init__.py",
		"src/commands/hello.py",
		"tests/__init__.py",
		"tests/test_hello.py",
	} {
		if !got[want] {
			t.Errorf("expected file %q not written; written: %v", want, written)
		}
	}
}

func TestPythonCliManifestHasEntryPoint(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("python-cli")
	if err != nil {
		t.Fatalf("Find(python-cli): %v", err)
	}

	dir := filepath.Join(t.TempDir(), "output")
	ctx := map[string]any{
		"Name":        "test-cli",
		"Description": "A test CLI",
	}
	if _, err := render.WriteBlueprint(bp, ctx, dir); err != nil {
		t.Fatalf("WriteBlueprint: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if err != nil {
		t.Fatalf("reading pyproject.toml: %v", err)
	}
	toml := string(content)

	if !strings.Contains(toml, "[project.scripts]") {
		t.Error("pyproject.toml missing [project.scripts] section")
	}
	// hyphen in name must be replaced with underscore in the entry point key
	if !strings.Contains(toml, "test_cli = ") {
		t.Errorf("pyproject.toml missing entry point 'test_cli'; got:\n%s", toml)
	}
	if !strings.Contains(toml, "[build-system]") {
		t.Error("pyproject.toml missing [build-system] section")
	}
	if !strings.Contains(toml, "hatchling") {
		t.Error("pyproject.toml missing hatchling build backend")
	}
}
