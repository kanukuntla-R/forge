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
	tree, err := a.parser.Parse(content)
	if err != nil {
		return &FileAnalysis{}, nil
	}

	rawImports := tree.Imports()
	imports := make([]Import, len(rawImports))
	for i, imp := range rawImports {
		imports[i] = Import{
			Source:     imp.Source,
			Names:      imp.Names,
			Alias:      imp.Alias,
			IsRelative: imp.IsRelative,
			IsStar:     imp.IsStar,
			External:   imp.External,
			Line:       imp.Line,
		}
	}

	rawDecls := tree.Declarations()
	decls := make([]Declaration, len(rawDecls))
	for i, d := range rawDecls {
		decls[i] = Declaration{
			Name:       d.Name,
			Type:       d.Kind,
			Line:       d.Line,
			Decorators: d.Decorators,
			Bases:      d.Bases,
			ValueRepr:  d.ValueRepr,
		}
	}

	return &FileAnalysis{
		Imports:      imports,
		Exports:      []Export{},
		Declarations: decls,
		Calls:        []Call{},
	}, nil
}
