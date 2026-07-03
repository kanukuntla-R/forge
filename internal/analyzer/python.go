package analyzer

import (
	"path/filepath"

	pyparser "github.com/kanukuntla-r/forge/internal/analyzer/parser/python"
)

var pyExtensions = []string{".py"}

type pythonAdapter struct {
	parser *pyparser.Parser
}

func newPythonAdapter() *pythonAdapter {
	return &pythonAdapter{parser: pyparser.NewParser()}
}

// NewPythonAdapter returns an exported handle for use in tests and external callers.
func NewPythonAdapter() LanguageAdapter {
	return newPythonAdapter()
}

func (a *pythonAdapter) Name() string             { return "python" }
func (a *pythonAdapter) FileExtensions() []string { return pyExtensions }

func (a *pythonAdapter) Detect(path string, _ []byte) bool {
	return filepath.Ext(path) == ".py"
}

func (a *pythonAdapter) Analyze(_ string, content []byte) (*FileAnalysis, error) {
	// M10.1: verify parse succeeds. Extraction (imports, declarations, calls) comes in M10.2–M10.3.
	if _, err := a.parser.Parse(content); err != nil {
		// tree-sitter rarely errors on broken syntax (it's resilient).
		// A real error here means something is wrong with the parser setup.
		return &FileAnalysis{}, nil
	}
	return &FileAnalysis{}, nil
}
