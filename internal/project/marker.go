package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Project struct {
	Blueprint         string                 `json:"blueprint"`
	BlueprintVersion  string                 `json:"blueprint_version"`
	ForgeVersion      string                 `json:"forge_version"`
	CreatedAt         string                 `json:"created_at"`
	Variables         map[string]any         `json:"variables"`
	ExtensionsApplied []ExtensionApplication `json:"extensions_applied"`
}

type ExtensionApplication struct {
	Name      string         `json:"name"`
	AppliedAt string         `json:"applied_at"`
	Args      map[string]any `json:"args"`
}

// Write marshals p to indented JSON and writes it to
// <targetPath>/.forge/project.json, creating the .forge directory if needed.
func Write(targetPath string, p Project) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling project marker to JSON: %w", err)
	}

	dir := filepath.Join(targetPath, ".forge")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %q: %w", dir, err)
	}

	dest := filepath.Join(dir, "project.json")
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("writing project.json: %w", err)
	}
	return nil
}
