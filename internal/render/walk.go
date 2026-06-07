package render

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/kanukuntla-r/forge/internal/blueprint"
	"github.com/kanukuntla-r/forge/internal/fsutil"
)

// WriteBlueprint renders all files from bp.FS/template/ into targetPath.
//
// Files ending in .tmpl are rendered with ctx and written without the .tmpl
// suffix. All other files are copied verbatim (binary-safe). Directory
// structure and file permission bits are preserved exactly.
//
// All writes go through a Stager so a single failure rolls back the entire
// operation. Callers must pass ctx with PascalCase keys — use
// ToTemplateContext to convert a resolved variable map before calling.
//
// Returns the sorted list of relative paths written, or nil if the blueprint
// has no template/ directory (not an error — some blueprints are manifest-only).
func WriteBlueprint(bp *blueprint.Blueprint, ctx map[string]any, targetPath string) ([]string, error) {
	const templateRoot = "template"

	if _, err := fs.Stat(bp.FS, templateRoot); err != nil {
		return nil, nil
	}

	stager, err := fsutil.NewStager(targetPath)
	if err != nil {
		return nil, fmt.Errorf("creating stager: %w", err)
	}
	defer stager.Discard()

	var written []string

	walkErr := fs.WalkDir(bp.FS, templateRoot, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if srcPath == templateRoot {
			return nil
		}

		rel := strings.TrimPrefix(srcPath, templateRoot+"/")

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat %q: %w", srcPath, err)
		}

		if d.IsDir() {
			// Ensure owner rwx so we can write files into the staged directory.
			// Source dirs (especially fstest.MapFS synthetics) may lack the write bit.
			mode := info.Mode().Perm() | 0o700
			return stager.Mkdir(rel, mode)
		}

		content, err := fs.ReadFile(bp.FS, srcPath)
		if err != nil {
			return fmt.Errorf("reading %q: %w", srcPath, err)
		}

		outPath := rel
		if strings.HasSuffix(rel, ".tmpl") {
			rendered, err := Render(rel, string(content), ctx)
			if err != nil {
				return fmt.Errorf("rendering %q: %w", srcPath, err)
			}
			content = []byte(rendered)
			outPath = strings.TrimSuffix(rel, ".tmpl")
		}

		if err := stager.WriteFile(outPath, content, info.Mode().Perm()); err != nil {
			return fmt.Errorf("writing %q: %w", outPath, err)
		}
		written = append(written, outPath)
		return nil
	})

	if walkErr != nil {
		return nil, walkErr
	}

	sort.Strings(written)

	if err := stager.Commit(); err != nil {
		return nil, fmt.Errorf("committing to %q: %w", targetPath, err)
	}
	return written, nil
}
