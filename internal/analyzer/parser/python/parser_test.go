package python_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer/parser/python"
)

func TestParseValidPython(t *testing.T) {
	p := python.NewParser()
	tree, err := p.Parse([]byte("def hello():\n    return 'world'\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	root := tree.RootNode()
	if root == nil {
		t.Fatal("root node is nil")
	}
	if root.NamedChildCount() == 0 {
		t.Fatal("expected at least one named child in root")
	}
}

func TestParseEmptyFile(t *testing.T) {
	p := python.NewParser()
	tree, err := p.Parse([]byte{})
	if err != nil {
		t.Fatalf("parse empty: %v", err)
	}
	if tree.RootNode() == nil {
		t.Fatal("root node is nil for empty file")
	}
}

func TestParseInvalidPython(t *testing.T) {
	// tree-sitter is error-tolerant — broken syntax returns a partial tree, not an error.
	p := python.NewParser()
	tree, err := p.Parse([]byte("def broken_syntax(\n"))
	if err != nil {
		t.Fatalf("parse broken: %v", err)
	}
	if tree.RootNode() == nil {
		t.Fatal("root node is nil for broken syntax")
	}
}

func TestNodeContent(t *testing.T) {
	p := python.NewParser()
	src := "x = 1\n"
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	root := tree.RootNode()
	if root == nil {
		t.Fatal("root node is nil")
	}
	// Root node spans the entire file.
	if got := root.Content(); got != src {
		t.Errorf("root Content() = %q, want %q", got, src)
	}
}

func TestNodeType(t *testing.T) {
	p := python.NewParser()
	tree, err := p.Parse([]byte("import os\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	root := tree.RootNode()
	if root.Type() != "module" {
		t.Errorf("root Type() = %q, want module", root.Type())
	}
	if root.NamedChildCount() == 0 {
		t.Fatal("expected import_statement child")
	}
	child := root.NamedChild(0)
	if child.Type() != "import_statement" {
		t.Errorf("child Type() = %q, want import_statement", child.Type())
	}
}

func TestNodeStartLine(t *testing.T) {
	p := python.NewParser()
	tree, err := p.Parse([]byte("x = 1\nimport os\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	root := tree.RootNode()
	if root.NamedChildCount() < 2 {
		t.Fatal("expected 2 children")
	}
	// Second child (import os) starts on line 2.
	imp := root.NamedChild(1)
	if got := imp.StartLine(); got != 2 {
		t.Errorf("StartLine() = %d, want 2", got)
	}
}

func TestLanguageReturnsNonNil(t *testing.T) {
	if python.Language() == nil {
		t.Fatal("Language() returned nil")
	}
}
