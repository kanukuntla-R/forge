package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// DetectionResult is what Detect found.
type DetectionResult struct {
	Project Project // parsed contents of project.json
	Root    string  // absolute path of the directory containing .forge/
}

// Detect walks up from start (absolute or relative path) looking for
// .forge/project.json. Returns the result if found, nil if not found
// (no error in that case — absence is a normal answer).
//
// Walking stops at a .git directory boundary: if .git/ is found at any
// level before .forge/, the walk halts and Detect returns nil. This
// prevents picking up stray .forge/ markers above a project's git root.
//
// Errors are returned only for malformed project.json files or
// filesystem walk failures.
func Detect(start string) (*DetectionResult, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return nil, fmt.Errorf("resolving path %q: %w", start, err)
	}

	cur := abs
	for {
		// Check for .forge/project.json at this level.
		markerPath := filepath.Join(cur, ".forge", "project.json")
		data, err := os.ReadFile(markerPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("reading %q: %w", markerPath, err)
		}
		if err == nil {
			var p Project
			if err := json.Unmarshal(data, &p); err != nil {
				return nil, fmt.Errorf("parsing %q: %w", markerPath, err)
			}
			return &DetectionResult{Project: p, Root: cur}, nil
		}

		// Check for .git boundary before walking up.
		_, gitErr := os.Stat(filepath.Join(cur, ".git"))
		if gitErr != nil && !errors.Is(gitErr, fs.ErrNotExist) {
			return nil, fmt.Errorf("checking .git in %q: %w", cur, gitErr)
		}
		if gitErr == nil {
			return nil, nil
		}

		// Walk up; stop at filesystem root.
		parent := filepath.Dir(cur)
		if parent == cur {
			return nil, nil
		}
		cur = parent
	}
}
