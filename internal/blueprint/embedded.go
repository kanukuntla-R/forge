package blueprint

import (
	"fmt"
	"io/fs"

	forge "github.com/kanukuntla-r/forge"
	"github.com/kanukuntla-r/forge/internal/manifest"
)

// EmbeddedBlueprints loads and returns all blueprints baked into the binary.
// Each Blueprint.FS is rooted at the blueprint's own directory, so callers can
// open "manifest.yaml" or "template/package.json.tmpl" directly.
func EmbeddedBlueprints() ([]Blueprint, error) {
	root, err := fs.Sub(forge.BlueprintsFS(), "blueprints")
	if err != nil {
		return nil, fmt.Errorf("accessing embedded blueprints: %w", err)
	}
	entries, err := fs.ReadDir(root, ".")
	if err != nil {
		return nil, fmt.Errorf("reading embedded blueprints directory: %w", err)
	}
	var result []Blueprint
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		bpFS, err := fs.Sub(root, e.Name())
		if err != nil {
			return nil, fmt.Errorf("blueprint %q: sub-FS: %w", e.Name(), err)
		}
		f, err := bpFS.Open("manifest.yaml")
		if err != nil {
			return nil, fmt.Errorf("blueprint %q: opening manifest.yaml: %w", e.Name(), err)
		}
		m, parseErr := manifest.Parse(f)
		f.Close()
		if parseErr != nil {
			return nil, fmt.Errorf("blueprint %q: parsing manifest: %w", e.Name(), parseErr)
		}
		result = append(result, Blueprint{
			Manifest: m,
			FS:       bpFS,
			Source:   SourceEmbedded,
		})
	}
	return result, nil
}
