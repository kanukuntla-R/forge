package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstall_CreatesDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	result, err := Install()
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if !result.Created {
		t.Error("Created = false, want true (directory didn't exist)")
	}

	content, err := os.ReadFile(result.InstalledPath)
	if err != nil {
		t.Fatalf("reading installed file: %v", err)
	}
	if !strings.Contains(string(content), "forge") {
		t.Error("installed file doesn't contain expected content")
	}
}

func TestInstall_BackupExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	skillsDir := filepath.Join(tempDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	existingPath := filepath.Join(skillsDir, "forge.skill.md")
	if err := os.WriteFile(existingPath, []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Install()
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if result.BackupPath == "" {
		t.Fatal("BackupPath is empty, want a backup path")
	}

	backupContent, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(backupContent) != "old content" {
		t.Errorf("backup content = %q, want %q", backupContent, "old content")
	}

	newContent, err := os.ReadFile(result.InstalledPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(newContent), "forge") {
		t.Error("new file doesn't contain expected content")
	}
}

func TestInstall_ExistingDirectoryNoBackup(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	skillsDir := filepath.Join(tempDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := Install()
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if result.Created {
		t.Error("Created = true, want false (directory already existed)")
	}
	if result.BackupPath != "" {
		t.Errorf("BackupPath = %q, want empty (no existing file)", result.BackupPath)
	}
}
