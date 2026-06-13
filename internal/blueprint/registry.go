package blueprint

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kanukuntla-r/forge/internal/manifest"
)

// Registry holds all discovered blueprints and provides lookup and enumeration.
type Registry struct {
	blueprints []*Blueprint
}

// ConfigBlueprintsDir returns the platform-appropriate directory for user-installed blueprints.
func ConfigBlueprintsDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "forge", "blueprints"), nil
}

// NewRegistry discovers and loads all blueprints from embedded and disk sources.
func NewRegistry() (*Registry, error) {
	diskDir, err := ConfigBlueprintsDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "forge: warning: could not determine config directory: %v\n", err)
		diskDir = ""
	}
	return NewRegistryWithDiskDir(diskDir, os.Stderr)
}

// NewRegistryWithDiskDir loads embedded blueprints plus any blueprints found in diskDir.
// Pass diskDir="" to skip disk loading. stderr receives warnings for bad or shadowed blueprints.
func NewRegistryWithDiskDir(diskDir string, stderr io.Writer) (*Registry, error) {
	embedded, err := EmbeddedBlueprints()
	if err != nil {
		return nil, fmt.Errorf("loading embedded blueprints: %w", err)
	}
	r := &Registry{}
	embeddedNames := make(map[string]bool, len(embedded))
	for i := range embedded {
		r.blueprints = append(r.blueprints, &embedded[i])
		embeddedNames[embedded[i].Manifest.Name] = true
	}
	if diskDir != "" {
		r.blueprints = append(r.blueprints, loadDiskBlueprints(diskDir, embeddedNames, stderr)...)
	}
	return r, nil
}

// loadDiskBlueprints walks diskDir and loads each subdirectory as a blueprint.
// Errors for individual blueprints are logged to stderr; the function never fails overall.
func loadDiskBlueprints(diskDir string, embeddedNames map[string]bool, stderr io.Writer) []*Blueprint {
	entries, err := os.ReadDir(diskDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		fmt.Fprintf(stderr, "forge: warning: reading blueprints directory %s: %v\n", diskDir, err)
		return nil
	}
	var result []*Blueprint
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if embeddedNames[name] {
			fmt.Fprintf(stderr, "forge: warning: blueprint %q shadowed by embedded; skipping disk version at %s\n",
				name, filepath.Join(diskDir, name))
			continue
		}
		subdirPath := filepath.Join(diskDir, name)
		m, err := manifest.ParseFile(filepath.Join(subdirPath, "manifest.yaml"))
		if err != nil {
			fmt.Fprintf(stderr, "forge: warning: blueprint %q: %v; skipping\n", name, err)
			continue
		}
		result = append(result, &Blueprint{
			Manifest: m,
			FS:       os.DirFS(subdirPath),
			Source:   SourceUserInstalled,
		})
	}
	return result
}

// Find returns the blueprint with the given name, or an error if not found.
// Matching is case-sensitive against the manifest's `name` field.
func (r *Registry) Find(name string) (*Blueprint, error) {
	for _, bp := range r.blueprints {
		if bp.Manifest.Name == name {
			return bp, nil
		}
	}
	return nil, fmt.Errorf("blueprint %q not found; run 'forge list' to see available blueprints", name)
}

// List returns all discovered blueprints across all sources.
func (r *Registry) List() []*Blueprint {
	return r.blueprints
}
