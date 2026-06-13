package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeriveNameFromURL(t *testing.T) {
	cases := []struct{ url, want string }{
		{"https://github.com/foo/bar", "bar"},
		{"https://github.com/foo/bar.git", "bar"},
		{"git@github.com:foo/bar.git", "bar"},
		{"https://example.com/repo.git/", "repo"},
	}
	for _, tc := range cases {
		got := deriveName(tc.url)
		if got != tc.want {
			t.Errorf("deriveName(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestRunInstallTargetDirExists(t *testing.T) {
	tmp := t.TempDir()
	blueprintsDir := filepath.Join(tmp, "blueprints")
	target := filepath.Join(blueprintsDir, "my-bp")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	var out, stderr strings.Builder
	err := runInstall(context.Background(), &out, &stderr, "https://example.com/my-bp.git", "my-bp", blueprintsDir, false)
	if err == nil {
		t.Fatal("expected error for pre-existing target dir, got nil")
	}
	if !strings.Contains(err.Error(), "already installed") {
		t.Errorf("error %q should mention 'already installed'", err.Error())
	}
}

func TestRunInstallSuccessfulClone(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	srcDir := t.TempDir()
	writeInstallTestManifest(t, srcDir, "name: test-bp\nkind: app\nversion: \"0.1.0\"\n")
	gitInitRepo(t, srcDir)

	blueprintsDir := t.TempDir()
	var out, stderr strings.Builder
	err := runInstall(context.Background(), &out, &stderr, "file://"+srcDir, "test-bp", blueprintsDir, false)
	if err != nil {
		t.Fatalf("runInstall error: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(out.String(), "Installed blueprint") {
		t.Errorf("expected success message, got: %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(blueprintsDir, "test-bp", "manifest.yaml")); err != nil {
		t.Errorf("manifest.yaml missing in target: %v", err)
	}
}

func TestRunInstallInvalidManifest(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	srcDir := t.TempDir()
	writeInstallTestManifest(t, srcDir, "name: bad-bp\nversion: \"0.1.0\"\n") // missing kind
	gitInitRepo(t, srcDir)

	blueprintsDir := t.TempDir()
	var out, stderr strings.Builder
	err := runInstall(context.Background(), &out, &stderr, "file://"+srcDir, "bad-bp", blueprintsDir, false)
	if err == nil {
		t.Fatal("expected error for invalid manifest, got nil")
	}
	target := filepath.Join(blueprintsDir, "bad-bp")
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Errorf("target dir should be cleaned up after failed install, but exists at %s", target)
	}
}

func writeInstallTestManifest(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitInitRepo(t *testing.T, dir string) {
	t.Helper()
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	for _, args := range [][]string{{"init"}, {"add", "-A"}, {"commit", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}
