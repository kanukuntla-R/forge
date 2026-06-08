package fsutil_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanukuntla-r/forge/internal/fsutil"
)

func TestWriteFileCreatesFileInStaging(t *testing.T) {
	stager, err := fsutil.NewStager(filepath.Join(t.TempDir(), "target"))
	if err != nil {
		t.Fatalf("NewStager() error: %v", err)
	}
	defer stager.Discard()

	if err := stager.WriteFile("hello.txt", []byte("world"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	staged := filepath.Join(stager.StagingPath(), "hello.txt")
	if _, err := os.Stat(staged); err != nil {
		t.Errorf("staged file not found at %q: %v", staged, err)
	}
}

func TestCommitMovesFilesToTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "my-app")
	stager, err := fsutil.NewStager(target)
	if err != nil {
		t.Fatalf("NewStager() error: %v", err)
	}
	defer stager.Discard()

	if err := stager.WriteFile("README.md", []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := stager.Commit(); err != nil {
		t.Fatalf("Commit() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(target, "README.md"))
	if err != nil {
		t.Fatalf("reading committed file: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("content = %q, want %q", content, "hello")
	}
}

func TestDiscardRemovesStagingDirectory(t *testing.T) {
	stager, err := fsutil.NewStager(filepath.Join(t.TempDir(), "target"))
	if err != nil {
		t.Fatalf("NewStager() error: %v", err)
	}

	stagingDir := stager.StagingPath()
	if _, err := os.Stat(stagingDir); err != nil {
		t.Fatalf("staging dir should exist before Discard: %v", err)
	}

	if err := stager.Discard(); err != nil {
		t.Fatalf("Discard() error: %v", err)
	}

	if _, err := os.Stat(stagingDir); !os.IsNotExist(err) {
		t.Errorf("staging dir should be gone after Discard; got: %v", err)
	}
}

func TestDiscardIsIdempotent(t *testing.T) {
	stager, err := fsutil.NewStager(filepath.Join(t.TempDir(), "target"))
	if err != nil {
		t.Fatalf("NewStager() error: %v", err)
	}

	if err := stager.Discard(); err != nil {
		t.Fatalf("first Discard() error: %v", err)
	}
	if err := stager.Discard(); err != nil {
		t.Fatalf("second Discard() error: %v", err)
	}
}

func TestDiscardSafeAfterCommit(t *testing.T) {
	target := filepath.Join(t.TempDir(), "output")
	stager, err := fsutil.NewStager(target)
	if err != nil {
		t.Fatalf("NewStager() error: %v", err)
	}

	if err := stager.WriteFile("f.txt", []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := stager.Commit(); err != nil {
		t.Fatalf("Commit() error: %v", err)
	}
	// Simulates the defer stager.Discard() pattern.
	if err := stager.Discard(); err != nil {
		t.Errorf("Discard() after Commit() error: %v", err)
	}
}

func TestPathTraversalDoubleDotRejected(t *testing.T) {
	stager, err := fsutil.NewStager(filepath.Join(t.TempDir(), "target"))
	if err != nil {
		t.Fatalf("NewStager() error: %v", err)
	}
	defer stager.Discard()

	cases := []string{"../escape", "a/../../escape", "a/../../../etc/passwd"}
	for _, p := range cases {
		if err := stager.WriteFile(p, []byte("x"), 0644); err == nil {
			t.Errorf("WriteFile(%q): expected error for traversal path, got nil", p)
		}
	}
}

func TestPathAbsoluteRejected(t *testing.T) {
	stager, err := fsutil.NewStager(filepath.Join(t.TempDir(), "target"))
	if err != nil {
		t.Fatalf("NewStager() error: %v", err)
	}
	defer stager.Discard()

	if err := stager.WriteFile("/etc/passwd", []byte("x"), 0644); err == nil {
		t.Error("WriteFile(/etc/passwd): expected error for absolute path, got nil")
	}
}

// TestStagingPathIsNextToTarget asserts that the staging directory lives inside
// the target's parent, not in /tmp — the same-filesystem requirement for Rename.
func TestStagingPathIsNextToTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "my-app")
	stager, err := fsutil.NewStager(target)
	if err != nil {
		t.Fatalf("NewStager() error: %v", err)
	}
	defer stager.Discard()

	parent := filepath.Dir(target)
	if !strings.HasPrefix(stager.StagingPath(), parent+string(filepath.Separator)) {
		t.Errorf("staging path %q does not start with target parent %q", stager.StagingPath(), parent)
	}
}

// TestNewStagerCreatesDeepParent verifies that NewStager succeeds even when
// intermediate directories in the target path don't exist yet.
func TestNewStagerCreatesDeepParent(t *testing.T) {
	target := filepath.Join(t.TempDir(), "deep", "path", "project")
	stager, err := fsutil.NewStager(target)
	if err != nil {
		t.Fatalf("NewStager() with non-existent deep parent: %v", err)
	}
	defer stager.Discard()

	parent := filepath.Dir(target)
	if _, err := os.Stat(parent); err != nil {
		t.Errorf("parent directory %q not created: %v", parent, err)
	}
}

func TestCommitMovesDeepTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "deep", "path", "project")
	stager, err := fsutil.NewStager(target)
	if err != nil {
		t.Fatalf("NewStager() error: %v", err)
	}
	defer stager.Discard()

	if err := stager.WriteFile("main.go", []byte("package main"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := stager.Commit(); err != nil {
		t.Fatalf("Commit() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(target, "main.go"))
	if err != nil {
		t.Fatalf("reading committed file: %v", err)
	}
	if string(content) != "package main" {
		t.Errorf("content = %q, want %q", content, "package main")
	}
}

func TestCommitFailsWhenTargetAlreadyExists(t *testing.T) {
	target := filepath.Join(t.TempDir(), "existing")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatalf("creating existing target: %v", err)
	}

	stager, err := fsutil.NewStager(target)
	if err != nil {
		t.Fatalf("NewStager() error: %v", err)
	}
	defer stager.Discard()

	if err := stager.Commit(); err == nil {
		t.Error("Commit(): expected error when target already exists, got nil")
	}
}
