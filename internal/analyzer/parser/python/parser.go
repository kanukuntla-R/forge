// Package python wraps tree-sitter for Python parsing.
package python

import (
	"context"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

// Parser is a reusable parser instance. tree-sitter parsers are not goroutine-safe;
// callers must serialize access or construct one Parser per goroutine.
type Parser struct {
	p *sitter.Parser
}

// NewParser returns a Parser initialized with the Python grammar.
func NewParser() *Parser {
	p := sitter.NewParser()
	p.SetLanguage(python.GetLanguage())
	return &Parser{p: p}
}

// Parse parses Python source and returns a Tree. tree-sitter is resilient — malformed
// source returns a partial tree with error nodes rather than an error.
func (p *Parser) Parse(content []byte) (*Tree, error) {
	tree, err := p.p.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}
	return &Tree{tree: tree, source: content}, nil
}

// Tree wraps a tree-sitter tree and the source bytes.
// Future chunks will add query methods for import and declaration extraction.
type Tree struct {
	tree   *sitter.Tree
	source []byte
}

// RootNode returns the root node of the parsed tree.
func (t *Tree) RootNode() *sitter.Node {
	return t.tree.RootNode()
}

// Language returns the tree-sitter Language for Python.
// Used by detectors that need to run tree-sitter queries.
func Language() *sitter.Language {
	return python.GetLanguage()
}
