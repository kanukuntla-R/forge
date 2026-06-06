package blueprint

import (
	"io/fs"

	"github.com/kanukuntla-r/forge/internal/manifest"
)

// Source indicates where a blueprint was discovered.
type Source int

const (
	SourceEmbedded      Source = iota // built into the forge binary
	SourceUserInstalled               // ~/.config/forge/blueprints/
	SourceProjectLocal                // .forge/blueprints/
)

// Blueprint is a loaded blueprint ready for use by the render engine.
// FS is rooted at the blueprint's own directory, so callers can open
// "manifest.yaml" or "template/package.json.tmpl" without path prefixes.
type Blueprint struct {
	Manifest *manifest.Manifest
	FS       fs.FS
	Source   Source
}
