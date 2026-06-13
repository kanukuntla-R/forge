package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanukuntla-r/forge/internal/graph"
	"github.com/kanukuntla-r/forge/internal/project"
)

// scaffoldMinimalProject creates a temp directory with a .forge/project.json
// and an empty knowledge graph — the minimum a forge project needs for forge add.
func scaffoldMinimalProject(t *testing.T, blueprintName string) string {
	t.Helper()
	dir := t.TempDir()

	if err := project.Write(dir, project.Project{
		Blueprint:         blueprintName,
		BlueprintVersion:  "0.1.0",
		ForgeVersion:      "0.1.0-dev",
		CreatedAt:         "2026-01-01T00:00:00Z",
		Variables:         map[string]any{"name": blueprintName + "-demo"},
		ExtensionsApplied: []project.ExtensionApplication{},
	}); err != nil {
		t.Fatalf("scaffoldMinimalProject: writing marker: %v", err)
	}

	if err := graph.Write(&graph.Graph{
		Version: "1.0",
		Nodes:   []graph.Node{},
		Edges:   []graph.Edge{},
	}, dir); err != nil {
		t.Fatalf("scaffoldMinimalProject: writing graph: %v", err)
	}

	return dir
}

func TestRunAddNotInForgeProject(t *testing.T) {
	dir := t.TempDir()
	// .git stops project.Detect from walking above this directory, ensuring
	// it returns nil rather than finding a stale marker in a parent.
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := runAdd(context.Background(), &buf, "api-route", []string{"users"}, nil, runAddOptions{
		toPath: dir,
	})
	if err == nil {
		t.Fatal("expected error when not inside a forge project")
	}
	if !strings.Contains(err.Error(), "not inside a forge project") {
		t.Errorf("error should mention 'not inside a forge project', got: %v", err)
	}
}

func TestRunAddUnknownExtension(t *testing.T) {
	dir := scaffoldMinimalProject(t, "hackathon-app")

	var buf bytes.Buffer
	err := runAdd(context.Background(), &buf, "nonexistent", nil, nil, runAddOptions{
		toPath: dir,
	})
	if err == nil {
		t.Fatal("expected error for unknown extension")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention the unknown extension name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "api-route") {
		t.Errorf("error should list available extensions, got: %v", err)
	}
}

func TestRunAddApiRouteHappyPath(t *testing.T) {
	dir := scaffoldMinimalProject(t, "hackathon-app")

	var buf bytes.Buffer
	err := runAdd(context.Background(), &buf, "api-route", []string{"users"}, nil, runAddOptions{
		toPath: dir,
	})
	if err != nil {
		t.Fatalf("runAdd() error: %v", err)
	}

	// File created at the rendered path.
	routeFile := filepath.Join(dir, "app", "api", "users", "route.ts")
	content, err := os.ReadFile(routeFile)
	if err != nil {
		t.Fatalf("expected app/api/users/route.ts to exist: %v", err)
	}
	if !strings.Contains(string(content), "Hello from users") {
		t.Errorf("route.ts should contain 'Hello from users', got: %s", content)
	}

	// Knowledge graph has the new node.
	g, err := graph.Read(dir)
	if err != nil {
		t.Fatalf("graph.Read() error: %v", err)
	}
	found := false
	for _, n := range g.Nodes {
		if n.ID == "app/api/users" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("knowledge graph should have node id 'app/api/users', got nodes: %v", g.Nodes)
	}

	// Project marker records the extension application.
	det, err := project.Detect(dir)
	if err != nil || det == nil {
		t.Fatalf("project.Detect() error: %v, det: %v", err, det)
	}
	if len(det.Project.ExtensionsApplied) != 1 {
		t.Fatalf("ExtensionsApplied len = %d, want 1", len(det.Project.ExtensionsApplied))
	}
	app := det.Project.ExtensionsApplied[0]
	if app.Name != "api-route" {
		t.Errorf("ExtensionsApplied[0].Name = %q, want %q", app.Name, "api-route")
	}
	if app.Args["route_name"] != "users" {
		t.Errorf("ExtensionsApplied[0].Args[route_name] = %v, want %q", app.Args["route_name"], "users")
	}
}

func TestRunAddDuplicateExtension(t *testing.T) {
	dir := scaffoldMinimalProject(t, "hackathon-app")
	addOpts := runAddOptions{toPath: dir}

	var buf bytes.Buffer
	if err := runAdd(context.Background(), &buf, "api-route", []string{"users"}, nil, addOpts); err != nil {
		t.Fatalf("first runAdd() error: %v", err)
	}
	buf.Reset()
	if err := runAdd(context.Background(), &buf, "api-route", []string{"users"}, nil, addOpts); err != nil {
		t.Fatalf("second runAdd() error: %v", err)
	}

	// File content is correct and not doubled.
	content, err := os.ReadFile(filepath.Join(dir, "app", "api", "users", "route.ts"))
	if err != nil {
		t.Fatalf("route.ts not found: %v", err)
	}
	if strings.Count(string(content), "Hello from users") != 1 {
		t.Errorf("route.ts content should appear exactly once, got: %s", content)
	}

	// Graph node not duplicated (second merge was skipped).
	g, err := graph.Read(dir)
	if err != nil {
		t.Fatalf("graph.Read() error: %v", err)
	}
	count := 0
	for _, n := range g.Nodes {
		if n.ID == "app/api/users" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 node with id 'app/api/users', found %d", count)
	}

	// Marker records both applications (idempotency audit trail).
	det, err := project.Detect(dir)
	if err != nil || det == nil {
		t.Fatalf("project.Detect() error: %v", err)
	}
	if len(det.Project.ExtensionsApplied) != 2 {
		t.Errorf("ExtensionsApplied len = %d, want 2 (both applications recorded)", len(det.Project.ExtensionsApplied))
	}
}
