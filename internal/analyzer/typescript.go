package analyzer

import (
	"fmt"
	"path/filepath"

	tsparser "github.com/kanukuntla-r/forge/internal/analyzer/parser/typescript"
)

var tsExtensions = []string{".ts", ".tsx", ".js", ".jsx"}

type typescriptAdapter struct {
	parser *tsparser.Parser
}

func newTypescriptAdapter() *typescriptAdapter {
	return &typescriptAdapter{parser: tsparser.NewParser()}
}

func (a *typescriptAdapter) Name() string             { return "typescript" }
func (a *typescriptAdapter) FileExtensions() []string { return tsExtensions }

func (a *typescriptAdapter) Detect(path string, _ []byte) bool {
	ext := filepath.Ext(path)
	for _, e := range tsExtensions {
		if ext == e {
			return true
		}
	}
	return false
}

func (a *typescriptAdapter) Analyze(path string, content []byte) (*FileAnalysis, error) {
	_, err := a.parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", path, err)
	}
	// M8.2a: parse proves tree-sitter works; extraction (imports/exports/declarations) in M8.2b-d.
	return &FileAnalysis{}, nil
}
