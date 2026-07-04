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

func TestPythonAdapter_ImportsFlowThrough(t *testing.T) {
	a := analyzer.NewPythonAdapter()
	src := "import os\nfrom fastapi import FastAPI\nfrom .config import settings\n"
	result, err := a.Analyze("app.py", []byte(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Imports) != 3 {
		t.Fatalf("want 3 imports, got %d: %+v", len(result.Imports), result.Imports)
	}

	// import os
	if result.Imports[0].Source != "os" {
		t.Errorf("import 0 source: want %q, got %q", "os", result.Imports[0].Source)
	}
	if !result.Imports[0].External {
		t.Errorf("import 0: want external=true")
	}

	// from fastapi import FastAPI
	if result.Imports[1].Source != "fastapi" {
		t.Errorf("import 1 source: want %q, got %q", "fastapi", result.Imports[1].Source)
	}
	if len(result.Imports[1].Names) != 1 || result.Imports[1].Names[0] != "FastAPI" {
		t.Errorf("import 1 names: want [FastAPI], got %v", result.Imports[1].Names)
	}

	// from .config import settings
	if result.Imports[2].Source != ".config" {
		t.Errorf("import 2 source: want %q, got %q", ".config", result.Imports[2].Source)
	}
	if !result.Imports[2].IsRelative {
		t.Errorf("import 2: want is_relative=true")
	}
	if result.Imports[2].External {
		t.Errorf("import 2: want external=false")
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

func TestPythonAdapter_DeclarationsFlowThrough(t *testing.T) {
	a := analyzer.NewPythonAdapter()
	src := "app = FastAPI()\n\n@app.get(\"/health\")\ndef health():\n    return {}\n"
	result, err := a.Analyze("app.py", []byte(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Declarations) != 2 {
		t.Fatalf("want 2 declarations, got %d: %+v", len(result.Declarations), result.Declarations)
	}
	if result.Declarations[0].Type != "variable" || result.Declarations[0].Name != "app" {
		t.Errorf("decl 0: want variable app, got %s %s", result.Declarations[0].Type, result.Declarations[0].Name)
	}
	if result.Declarations[1].Type != "function" || result.Declarations[1].Name != "health" {
		t.Errorf("decl 1: want function health, got %s %s", result.Declarations[1].Type, result.Declarations[1].Name)
	}
	if len(result.Declarations[1].Decorators) != 1 {
		t.Errorf("health.decorators: want 1, got %v", result.Declarations[1].Decorators)
	}
}

func TestPythonAdapter_ModuleCallsFlowThrough(t *testing.T) {
	a := analyzer.NewPythonAdapter()
	src := "app.include_router(users.router)\napp.include_router(auth_router, prefix=\"/auth\")\n"
	result, err := a.Analyze("main.py", []byte(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.ModuleCalls) != 2 {
		t.Fatalf("want 2 module calls, got %d: %+v", len(result.ModuleCalls), result.ModuleCalls)
	}
	if result.ModuleCalls[0].FunctionRepr != "app.include_router" {
		t.Errorf("call 0 function: want %q, got %q", "app.include_router", result.ModuleCalls[0].FunctionRepr)
	}
	if result.ModuleCalls[0].ArgsRepr != "users.router" {
		t.Errorf("call 0 args: want %q, got %q", "users.router", result.ModuleCalls[0].ArgsRepr)
	}
	if result.ModuleCalls[1].ArgsRepr != `auth_router, prefix="/auth"` {
		t.Errorf("call 1 args: want %q, got %q", `auth_router, prefix="/auth"`, result.ModuleCalls[1].ArgsRepr)
	}
}

func TestWalkPythonImportsPopulated(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.py", "import os\nfrom fastapi import FastAPI\nfrom .config import settings\n")

	reg := analyzer.NewRegistry()
	result, err := analyzer.Walk(dir, reg)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(result.Analysis.Files) != 1 {
		t.Fatalf("want 1 file, got %d", len(result.Analysis.Files))
	}
	f := result.Analysis.Files[0]
	if f.Language != "python" {
		t.Fatalf("want language=python, got %q", f.Language)
	}
	if len(f.Imports) != 3 {
		t.Fatalf("want 3 imports in walk result, got %d: %+v", len(f.Imports), f.Imports)
	}
	if f.Imports[2].IsRelative != true {
		t.Errorf("third import: want is_relative=true")
	}
}
