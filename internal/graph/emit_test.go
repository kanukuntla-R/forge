package graph_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kanukuntla-r/forge/internal/blueprint"
	"github.com/kanukuntla-r/forge/internal/graph"
	"github.com/kanukuntla-r/forge/internal/manifest"
)

func newBlueprintWithFS(files fstest.MapFS) *blueprint.Blueprint {
	return &blueprint.Blueprint{
		Manifest: &manifest.Manifest{Name: "test", Kind: manifest.KindApp},
		FS:       files,
		Source:   blueprint.SourceEmbedded,
	}
}

const basicGraphYAML = `version: "1.0"
project:
  name: test-project
  description: A test project
nodes:
  - id: node-1
    type: module
    label: Main Module
    path: src/main.go
edges:
  - from: node-1
    to: node-2
    type: imports
`

func TestRenderBasicGraph(t *testing.T) {
	bp := newBlueprintWithFS(fstest.MapFS{
		"graph.yaml": &fstest.MapFile{Data: []byte(basicGraphYAML)},
	})

	g, err := graph.Render(bp, map[string]any{})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if g == nil {
		t.Fatal("Render() returned nil graph, want non-nil")
	}

	if g.Version != "1.0" {
		t.Errorf("Version = %q, want %q", g.Version, "1.0")
	}
	if g.Project.Name != "test-project" {
		t.Errorf("Project.Name = %q, want %q", g.Project.Name, "test-project")
	}
	if len(g.Nodes) != 1 {
		t.Fatalf("len(Nodes) = %d, want 1", len(g.Nodes))
	}
	if g.Nodes[0].ID != "node-1" || g.Nodes[0].Type != "module" {
		t.Errorf("Nodes[0] = %+v, want id=node-1 type=module", g.Nodes[0])
	}
	if len(g.Edges) != 1 {
		t.Fatalf("len(Edges) = %d, want 1", len(g.Edges))
	}
	if g.Edges[0].From != "node-1" || g.Edges[0].Type != "imports" {
		t.Errorf("Edges[0] = %+v, want from=node-1 type=imports", g.Edges[0])
	}
}

// conditionalGraphYAML has an AI node+edge gated on .WithAi.
// The template is designed so both the true and false renders produce valid YAML:
// with WithAi=false the nodes list has one entry and edges is empty (null).
const conditionalGraphYAML = `version: "1.0"
project:
  name: my-app
  description: App with optional AI
nodes:
  - id: app
    type: module
    label: App
    path: src/app.go{{ if .WithAi }}
  - id: ai
    type: module
    label: AI
    path: src/ai.go{{ end }}
edges:{{ if .WithAi }}
  - from: app
    to: ai
    type: imports{{ end }}
`

func TestRenderGraphWithConditionals(t *testing.T) {
	bp := newBlueprintWithFS(fstest.MapFS{
		"graph.yaml": &fstest.MapFile{Data: []byte(conditionalGraphYAML)},
	})

	t.Run("WithAi=true", func(t *testing.T) {
		g, err := graph.Render(bp, map[string]any{"WithAi": true})
		if err != nil {
			t.Fatalf("Render() error: %v", err)
		}
		if len(g.Nodes) != 2 {
			t.Errorf("len(Nodes) = %d, want 2", len(g.Nodes))
		}
		if len(g.Edges) != 1 {
			t.Errorf("len(Edges) = %d, want 1", len(g.Edges))
		}
	})

	t.Run("WithAi=false", func(t *testing.T) {
		g, err := graph.Render(bp, map[string]any{"WithAi": false})
		if err != nil {
			t.Fatalf("Render() error: %v", err)
		}
		if len(g.Nodes) != 1 {
			t.Errorf("len(Nodes) = %d, want 1", len(g.Nodes))
		}
		if len(g.Edges) != 0 {
			t.Errorf("len(Edges) = %d, want 0", len(g.Edges))
		}
	})
}

func TestRenderGraphTemplateError(t *testing.T) {
	bp := newBlueprintWithFS(fstest.MapFS{
		"graph.yaml": &fstest.MapFile{Data: []byte(`{{ unclosed`)},
	})

	_, err := graph.Render(bp, map[string]any{})
	if err == nil {
		t.Fatal("expected error for malformed template, got nil")
	}
	if !strings.Contains(err.Error(), "rendering") {
		t.Errorf("error should mention rendering stage, got: %v", err)
	}
}

func TestRenderGraphInvalidYAML(t *testing.T) {
	// Renders to syntactically valid Go template output but invalid YAML
	// (unclosed flow sequence).
	bp := newBlueprintWithFS(fstest.MapFS{
		"graph.yaml": &fstest.MapFile{Data: []byte(`nodes: [unclosed`)},
	})

	_, err := graph.Render(bp, map[string]any{})
	if err == nil {
		t.Fatal("expected error for invalid YAML output, got nil")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("error should mention parsing stage, got: %v", err)
	}
}

func TestRenderGraphAbsent(t *testing.T) {
	bp := newBlueprintWithFS(fstest.MapFS{
		"manifest.yaml": &fstest.MapFile{Data: []byte("name: test")},
	})

	g, err := graph.Render(bp, map[string]any{})
	if err != nil {
		t.Fatalf("Render() error: %v, want nil", err)
	}
	if g != nil {
		t.Errorf("Render() = %+v, want nil when graph.yaml absent", g)
	}
}

func TestWriteCreatesDirectory(t *testing.T) {
	targetPath := t.TempDir()

	g := &graph.Graph{Version: "1.0", Project: graph.Project{Name: "p"}}
	if err := graph.Write(g, targetPath); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	dir := filepath.Join(targetPath, ".understand-anything")
	checkMode(t, dir, 0o755)

	file := filepath.Join(dir, "knowledge-graph.json")
	checkMode(t, file, 0o644)
}

func TestWriteValidJSON(t *testing.T) {
	want := &graph.Graph{
		Version: "1.0",
		Project: graph.Project{Name: "myapp", Description: "test"},
		Nodes: []graph.Node{
			{ID: "a", Type: "module", Label: "A", Path: "src/a.go"},
		},
		Edges: []graph.Edge{
			{From: "a", To: "b", Type: "imports"},
		},
	}

	targetPath := t.TempDir()
	if err := graph.Write(want, targetPath); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(targetPath, ".understand-anything", "knowledge-graph.json"))
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}

	var got graph.Graph
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Version != want.Version || got.Project.Name != want.Project.Name {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
	}
	if len(got.Nodes) != 1 || got.Nodes[0].ID != "a" {
		t.Errorf("Nodes round-trip: got %+v", got.Nodes)
	}
	if len(got.Edges) != 1 || got.Edges[0].From != "a" {
		t.Errorf("Edges round-trip: got %+v", got.Edges)
	}
}

func checkMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s mode = %04o, want %04o", path, got, want)
	}
}
