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
// suffix. All other files are copied verbatim (binary-safe).
//
// Source permission bits from the embedded FS are NOT propagated — Go's
// //go:embed strips them, so they are meaningless at the target. All
// regular files are written as 0644 and all directories as 0755.
//
// All writes go through a Stager so a single failure rolls back the entire
// operation. Callers must pass ctx with PascalCase keys — use
// ToTemplateContext to convert a resolved variable map before calling.
//
// Returns the sorted list of relative paths written, or an empty list if the
// blueprint's template/ directory exists but contains no files.
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

		if d.IsDir() {
			return nil // WriteFile creates parent dirs on demand
		}

		// Render each path segment so blueprints can use variables in
		// directory names and filenames (e.g. {{ .RouteName }}/route.ts.tmpl).
		segments := strings.Split(rel, "/")
		renderedSegs := make([]string, 0, len(segments))
		for _, seg := range segments {
			renderedSeg, err := Render("path:"+seg, seg, ctx)
			if err != nil {
				return fmt.Errorf("rendering path segment %q in %q: %w", seg, srcPath, err)
			}
			if strings.TrimSpace(renderedSeg) == "" {
				return nil // conditional evaluated to nothing — skip this file
			}
			renderedSegs = append(renderedSegs, renderedSeg)
		}
		destPath := strings.Join(renderedSegs, "/")

		content, err := fs.ReadFile(bp.FS, srcPath)
		if err != nil {
			return fmt.Errorf("reading %q: %w", srcPath, err)
		}

		if strings.HasSuffix(destPath, ".tmpl") {
			rendered, err := Render(rel, string(content), ctx)
			if err != nil {
				return fmt.Errorf("rendering %q: %w", srcPath, err)
			}
			if strings.TrimSpace(rendered) == "" {
				return nil // empty render means this file is excluded by a conditional
			}
			content = []byte(rendered)
			destPath = strings.TrimSuffix(destPath, ".tmpl")
		}

		if err := stager.WriteFile(destPath, content, 0o644); err != nil {
			return fmt.Errorf("writing %q: %w", destPath, err)
		}
		written = append(written, destPath)
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
