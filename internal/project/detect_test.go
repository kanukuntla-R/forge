package project_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanukuntla-r/forge/internal/project"
)

func writeProjectJSON(t *testing.T, dir string, p project.Project) {
	t.Helper()
	forgeDir := filepath.Join(dir, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", forgeDir, err)
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("marshal project: %v", err)
	}
	dest := filepath.Join(forgeDir, "project.json")
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		t.Fatalf("write %q: %v", dest, err)
	}
}

func sampleDetectProject() project.Project {
	return project.Project{
		Blueprint:        "hackathon-app",
		BlueprintVersion: "0.1.0",
		ForgeVersion:     "0.4.0",
		CreatedAt:        "2026-06-12T00:00:00Z",
		Variables:        map[string]any{"name": "demo"},
	}
}

func TestDetectFindsAtStartingDirectory(t *testing.T) {
	dir := t.TempDir()
	want := sampleDetectProject()
	writeProjectJSON(t, dir, want)

	result, err := project.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if result == nil {
		t.Fatal("Detect() = nil, want non-nil")
	}
	if result.Project.Blueprint != want.Blueprint {
		t.Errorf("Blueprint = %q, want %q", result.Project.Blueprint, want.Blueprint)
	}
	if result.Root != dir {
		t.Errorf("Root = %q, want %q", result.Root, dir)
	}
}

func TestDetectFindsInParent(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProjectJSON(t, tmp, sampleDetectProject())

	result, err := project.Detect(sub)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if result == nil {
		t.Fatal("Detect() = nil, want non-nil")
	}
	if result.Root != tmp {
		t.Errorf("Root = %q, want %q", result.Root, tmp)
	}
}

func TestDetectFindsInGrandparent(t *testing.T) {
	tmp := t.TempDir()
	deep := filepath.Join(tmp, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProjectJSON(t, tmp, sampleDetectProject())

	result, err := project.Detect(deep)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if result == nil {
		t.Fatal("Detect() = nil, want non-nil")
	}
	if result.Root != tmp {
		t.Errorf("Root = %q, want %q", result.Root, tmp)
	}
}

func TestDetectNotFound(t *testing.T) {
	dir := t.TempDir()
	// No .forge/ anywhere — but TempDir is inside the system temp dir,
	// which won't have a .forge/. We add a .git so the walk stops here.
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := project.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if result != nil {
		t.Errorf("Detect() = %+v, want nil", result)
	}
}

func TestDetectMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	forgeDir := filepath.Join(dir, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(forgeDir, "project.json"), []byte("{not valid"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := project.Detect(dir)
	if result != nil {
		t.Errorf("Detect() result = %+v, want nil", result)
	}
	if err == nil {
		t.Fatal("Detect() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("error %q does not contain %q", err.Error(), "parsing")
	}
	markerPath := filepath.Join(dir, ".forge", "project.json")
	if !strings.Contains(err.Error(), markerPath) {
		t.Errorf("error %q does not contain path %q", err.Error(), markerPath)
	}
}

func TestDetectStopsAtGitBoundary(t *testing.T) {
	tmp := t.TempDir()
	// .forge/project.json lives at tmp/
	writeProjectJSON(t, tmp, sampleDetectProject())
	// .git lives at tmp/proj/ — the walk starts from tmp/proj/src/
	src := filepath.Join(tmp, "proj", "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmp, "proj", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := project.Detect(src)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if result != nil {
		t.Errorf("Detect() = %+v, want nil (should not cross .git boundary)", result)
	}
}

func TestDetectReturnsCorrectRoot(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "child")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProjectJSON(t, tmp, sampleDetectProject())

	result, err := project.Detect(sub)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if result == nil {
		t.Fatal("Detect() = nil, want non-nil")
	}
	if !filepath.IsAbs(result.Root) {
		t.Errorf("Root = %q is not absolute", result.Root)
	}
	if result.Root != tmp {
		t.Errorf("Root = %q, want %q", result.Root, tmp)
	}
}

func TestDetectStartingPathRelative(t *testing.T) {
	// Write project.json into the current directory's temp sibling,
	// then pass a relative path to Detect.
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProjectJSON(t, tmp, sampleDetectProject())

	// Change working directory so we can form a relative path.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	result, err := project.Detect("./sub")
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if result == nil {
		t.Fatal("Detect() = nil, want non-nil")
	}
	if !filepath.IsAbs(result.Root) {
		t.Errorf("Root = %q is not absolute", result.Root)
	}
}
