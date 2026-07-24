package analyzer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ignoreDirs is the set of directory names skipped entirely during the walk.
var ignoreDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".next":        true,
	"dist":         true,
	"build":        true,
	"out":          true,
	"coverage":     true,
	".cache":       true,
}

// ignoreFiles is the set of exact filenames always skipped.
var ignoreFiles = map[string]bool{
	"package-lock.json": true,
	"pnpm-lock.yaml":    true,
	"yarn.lock":         true,
	"bun.lockb":         true,
	".DS_Store":         true,
	"Thumbs.db":         true,
}

// ignoreExts is the set of file extensions always skipped (binary/asset types).
var ignoreExts = map[string]bool{
	".ico":   true,
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".gif":   true,
	".svg":   true,
	".woff":  true,
	".woff2": true,
	".ttf":   true,
	".otf":   true,
	".eot":   true,
	".pdf":   true,
	".zip":   true,
	".tar":   true,
	".gz":    true,
}

// selfOutputPath is the relative path of the analyzer's own output file.
// We skip it so analysis.json never appears in its own output.
var selfOutputPath = filepath.Join(".forge", "analysis.json")

// IsIgnoredDir reports whether a directory with the given base name should be
// skipped during analysis and file watching.
func IsIgnoredDir(name string) bool {
	return ignoreDirs[name]
}

// isPythonTestFile reports whether rel looks like a Python test file:
// test_*.py, *_test.py, or anywhere under a tests/ directory.
func isPythonTestFile(rel string) bool {
	base := filepath.Base(rel)
	if strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") {
		return true
	}
	slashed := "/" + filepath.ToSlash(rel) + "/"
	return strings.Contains(slashed, "/tests/")
}

// WalkResult holds the assembled analysis and any non-fatal per-file errors.
type WalkResult struct {
	Analysis *ProjectAnalysis
	Errors   []error
}

// Walk walks projectRoot, dispatches each source file to the appropriate adapter,
// and returns the assembled ProjectAnalysis. Non-fatal errors (e.g. unreadable
// files) accumulate in WalkResult.Errors without halting the walk.
func Walk(projectRoot string, registry *Registry) (*WalkResult, error) {
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	analysis := &ProjectAnalysis{
		Version:     "1",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Project: ProjectInfo{
			Root:       absRoot,
			Name:       filepath.Base(absRoot),
			Languages:  []string{},
			Frameworks: []string{},
		},
		Files: []FileInfo{},
	}
	result := &WalkResult{Analysis: analysis}

	walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", path, err))
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(absRoot, path)
		if rel == "." {
			return nil
		}

		if d.IsDir() {
			base := d.Name()
			if ignoreDirs[base] {
				return filepath.SkipDir
			}
			// Skip all dot-directories except .forge (our output location).
			if strings.HasPrefix(base, ".") && base != ".forge" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip the analyzer's own output to prevent self-reference.
		if rel == selfOutputPath {
			return nil
		}

		base := d.Name()
		if ignoreFiles[base] {
			return nil
		}
		if strings.HasSuffix(base, ".swp") || strings.HasSuffix(base, "~") {
			return nil
		}
		if ignoreExts[filepath.Ext(base)] {
			return nil
		}

		info, statErr := d.Info()
		if statErr != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: stat: %w", rel, statErr))
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: read: %w", rel, readErr))
			return nil
		}

		adapter := registry.Dispatch(path, content)
		fa, analyzeErr := adapter.Analyze(path, content)
		if analyzeErr != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: analyze: %w", rel, analyzeErr))
			fa = &FileAnalysis{}
		}

		// HTTP call detection is skipped for Python test files (test_*.py, *_test.py,
		// or anywhere under a tests/ directory) — test traffic isn't production
		// API usage. This must use rel (the relative path), not the absolute path
		// Analyze() receives, so the check happens here rather than in the adapter.
		if adapter.Name() == "python" && isPythonTestFile(rel) {
			fa.Calls = nil
		}

		analysis.Files = append(analysis.Files, FileInfo{
			ID:           rel,
			Path:         rel,
			Language:     adapter.Name(),
			SizeBytes:    info.Size(),
			Exports:      fa.Exports,
			Imports:      fa.Imports,
			Declarations: fa.Declarations,
			Calls:        fa.Calls,
			ModuleCalls:  fa.ModuleCalls,
			ChainedCalls: fa.ChainedCalls,
			Metadata:     fa.Metadata,
		})
		return nil
	})

	return result, walkErr
}
