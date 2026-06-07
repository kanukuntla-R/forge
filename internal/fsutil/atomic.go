// Package fsutil provides filesystem utilities for safe, atomic project writes.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Stager accumulates files in a private temporary directory and commits them
// to the target path in a single atomic rename. Typical usage:
//
//	s, err := fsutil.NewStager("./my-app")
//	if err != nil { return err }
//	defer s.Discard() // safe even after Commit; cleans up on any early return
//	// ... WriteFile / Mkdir calls ...
//	return s.Commit()
type Stager struct {
	tmpDir     string
	targetPath string
}

// NewStager creates a staging directory (forge- prefix in the OS temp dir)
// for targetPath. targetPath is stored but not touched until Commit.
func NewStager(targetPath string) (*Stager, error) {
	abs, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, fmt.Errorf("resolving target path: %w", err)
	}
	tmp, err := os.MkdirTemp("", "forge-")
	if err != nil {
		return nil, fmt.Errorf("creating staging directory: %w", err)
	}
	return &Stager{tmpDir: tmp, targetPath: abs}, nil
}

// StagingPath returns the temporary staging directory path.
func (s *Stager) StagingPath() string {
	return s.tmpDir
}

// WriteFile writes content to <staging>/<relPath>, creating any missing
// parent directories. relPath must be relative and must not escape the
// staging root via "..".
func (s *Stager) WriteFile(relPath string, content []byte, mode os.FileMode) error {
	dest, err := s.safeJoin(relPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("creating parent directories for %q: %w", relPath, err)
	}
	return os.WriteFile(dest, content, mode)
}

// Mkdir creates a directory at <staging>/<relPath>. relPath must be relative
// and must not escape the staging root via "..".
func (s *Stager) Mkdir(relPath string, mode os.FileMode) error {
	dest, err := s.safeJoin(relPath)
	if err != nil {
		return err
	}
	return os.MkdirAll(dest, mode)
}

// Commit moves the staging directory to targetPath atomically via os.Rename.
// Returns an error if targetPath already exists — forge never overwrites without
// explicit consent (v0.1 safety invariant).
func (s *Stager) Commit() error {
	if _, err := os.Stat(s.targetPath); err == nil {
		return fmt.Errorf("target %q already exists; refusing to overwrite", s.targetPath)
	}
	if err := os.Rename(s.tmpDir, s.targetPath); err != nil {
		return fmt.Errorf("committing staged files to %q: %w", s.targetPath, err)
	}
	return nil
}

// Discard removes the staging directory. Safe to call after Commit (no-op if
// the staging dir is already gone) and safe to call multiple times.
func (s *Stager) Discard() error {
	// os.RemoveAll returns nil when the path does not exist, so this is
	// naturally idempotent.
	if err := os.RemoveAll(s.tmpDir); err != nil {
		return fmt.Errorf("discarding staging directory: %w", err)
	}
	return nil
}

// safeJoin resolves relPath inside the staging directory, rejecting any path
// that is absolute or that would escape the staging root.
func (s *Stager) safeJoin(relPath string) (string, error) {
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("path must be relative, got %q", relPath)
	}
	cleaned := filepath.Clean(relPath)
	if strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("path %q escapes the target directory", relPath)
	}
	return filepath.Join(s.tmpDir, cleaned), nil
}
