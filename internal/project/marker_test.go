package project_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanukuntla-r/forge/internal/project"
)

func sampleProject() project.Project {
	return project.Project{
		Blueprint:        "hackathon-app",
		BlueprintVersion: "0.1.0",
		ForgeVersion:     "0.4.0",
		CreatedAt:        "2026-06-11T00:00:00Z",
		Variables: map[string]any{
			"name":    "demo",
			"with_ai": true,
		},
		ExtensionsApplied: []project.ExtensionApplication{},
	}
}

func TestWriteCreatesDirectory(t *testing.T) {
	targetPath := t.TempDir()

	if err := project.Write(targetPath, sampleProject()); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	dir := filepath.Join(targetPath, ".forge")
	checkMode(t, dir, 0o755)

	file := filepath.Join(dir, "project.json")
	checkMode(t, file, 0o644)
}

func TestWriteValidJSON(t *testing.T) {
	want := sampleProject()
	targetPath := t.TempDir()

	if err := project.Write(targetPath, want); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(targetPath, ".forge", "project.json"))
	if err != nil {
		t.Fatalf("reading project.json: %v", err)
	}

	var got project.Project
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Blueprint != want.Blueprint {
		t.Errorf("Blueprint = %q, want %q", got.Blueprint, want.Blueprint)
	}
	if got.BlueprintVersion != want.BlueprintVersion {
		t.Errorf("BlueprintVersion = %q, want %q", got.BlueprintVersion, want.BlueprintVersion)
	}
	if got.ForgeVersion != want.ForgeVersion {
		t.Errorf("ForgeVersion = %q, want %q", got.ForgeVersion, want.ForgeVersion)
	}
	if got.CreatedAt != want.CreatedAt {
		t.Errorf("CreatedAt = %q, want %q", got.CreatedAt, want.CreatedAt)
	}
	if got.ExtensionsApplied == nil {
		t.Error("ExtensionsApplied is nil after round-trip; want empty slice, not null")
	}
	if len(got.ExtensionsApplied) != 0 {
		t.Errorf("len(ExtensionsApplied) = %d, want 0", len(got.ExtensionsApplied))
	}
}

func TestWriteJSONShape(t *testing.T) {
	targetPath := t.TempDir()

	if err := project.Write(targetPath, sampleProject()); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(targetPath, ".forge", "project.json"))
	if err != nil {
		t.Fatalf("reading project.json: %v", err)
	}

	// Keys must be snake_case (JSON tags), not Go field names.
	if !bytes.Contains(data, []byte(`"blueprint_version"`)) {
		t.Error(`JSON missing "blueprint_version" key; struct tag may be wrong`)
	}
	if bytes.Contains(data, []byte(`"BlueprintVersion"`)) {
		t.Error(`JSON contains Go field name "BlueprintVersion"; snake_case tags not applied`)
	}
	if !bytes.Contains(data, []byte(`"extensions_applied"`)) {
		t.Error(`JSON missing "extensions_applied" key`)
	}

	// Indentation must be two spaces.
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "\t") {
			t.Errorf("found tab indentation in JSON output; want two-space indent. Line: %q", line)
			break
		}
	}
	// Verify at least one two-space indent exists.
	hasIndent := false
	for _, line := range lines {
		if strings.HasPrefix(line, "  ") {
			hasIndent = true
			break
		}
	}
	if !hasIndent {
		t.Error("no two-space indentation found in JSON; want indented output")
	}

	// extensions_applied must be [] not null.
	if !bytes.Contains(data, []byte(`"extensions_applied": []`)) {
		t.Errorf("extensions_applied is not [] in output; raw JSON:\n%s", data)
	}
}

func checkMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s mode = %04o, want %04o", path, got, want)
	}
}
