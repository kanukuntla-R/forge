package analyzer_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

func TestPythonAdapter_Name(t *testing.T) {
	a := analyzer.NewPythonAdapter()
	if a.Name() != "python" {
		t.Errorf("Name() = %q, want python", a.Name())
	}
}

func TestPythonAdapter_Detect(t *testing.T) {
	a := analyzer.NewPythonAdapter()
	cases := []struct {
		path string
		want bool
	}{
		{"main.py", true},
		{"src/app.py", true},
		{"test.pyi", false},
		{"index.ts", false},
		{"main.go", false},
		{"README.md", false},
	}
	for _, c := range cases {
		if got := a.Detect(c.path, nil); got != c.want {
			t.Errorf("Detect(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestPythonAdapter_AnalyzeValidPython(t *testing.T) {
	a := analyzer.NewPythonAdapter()
	result, err := a.Analyze("hello.py", []byte("def hello():\n    return 'world'\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestPythonAdapter_AnalyzeBrokenSyntax(t *testing.T) {
	a := analyzer.NewPythonAdapter()
	result, err := a.Analyze("broken.py", []byte("def missing_paren(\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil; want non-nil FileAnalysis for broken syntax")
	}
}

func TestWalkUsesPythonAdapter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.py", "def hello():\n    return 'world'\n")

	reg := analyzer.NewRegistry()
	result, err := analyzer.Walk(dir, reg)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if got := len(result.Analysis.Files); got != 1 {
		t.Fatalf("want 1 file, got %d", got)
	}
	if got := result.Analysis.Files[0].Language; got != "python" {
		t.Errorf("app.py: want language=python, got %q", got)
	}
}
