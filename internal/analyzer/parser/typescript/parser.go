// Package typescript wraps tree-sitter for TypeScript/TSX/JS/JSX parsing.
// Uses the TSX grammar for all file types since it is a superset.
package typescript

import (
	"context"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
)

// Parser is a reusable parser instance. Construct once via NewParser, call Parse many times.
// tree-sitter parsers are not goroutine-safe; callers must serialize access.
type Parser struct {
	p *sitter.Parser
}

// NewParser returns a Parser initialized with the TSX grammar.
func NewParser() *Parser {
	p := sitter.NewParser()
	p.SetLanguage(tsx.GetLanguage())
	return &Parser{p: p}
}

// Parse parses content and returns a Tree. Tree-sitter is resilient — even malformed
// source returns a usable tree with error nodes rather than returning an error.
// An error here indicates a parser-level failure (e.g. context cancellation,
// uninitialized language), not a syntax error in the source.
func (p *Parser) Parse(content []byte) (*Tree, error) {
	tree, err := p.p.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}
	return &Tree{tree: tree, source: content}, nil
}

// Tree wraps a tree-sitter tree. It holds the original source bytes so that
// Node.Text() can return the corresponding source text without extra parameters.
type Tree struct {
	tree   *sitter.Tree
	source []byte
}

// RootNode returns the root node of the parsed tree.
func (t *Tree) RootNode() *Node {
	return &Node{node: t.tree.RootNode(), source: t.source}
}

// Node wraps a tree-sitter node and its source bytes.
// Future chunks will add Walk and query methods for extraction.
type Node struct {
	node   *sitter.Node
	source []byte
}

// Type returns the node's grammar type (e.g. "program", "import_statement").
func (n *Node) Type() string { return n.node.Type() }

// Text returns the source text corresponding to this node.
func (n *Node) Text() string { return n.node.Content(n.source) }
