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
	tree, err := a.parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", path, err)
	}

	rawImports := tree.Imports()
	imports := make([]Import, len(rawImports))
	for i, imp := range rawImports {
		imports[i] = Import{
			Source:   imp.Source,
			Names:    imp.Names,
			External: imp.External,
			Line:     imp.Line,
			// Resolved stays empty until M8.2e.
		}
	}

	rawExports := tree.Exports()
	exports := make([]Export, len(rawExports))
	for i, exp := range rawExports {
		exports[i] = Export{
			Name: exp.Name,
			Type: exp.Type,
			Line: exp.Line,
		}
	}

	rawDecls := tree.Declarations()
	decls := make([]Declaration, len(rawDecls))
	for i, d := range rawDecls {
		decls[i] = Declaration{
			Name: d.Name,
			Type: d.Type,
			Line: d.Line,
		}
	}

	rawCalls := tree.APICalls()
	calls := make([]Call, len(rawCalls))
	for i, c := range rawCalls {
		calls[i] = Call{
			Target:     c.Target,
			Method:     c.Method,
			Line:       c.Line,
			Kind:       c.Kind,
			Confidence: c.Confidence,
		}
	}

	rawChainedCalls := tree.ChainedCalls()
	chainedCalls := make([]ChainedCall, len(rawChainedCalls))
	for i, c := range rawChainedCalls {
		chainedCalls[i] = ChainedCall{
			Object:   c.Object,
			Property: c.Property,
			Method:   c.Method,
			Line:     c.Line,
		}
	}

	rawMethodChains := tree.MethodChains()
	methodChains := make([]MethodChain, len(rawMethodChains))
	for i, mc := range rawMethodChains {
		segments := make([]MethodChainSegment, len(mc.Calls))
		for j, seg := range mc.Calls {
			segments[j] = MethodChainSegment{Name: seg.Name, Args: seg.Args}
		}
		methodChains[i] = MethodChain{Root: mc.Root, Calls: segments, Line: mc.Line}
	}

	return &FileAnalysis{
		Imports:      imports,
		Exports:      exports,
		Declarations: decls,
		Calls:        calls,
		ChainedCalls: chainedCalls,
		MethodChains: methodChains,
	}, nil
}
