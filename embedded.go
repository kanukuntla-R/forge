// Package forge exists solely to embed the blueprints/ directory into the
// binary. Go's //go:embed directive requires paths relative to the source file
// and forbids ".."; because blueprints/ lives at the project root (where it is
// visible as first-class content, not an internal detail), the embed directive
// must live here too. internal/blueprint imports this package to access the
// embedded filesystem. See the Architecture note in docs/forge-design.md.
package forge

import "embed"

// Version is set at build time via -ldflags. The default value is the
// local development indicator; release builds inject the real version.
var Version = "0.1.0-dev"

// all: is required so that template files beginning with '.' or '_'
// (e.g. .gitignore, .env.example) are included when we add the template tree.
//
//go:embed all:blueprints
var blueprintsFS embed.FS

// BlueprintsFS returns the embedded filesystem containing all built-in
// blueprints. Paths within the FS start with "blueprints/" — callers
// typically sub into it with fs.Sub(BlueprintsFS(), "blueprints").
func BlueprintsFS() embed.FS {
	return blueprintsFS
}
