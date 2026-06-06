package manifest_test

import (
	"strings"
	"testing"

	"github.com/kanukuntla-r/forge/internal/manifest"
)

func TestParseHackathonApp(t *testing.T) {
	m, err := manifest.ParseFile("testdata/hackathon-app.yaml")
	if err != nil {
		t.Fatalf("ParseFile returned unexpected error: %v", err)
	}

	// Identity
	if m.Name != "hackathon-app" {
		t.Errorf("Name = %q, want %q", m.Name, "hackathon-app")
	}
	if m.DisplayName != "Hackathon Web App" {
		t.Errorf("DisplayName = %q, want %q", m.DisplayName, "Hackathon Web App")
	}
	if m.Version != "0.1.0" {
		t.Errorf("Version = %q, want %q", m.Version, "0.1.0")
	}
	if len(m.Authors) != 1 || m.Authors[0] != "kanukuntla-r" {
		t.Errorf("Authors = %v, want [kanukuntla-r]", m.Authors)
	}
	if m.Kind != manifest.KindApp {
		t.Errorf("Kind = %q, want %q", m.Kind, manifest.KindApp)
	}

	// Stack
	if m.Stack.Language != "typescript" {
		t.Errorf("Stack.Language = %q, want %q", m.Stack.Language, "typescript")
	}
	if m.Stack.Runtime != "node" {
		t.Errorf("Stack.Runtime = %q, want %q", m.Stack.Runtime, "node")
	}
	if m.Stack.Framework != "nextjs" {
		t.Errorf("Stack.Framework = %q, want %q", m.Stack.Framework, "nextjs")
	}
	if len(m.Stack.Tags) != 4 {
		t.Errorf("Stack.Tags = %v, want 4 items", m.Stack.Tags)
	}

	// Variables — 6 total
	if len(m.Variables) != 6 {
		t.Fatalf("Variables len = %d, want 6", len(m.Variables))
	}

	// description: string with string default
	v := m.Variables[0]
	if v.Name != "description" {
		t.Errorf("Variables[0].Name = %q, want %q", v.Name, "description")
	}
	if v.Type != manifest.VarTypeString {
		t.Errorf("Variables[0].Type = %q, want %q", v.Type, manifest.VarTypeString)
	}
	if v.Default != "A hackathon app" {
		t.Errorf("Variables[0].Default = %v (%T), want %q", v.Default, v.Default, "A hackathon app")
	}

	// with_database: bool with false default
	v = m.Variables[1]
	if v.Name != "with_database" {
		t.Errorf("Variables[1].Name = %q, want %q", v.Name, "with_database")
	}
	if v.Type != manifest.VarTypeBool {
		t.Errorf("Variables[1].Type = %q, want %q", v.Type, manifest.VarTypeBool)
	}
	if v.Default != false {
		t.Errorf("Variables[1].Default = %v (%T), want false", v.Default, v.Default)
	}

	// with_ai: bool with true default
	v = m.Variables[3]
	if v.Name != "with_ai" {
		t.Errorf("Variables[3].Name = %q, want %q", v.Name, "with_ai")
	}
	if v.Default != true {
		t.Errorf("Variables[3].Default = %v (%T), want true", v.Default, v.Default)
	}

	// package_manager: choice with choices list
	v = m.Variables[5]
	if v.Name != "package_manager" {
		t.Errorf("Variables[5].Name = %q, want %q", v.Name, "package_manager")
	}
	if v.Type != manifest.VarTypeChoice {
		t.Errorf("Variables[5].Type = %q, want %q", v.Type, manifest.VarTypeChoice)
	}
	if v.Default != "pnpm" {
		t.Errorf("Variables[5].Default = %v (%T), want %q", v.Default, v.Default, "pnpm")
	}
	if len(v.Choices) != 4 {
		t.Errorf("Variables[5].Choices = %v, want [pnpm npm yarn bun]", v.Choices)
	}

	// Target
	if m.Target.DefaultPath != "./{{ .Name }}" {
		t.Errorf("Target.DefaultPath = %q, want %q", m.Target.DefaultPath, "./{{ .Name }}")
	}

	// PostCreate hooks
	if len(m.PostCreate) != 2 {
		t.Fatalf("PostCreate len = %d, want 2", len(m.PostCreate))
	}
	if m.PostCreate[0].Name != "Initialize git" {
		t.Errorf("PostCreate[0].Name = %q, want %q", m.PostCreate[0].Name, "Initialize git")
	}
	if !m.PostCreate[0].Optional {
		t.Errorf("PostCreate[0].Optional = false, want true")
	}
	if m.PostCreate[1].Name != "Install dependencies" {
		t.Errorf("PostCreate[1].Name = %q, want %q", m.PostCreate[1].Name, "Install dependencies")
	}
	if m.PostCreate[1].Optional {
		t.Errorf("PostCreate[1].Optional = true, want false")
	}

	// Extensions
	if len(m.Extensions) != 3 {
		t.Fatalf("Extensions len = %d, want 3", len(m.Extensions))
	}
	ext := m.Extensions[0]
	if ext.Name != "api-route" {
		t.Errorf("Extensions[0].Name = %q, want %q", ext.Name, "api-route")
	}
	if ext.Template != "extensions/api-route/template" {
		t.Errorf("Extensions[0].Template = %q", ext.Template)
	}
	if len(ext.Args) != 1 || ext.Args[0].Name != "route_name" {
		t.Errorf("Extensions[0].Args = %v, want [{route_name ...}]", ext.Args)
	}
	if ext.GraphFragment != "extensions/api-route/graph-fragment.yaml" {
		t.Errorf("Extensions[0].GraphFragment = %q", ext.GraphFragment)
	}
	// page and component extensions have no GraphFragment
	if m.Extensions[1].Name != "page" {
		t.Errorf("Extensions[1].Name = %q, want %q", m.Extensions[1].Name, "page")
	}
	if m.Extensions[2].Name != "component" {
		t.Errorf("Extensions[2].Name = %q, want %q", m.Extensions[2].Name, "component")
	}
}

func TestParseInvalidYAML(t *testing.T) {
	_, err := manifest.Parse(strings.NewReader("name: [unclosed"))
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestParseMissingName(t *testing.T) {
	input := `
display_name: "No Name"
kind: app
version: "0.1.0"
`
	_, err := manifest.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestParseInvalidKind(t *testing.T) {
	input := `
name: test-blueprint
kind: website
version: "0.1.0"
`
	_, err := manifest.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid kind, got nil")
	}
}

func TestParseChoiceVariableMissingChoices(t *testing.T) {
	input := `
name: test-blueprint
kind: app
version: "0.1.0"
variables:
  - name: color
    type: choice
`
	_, err := manifest.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for choice variable with no choices, got nil")
	}
}

func TestParseInvalidVariablePattern(t *testing.T) {
	input := `
name: test-blueprint
kind: app
version: "0.1.0"
variables:
  - name: slug
    type: string
    pattern: "[invalid"
`
	_, err := manifest.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid regex pattern, got nil")
	}
	if !strings.Contains(err.Error(), "slug") {
		t.Errorf("error should mention variable name, got: %v", err)
	}
}

func TestParseFileNotFound(t *testing.T) {
	_, err := manifest.ParseFile("testdata/does-not-exist.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
