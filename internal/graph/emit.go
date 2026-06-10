package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/kanukuntla-r/forge/internal/blueprint"
	"github.com/kanukuntla-r/forge/internal/render"
)

// Render loads graph.yaml from bp.FS, runs it through the forge template engine
// with ctx, then parses the result as YAML into a *Graph. Returns nil, nil if
// graph.yaml is absent — graph emission is optional per blueprint.
func Render(bp *blueprint.Blueprint, ctx map[string]any) (*Graph, error) {
	src, err := fs.ReadFile(bp.FS, "graph.yaml")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading graph.yaml: %w", err)
	}

	rendered, err := render.Render("graph.yaml", string(src), ctx)
	if err != nil {
		return nil, fmt.Errorf("rendering graph.yaml: %w", err)
	}

	var g Graph
	if err := yaml.Unmarshal([]byte(rendered), &g); err != nil {
		return nil, fmt.Errorf("parsing rendered graph.yaml: %w", err)
	}
	return &g, nil
}

// Write marshals g to indented JSON and writes it to
// <targetPath>/.understand-anything/knowledge-graph.json,
// creating the directory if it doesn't exist.
func Write(g *Graph, targetPath string) error {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling graph to JSON: %w", err)
	}

	dir := filepath.Join(targetPath, ".understand-anything")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %q: %w", dir, err)
	}

	dest := filepath.Join(dir, "knowledge-graph.json")
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("writing knowledge-graph.json: %w", err)
	}
	return nil
}
