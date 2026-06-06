package blueprint_test

import (
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
