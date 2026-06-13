package blueprint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanukuntla-r/forge/internal/blueprint"
)

func TestRegistryListContainsHackathonApp(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}
	bps := r.List()
	if len(bps) == 0 {
		t.Fatal("List() returned no blueprints, want at least hackathon-app")
	}
	found := false
	for _, bp := range bps {
		if bp.Manifest.Name == "hackathon-app" {
			found = true
			break
		}
	}
	if !found {
		t.Error("List() did not include hackathon-app")
	}
}

func TestRegistryFindHackathonApp(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}
	bp, err := r.Find("hackathon-app")
	if err != nil {
		t.Fatalf("Find(%q) error: %v", "hackathon-app", err)
	}
	if bp.Manifest.Name != "hackathon-app" {
		t.Errorf("Manifest.Name = %q, want %q", bp.Manifest.Name, "hackathon-app")
	}
	if bp.Source != blueprint.SourceEmbedded {
		t.Errorf("Source = %v, want SourceEmbedded", bp.Source)
	}
	// FS must be rooted at the blueprint directory, not at blueprints/
	f, err := bp.FS.Open("manifest.yaml")
	if err != nil {
		t.Errorf("bp.FS.Open(manifest.yaml) error: %v — FS is not correctly sub'd", err)
	} else {
		f.Close()
	}
}

func TestRegistryFindNotFound(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}
	_, err = r.Find("does-not-exist")
	if err == nil {
		t.Fatal("Find(nonexistent) returned nil error, want error")
	}
}

func TestNewRegistryWithDiskDirLoadsDiskBlueprints(t *testing.T) {
	diskDir := t.TempDir()
	bpDir := filepath.Join(diskDir, "test-disk-bp")
	if err := os.MkdirAll(bpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, bpDir, "name: test-disk-bp\nkind: app\nversion: \"0.1.0\"\n")

	var stderr strings.Builder
	r, err := blueprint.NewRegistryWithDiskDir(diskDir, &stderr)
	if err != nil {
		t.Fatalf("NewRegistryWithDiskDir error: %v", err)
	}
	found := false
	for _, bp := range r.List() {
		if bp.Manifest.Name == "test-disk-bp" {
			found = true
			if bp.Source != blueprint.SourceUserInstalled {
				t.Errorf("Source = %v, want SourceUserInstalled", bp.Source)
			}
		}
	}
	if !found {
		t.Error("test-disk-bp not found in registry")
	}
}

func TestNewRegistryWithDiskDirShadowedByEmbedded(t *testing.T) {
	diskDir := t.TempDir()
	bpDir := filepath.Join(diskDir, "hackathon-app") // same name as embedded
	if err := os.MkdirAll(bpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, bpDir, "name: hackathon-app\nkind: app\nversion: \"9.9.9\"\n")

	var stderr strings.Builder
	r, err := blueprint.NewRegistryWithDiskDir(diskDir, &stderr)
	if err != nil {
		t.Fatalf("NewRegistryWithDiskDir error: %v", err)
	}
	var found []*blueprint.Blueprint
	for _, bp := range r.List() {
		if bp.Manifest.Name == "hackathon-app" {
			found = append(found, bp)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly 1 hackathon-app, got %d", len(found))
	}
	if found[0].Source != blueprint.SourceEmbedded {
		t.Errorf("hackathon-app Source = %v, want SourceEmbedded (embedded wins)", found[0].Source)
	}
	if !strings.Contains(stderr.String(), "shadowed") {
		t.Errorf("expected 'shadowed' warning in stderr, got: %q", stderr.String())
	}
}

func TestNewRegistryWithDiskDirSkipsInvalidBlueprint(t *testing.T) {
	diskDir := t.TempDir()
	badDir := filepath.Join(diskDir, "bad-bp")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, badDir, "name: bad-bp\nversion: \"0.1.0\"\n") // missing kind

	goodDir := filepath.Join(diskDir, "good-bp")
	if err := os.MkdirAll(goodDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, goodDir, "name: good-bp\nkind: app\nversion: \"0.1.0\"\n")

	var stderr strings.Builder
	r, err := blueprint.NewRegistryWithDiskDir(diskDir, &stderr)
	if err != nil {
		t.Fatalf("NewRegistryWithDiskDir error: %v", err)
	}
	names := map[string]bool{}
	for _, bp := range r.List() {
		names[bp.Manifest.Name] = true
	}
	if names["bad-bp"] {
		t.Error("bad-bp (invalid manifest) should not be in registry")
	}
	if !names["good-bp"] {
		t.Error("good-bp (valid manifest) should be in registry")
	}
	if !strings.Contains(stderr.String(), "bad-bp") {
		t.Errorf("expected warning about bad-bp in stderr, got: %q", stderr.String())
	}
}

func writeManifest(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
