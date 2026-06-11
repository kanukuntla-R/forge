package hooks_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kanukuntla-r/forge/internal/hooks"
	"github.com/kanukuntla-r/forge/internal/manifest"
)

func mkHook(name, shell string, optional bool) manifest.Hook {
	return manifest.Hook{Name: name, Shell: shell, Optional: optional}
}

func TestRunNoHooks(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := hooks.Run(context.Background(), nil, t.TempDir(), nil, &stdout, &stderr); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}

func TestRunSingleSuccessfulHook(t *testing.T) {
	var stdout, stderr bytes.Buffer
	hs := []manifest.Hook{mkHook("say-hello", "echo hello", false)}
	if err := hooks.Run(context.Background(), hs, t.TempDir(), nil, &stdout, &stderr); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	out := stdout.String()
	for _, want := range []string{"Running hook", "✓", "hello"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q; got:\n%s", want, out)
		}
	}
}

func TestRunTemplateRender(t *testing.T) {
	var stdout, stderr bytes.Buffer
	hs := []manifest.Hook{mkHook("greet", "echo {{ .Greeting }}", false)}
	tmplCtx := map[string]any{"Greeting": "world"}
	if err := hooks.Run(context.Background(), hs, t.TempDir(), tmplCtx, &stdout, &stderr); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "world") {
		t.Errorf("stdout missing rendered value %q; got:\n%s", "world", stdout.String())
	}
}

func TestRunOptionalFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	hs := []manifest.Hook{
		mkHook("will-fail", "false", true),
		mkHook("sentinel", "echo sentinel-ok", false),
	}
	if err := hooks.Run(context.Background(), hs, t.TempDir(), nil, &stdout, &stderr); err != nil {
		t.Fatalf("Run() error = %v, want nil (optional failure should not propagate)", err)
	}
	if !strings.Contains(stderr.String(), "optional hook failed") {
		t.Errorf("stderr missing failure message; got:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "sentinel-ok") {
		t.Errorf("execution stopped after optional failure; subsequent hooks should still run; got:\n%s", stdout.String())
	}
}

func TestRunRequiredFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	hs := []manifest.Hook{mkHook("will-fail", "false", false)}
	err := hooks.Run(context.Background(), hs, t.TempDir(), nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil for required hook failure")
	}
	if !strings.Contains(err.Error(), "will-fail") {
		t.Errorf("error should mention hook name; got: %v", err)
	}
}

func TestRunStopsAfterRequiredFailure(t *testing.T) {
	workDir := t.TempDir()
	hs := []manifest.Hook{
		mkHook("fail-first", "false", false),
		mkHook("write-sentinel", "touch sentinel.txt", false),
	}
	var stdout, stderr bytes.Buffer
	_ = hooks.Run(context.Background(), hs, workDir, nil, &stdout, &stderr)

	if _, err := os.Stat(filepath.Join(workDir, "sentinel.txt")); err == nil {
		t.Error("sentinel.txt exists; second hook should not have run after required failure")
	}
}

func TestRunCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var stdout, stderr bytes.Buffer
	hs := []manifest.Hook{mkHook("long-sleep", "sleep 5", false)}
	start := time.Now()
	err := hooks.Run(ctx, hs, t.TempDir(), nil, &stdout, &stderr)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Run() error = nil, want non-nil after context cancellation")
	}
	if elapsed > 2*time.Second {
		t.Errorf("Run() took %v; should have been cancelled quickly", elapsed)
	}
}

func TestRunWorkDir(t *testing.T) {
	workDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	hs := []manifest.Hook{mkHook("print-wd", "pwd", false)}
	if err := hooks.Run(context.Background(), hs, workDir, nil, &stdout, &stderr); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	resolvedWorkDir, _ := filepath.EvalSymlinks(workDir)
	got := strings.TrimSpace(stdout.String())
	if !strings.Contains(got, resolvedWorkDir) {
		t.Errorf("pwd output = %q, want it to contain %q", got, resolvedWorkDir)
	}
}
