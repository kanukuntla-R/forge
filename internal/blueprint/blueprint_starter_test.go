package blueprint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kanukuntla-r/forge/internal/blueprint"
	"github.com/kanukuntla-r/forge/internal/manifest"
	"github.com/kanukuntla-r/forge/internal/render"
)

func TestBlueprintStarterManifest(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("blueprint-starter")
	if err != nil {
		t.Fatalf("Find(blueprint-starter): %v", err)
	}
	if bp.Manifest.Name != "blueprint-starter" {
		t.Errorf("Name = %q, want blueprint-starter", bp.Manifest.Name)
	}
	if bp.Manifest.Kind != manifest.KindApp {
		t.Errorf("Kind = %q, want app", bp.Manifest.Kind)
	}
	varNames := map[string]bool{}
	for _, v := range bp.Manifest.Variables {
		varNames[v.Name] = true
	}
	if !varNames["description"] {
		t.Error("manifest missing variable 'description'")
	}
	if !varNames["author"] {
		t.Error("manifest missing variable 'author'")
	}
	// FS must be rooted at the blueprint directory
	f, err := bp.FS.Open("manifest.yaml")
	if err != nil {
		t.Errorf("bp.FS.Open(manifest.yaml): %v — FS not correctly sub'd", err)
	} else {
		f.Close()
	}
}

func TestBlueprintStarterScaffolds(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("blueprint-starter")
	if err != nil {
		t.Fatalf("Find(blueprint-starter): %v", err)
	}

	dir := filepath.Join(t.TempDir(), "output")
	ctx := map[string]any{
		"Name":        "my-blueprint",
		"Description": "A test blueprint",
		"Author":      "",
	}
	written, err := render.WriteBlueprint(bp, ctx, dir)
	if err != nil {
		t.Fatalf("WriteBlueprint: %v", err)
	}

	want := []string{
		"README.md",
		"extensions/page/graph-fragment.yaml",
		"extensions/page/template/page.html",
		"graph.yaml",
		"manifest.yaml",
		"template/app.js",
		"template/index.html",
		"template/style.css",
	}
	got := map[string]bool{}
	for _, f := range written {
		got[f] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("expected file %q not written; written: %v", w, written)
		}
	}
}

func TestScaffoldedBlueprintIsValid(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("blueprint-starter")
	if err != nil {
		t.Fatalf("Find(blueprint-starter): %v", err)
	}

	dir := filepath.Join(t.TempDir(), "output")
	ctx := map[string]any{
		"Name":        "my-blueprint",
		"Description": "A test blueprint",
		"Author":      "Test User",
	}
	if _, err := render.WriteBlueprint(bp, ctx, dir); err != nil {
		t.Fatalf("WriteBlueprint: %v", err)
	}

	m, err := manifest.ParseFile(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		t.Fatalf("scaffolded manifest.yaml failed to parse: %v", err)
	}
	if m.Name != "my-blueprint" {
		t.Errorf("scaffolded Name = %q, want my-blueprint", m.Name)
	}
	if m.Kind == "" {
		t.Error("scaffolded manifest missing kind")
	}
	if len(m.Extensions) == 0 {
		t.Fatal("scaffolded manifest has no extensions")
	}
	if m.Extensions[0].Name != "page" {
		t.Errorf("extensions[0].Name = %q, want page", m.Extensions[0].Name)
	}
	if m.Extensions[0].GraphFragment == "" {
		t.Error("page extension missing graph_fragment")
	}
}

func TestNoteAppScaffolds(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("blueprint-starter")
	if err != nil {
		t.Fatalf("Find(blueprint-starter): %v", err)
	}

	// Step 1: scaffold blueprint-starter into a temp dir
	bpDir := filepath.Join(t.TempDir(), "blueprint")
	ctx := map[string]any{
		"Name":        "my-blueprint",
		"Description": "A test blueprint",
		"Author":      "",
	}
	if _, err := render.WriteBlueprint(bp, ctx, bpDir); err != nil {
		t.Fatalf("scaffolding blueprint-starter: %v", err)
	}

	// Step 2: parse the scaffolded blueprint's manifest
	m, err := manifest.ParseFile(filepath.Join(bpDir, "manifest.yaml"))
	if err != nil {
		t.Fatalf("parsing scaffolded manifest: %v", err)
	}
	scaffoldedBP := &blueprint.Blueprint{
		Manifest: m,
		FS:       os.DirFS(bpDir),
		Source:   blueprint.SourceProjectLocal,
	}

	// Step 3: scaffold a note app from the scaffolded blueprint
	appDir := filepath.Join(t.TempDir(), "app")
	appCtx := map[string]any{
		"Name":        "my-notes",
		"ProjectName": "my-notes",
	}
	written, err := render.WriteBlueprint(scaffoldedBP, appCtx, appDir)
	if err != nil {
		t.Fatalf("scaffolding note app: %v", err)
	}

	want := []string{"app.js", "index.html", "style.css"}
	got := map[string]bool{}
	for _, f := range written {
		got[f] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("note app missing %q; written: %v", w, written)
		}
		if _, err := os.Stat(filepath.Join(appDir, w)); err != nil {
			t.Errorf("%q not on disk: %v", w, err)
		}
	}
}
