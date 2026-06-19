package typescript_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer/parser/typescript"
)

func TestParserParsesValidTypescript(t *testing.T) {
	p := typescript.NewParser()
	tree, err := p.Parse([]byte(`export function foo() {}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if tree == nil {
		t.Fatal("want non-nil tree, got nil")
	}
}

func TestParserParsesValidTSX(t *testing.T) {
	p := typescript.NewParser()
	src := []byte(`export default function App() { return <div>hi</div>; }`)
	tree, err := p.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if tree == nil {
		t.Fatal("want non-nil tree for TSX input, got nil")
	}
}

func TestParserHandlesMalformedInput(t *testing.T) {
	p := typescript.NewParser()
	// tree-sitter is resilient: broken syntax returns a tree with error nodes,
	// not a Go error.
	tree, err := p.Parse([]byte(`function foo( {`))
	if err != nil {
		t.Fatalf("Parse returned unexpected error for malformed input: %v", err)
	}
	if tree == nil {
		t.Fatal("want non-nil tree for malformed input, got nil")
	}
}

func TestRootNodeType(t *testing.T) {
	p := typescript.NewParser()
	tree, err := p.Parse([]byte(`export function foo() {}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	root := tree.RootNode()
	if root == nil {
		t.Fatal("RootNode returned nil")
	}
	if root.Type() == "" {
		t.Error("want non-empty node type, got empty string")
	}
}

func TestNodeText(t *testing.T) {
	src := `export function foo() {}`
	p := typescript.NewParser()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	root := tree.RootNode()
	if got := root.Text(); got != src {
		t.Errorf("root.Text() = %q, want %q", got, src)
	}
}
