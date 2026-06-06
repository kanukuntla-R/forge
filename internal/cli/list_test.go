package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kanukuntla-r/forge/internal/blueprint"
)

func TestRunListHumanReadable(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}
	var buf bytes.Buffer
	if err := runList(&buf, r.List(), false); err != nil {
		t.Fatalf("runList() error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "hackathon-app") {
		t.Errorf("human output does not contain %q:\n%s", "hackathon-app", out)
	}
}

func TestRunListJSON(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}
	var buf bytes.Buffer
	if err := runList(&buf, r.List(), true); err != nil {
		t.Fatalf("runList() error: %v", err)
	}
	var items []blueprintListItem
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	found := false
	for _, item := range items {
		if item.Name == "hackathon-app" {
			found = true
			if item.Source != "embedded" {
				t.Errorf("Source = %q, want %q", item.Source, "embedded")
			}
			if item.Kind == "" {
				t.Error("Kind is empty")
			}
			if item.Version == "" {
				t.Error("Version is empty")
			}
			break
		}
	}
	if !found {
		t.Error("JSON output did not include hackathon-app")
	}
}
