package cli

import (
	"bytes"
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
