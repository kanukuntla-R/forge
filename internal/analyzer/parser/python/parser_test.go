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
	if tree.RootNode() == nil {
		t.Fatal("root node is nil")
	}
}

func TestParseEmptyFile(t *testing.T) {
	p := python.NewParser()
	_, err := p.Parse([]byte{})
	if err != nil {
		t.Fatalf("parse empty: %v", err)
	}
}

func TestParseInvalidPython(t *testing.T) {
	// tree-sitter is error-tolerant — broken syntax returns a partial tree, not an error.
	p := python.NewParser()
	_, err := p.Parse([]byte("def broken_syntax(\n"))
	if err != nil {
		t.Fatalf("parse broken: %v", err)
	}
}

func TestLanguageReturnsNonNil(t *testing.T) {
	if python.Language() == nil {
		t.Fatal("Language() returned nil")
	}
}
