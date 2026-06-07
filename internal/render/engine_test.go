package render_test

import (
	"strings"
	"testing"

	"github.com/kanukuntla-r/forge/internal/manifest"
	"github.com/kanukuntla-r/forge/internal/render"
)

func TestRenderBasicInterpolation(t *testing.T) {
	got, err := render.Render("test", `{{ .Name }}`, map[string]any{"Name": "hello"})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestRenderConditional(t *testing.T) {
	got, err := render.Render("test", `{{ if .WithAi }}yes{{ end }}`, map[string]any{"WithAi": true})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if got != "yes" {
		t.Errorf("got %q, want %q", got, "yes")
	}
}

func TestRenderSeqFunc(t *testing.T) {
	got, err := render.Render("test", `{{ range seq 1 3 }}{{ . }}{{ end }}`, map[string]any{})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if got != "123" {
		t.Errorf("got %q, want %q", got, "123")
	}
}

func TestRenderMissingKeyError(t *testing.T) {
	_, err := render.Render("test", `{{ .Missing }}`, map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
	if !strings.Contains(err.Error(), "Missing") {
		t.Errorf("error should mention the missing key name, got: %v", err)
	}
}

func TestRenderMalformedTemplateSyntax(t *testing.T) {
	_, err := render.Render("test", `{{ unclosed`, map[string]any{})
	if err == nil {
		t.Fatal("expected parse error for malformed template, got nil")
	}
}

func TestRenderFileReadsFromReader(t *testing.T) {
	tmpl := `Hello {{ .Place }}`
	got, err := render.RenderFile("test", strings.NewReader(tmpl), map[string]any{"Place": "world"})
	if err != nil {
		t.Fatalf("RenderFile() error: %v", err)
	}
	if got != "Hello world" {
		t.Errorf("got %q, want %q", got, "Hello world")
	}
}

// TestRenderHackathonAppIntegration verifies the full pipeline:
// manifest → Resolve → ToTemplateContext → Render.
func TestRenderHackathonAppIntegration(t *testing.T) {
	m, err := manifest.ParseFile("../manifest/testdata/hackathon-app.yaml")
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}
	values, err := render.Resolve(m, nil, nil, false)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	ctx := render.ToTemplateContext(values)
	got, err := render.Render("integration", `Hello {{ .Description }}`, ctx)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	want := "Hello A hackathon app"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
