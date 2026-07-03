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
type Tree struct {
	tree   *sitter.Tree
	source []byte
}

// RootNode returns the root node of the parsed tree as a Node.
func (t *Tree) RootNode() *Node {
	n := t.tree.RootNode()
	if n == nil {
		return nil
	}
	return &Node{node: n, source: t.source}
}

// Node wraps a tree-sitter node and the source bytes so callers can call
// Content() without threading the source slice through every helper function.
type Node struct {
	node   *sitter.Node
	source []byte
}

// Type returns the grammar node type (e.g. "import_statement", "dotted_name").
func (n *Node) Type() string { return n.node.Type() }

// Content returns the source text corresponding to this node.
func (n *Node) Content() string { return n.node.Content(n.source) }

// StartLine returns the 1-indexed line number where this node begins.
func (n *Node) StartLine() int { return int(n.node.StartPoint().Row) + 1 }

// NamedChildCount returns the number of named children.
func (n *Node) NamedChildCount() int { return int(n.node.NamedChildCount()) }

// NamedChild returns the i-th named child, or nil if out of range.
func (n *Node) NamedChild(i int) *Node {
	child := n.node.NamedChild(i)
	if child == nil {
		return nil
	}
	return &Node{node: child, source: n.source}
}

// HasAsyncKeyword reports whether this function_definition node begins with
// the async keyword. In tree-sitter-python, async is an anonymous (unnamed)
// child at position 0 — it has no grammar field name, so positional Child(0)
// is the only way to detect it.
func (n *Node) HasAsyncKeyword() bool {
	if n.node.ChildCount() == 0 {
		return false
	}
	first := n.node.Child(0)
	return first != nil && first.Type() == "async"
}

// Language returns the tree-sitter Language for Python.
// Used by detectors that need to run tree-sitter queries.
func Language() *sitter.Language {
	return python.GetLanguage()
}
