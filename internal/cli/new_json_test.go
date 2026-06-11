package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kanukuntla-r/forge/internal/blueprint"
	"github.com/kanukuntla-r/forge/internal/manifest"
)

// minimalBlueprint builds a blueprint.Blueprint with no template files and no
// graph, suitable for fast JSON-mode tests. The caller can add PostCreate hooks
// to the manifest to exercise hook behaviour.
func minimalBlueprint(name string, hooks []manifest.Hook) *blueprint.Blueprint {
	return &blueprint.Blueprint{
		Manifest: &manifest.Manifest{
			Name:       name,
			Version:    "0.1.0",
			PostCreate: hooks,
		},
		FS: fstest.MapFS{}, // no template/ dir, no graph.yaml
	}
}

func TestRunNewJSONSuccess(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	var buf bytes.Buffer
	bp := minimalBlueprint("test-bp", nil)
	if err := runNewFromBlueprint(context.Background(), &buf, bp, "my-project", nil, runNewOptions{
		useJSON: true,
		noHooks: true,
	}); err != nil {
		t.Fatalf("runNewFromBlueprint() error: %v", err)
	}

	var result newResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	if result.Blueprint != "test-bp" {
		t.Errorf("blueprint = %q, want %q", result.Blueprint, "test-bp")
	}
	if !filepath.IsAbs(result.Path) {
		t.Errorf("path should be absolute, got %q", result.Path)
	}
	if !strings.HasSuffix(result.Path, "my-project") {
		t.Errorf("path should end with project name, got %q", result.Path)
	}
	if result.FilesCreated == nil {
		t.Error("files_created should be a non-null array")
	}
	if result.MarkerPath != ".forge/project.json" {
		t.Errorf("marker_path = %q, want %q", result.MarkerPath, ".forge/project.json")
	}
	if result.Variables == nil {
		t.Error("variables should be present")
	}
	if result.Error != "" {
		t.Errorf("error should be empty on success, got %q", result.Error)
	}
	if result.Phase != "" {
		t.Errorf("phase should be empty on success, got %q", result.Phase)
	}
}

func TestRunNewJSONFailureInHooks(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	failingHooks := []manifest.Hook{
		// Use a command that emits output before failing so we can assert
		// the captured output ends up in hooks_run[i].output.
		{Name: "will-fail", Shell: "echo 'hook output' && exit 1", Optional: false},
	}
	bp := minimalBlueprint("test-bp", failingHooks)

	var buf bytes.Buffer
	err = runNewFromBlueprint(context.Background(), &buf, bp, "hook-project", nil, runNewOptions{
		useJSON: true,
	})
	if err == nil {
		t.Fatal("runNewFromBlueprint() should have returned an error for failing hook")
	}

	// The JSON document must still be on stdout.
	// buf may contain only the JSON (no human-readable text).
	var result newResult
	if parseErr := json.Unmarshal(buf.Bytes(), &result); parseErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", parseErr, buf.String())
	}

	if result.Error == "" {
		t.Error("error field should be non-empty on hook failure")
	}
	if result.Phase != "hooks" {
		t.Errorf("phase = %q, want %q", result.Phase, "hooks")
	}
	if len(result.HooksRun) == 0 {
		t.Fatal("hooks_run should be populated")
	}
	hr := result.HooksRun[0]
	if hr.Name != "will-fail" {
		t.Errorf("hooks_run[0].name = %q, want %q", hr.Name, "will-fail")
	}
	if hr.Success {
		t.Error("hooks_run[0].success should be false")
	}
	if hr.Output == "" {
		t.Error("hooks_run[0].output should be non-empty when capture=true and hook fails")
	}
}

func TestRunNewJSONNoStreamingOutput(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	bp := minimalBlueprint("test-bp", nil)
	var buf bytes.Buffer
	if err := runNewFromBlueprint(context.Background(), &buf, bp, "silent-project", nil, runNewOptions{
		useJSON: true,
		noHooks: true,
	}); err != nil {
		t.Fatalf("runNewFromBlueprint() error: %v", err)
	}

	out := buf.String()

	// None of the four human-readable lines should appear.
	for _, banned := range []string{
		"Created test-bp project",
		"files written",
		"Knowledge graph written",
		"Project marker written",
	} {
		if strings.Contains(out, banned) {
			t.Errorf("stdout contains human-readable line %q in JSON mode; output:\n%s", banned, out)
		}
	}

	// The entire output should parse as a single JSON document.
	var result newResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not a single valid JSON document: %v\noutput: %s", err, out)
	}
}
