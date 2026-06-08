package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunNewHackathonApp(t *testing.T) {
	// runNew resolves target path relative to CWD, so we park in a temp dir.
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}

	var buf bytes.Buffer
	if err := runNew(context.Background(), &buf, "hackathon-app", "my-idea", nil, false); err != nil {
		t.Fatalf("runNew() error: %v", err)
	}

	target := filepath.Join(dir, "my-idea")
	if _, err := os.Stat(target); err != nil {
		t.Errorf("target directory %q not found after runNew: %v", target, err)
	}
	if _, err := os.Stat(filepath.Join(target, "README.md")); err != nil {
		t.Errorf("README.md not found in target: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "hackathon-app") {
		t.Errorf("success message should mention blueprint name; got: %q", out)
	}
}
