package blueprint

import "fmt"

// Registry holds all discovered blueprints and provides lookup and enumeration.
type Registry struct {
	blueprints []*Blueprint
}

// NewRegistry discovers and loads all blueprints from known sources.
// Currently only embedded blueprints are loaded; user-installed and
// project-local sources are added in M7.
func NewRegistry() (*Registry, error) {
	embedded, err := EmbeddedBlueprints()
	if err != nil {
		return nil, fmt.Errorf("loading embedded blueprints: %w", err)
	}
	r := &Registry{}
	for i := range embedded {
		r.blueprints = append(r.blueprints, &embedded[i])
	}
	// TODO M7: load user-installed blueprints from ~/.config/forge/blueprints/
	// TODO M7: load project-local blueprints from .forge/blueprints/
	return r, nil
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
