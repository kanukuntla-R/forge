package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRunRenderNonexistentComponent(t *testing.T) {
	var out bytes.Buffer
	err := runRender(&out, "/nonexistent/path/Component.tsx", "", "", false)
	if err == nil {
		t.Fatal("runRender with nonexistent component: want error, got nil")
	}
	if !strings.Contains(err.Error(), "component not found") {
		t.Errorf("expected 'component not found' error, got: %v", err)
	}
}

// TestRunRenderBatchEmptyProject proves batch mode against a non-Next.js
// (zero-component) project is informational, not an error: exit 0, no
// Node/Playwright invocation needed since there's nothing to render.
func TestRunRenderBatchEmptyProject(t *testing.T) {
	dir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWD) })

	var out bytes.Buffer
	err = runRenderBatch(&out, "", "800x600", 4, false, false, false)
	if err != nil {
		t.Fatalf("runRenderBatch on empty project: unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Rendered 0/0 component(s)") {
		t.Errorf("expected informational 0/0 message, got: %q", out.String())
	}
}
