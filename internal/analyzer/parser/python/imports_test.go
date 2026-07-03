package python_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer/parser/python"
)

func parseImports(t *testing.T, src string) []python.ImportInfo {
	t.Helper()
	p := python.NewParser()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return tree.Imports()
}

func TestExtractSingleModule(t *testing.T) {
	imps := parseImports(t, "import os\n")
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	imp := imps[0]
	if imp.Source != "os" {
		t.Errorf("source: want %q, got %q", "os", imp.Source)
	}
	if !imp.External {
		t.Errorf("external: want true")
	}
	if imp.IsRelative {
		t.Errorf("is_relative: want false")
	}
	if imp.IsStar {
		t.Errorf("is_star: want false")
	}
	if imp.Line != 1 {
		t.Errorf("line: want 1, got %d", imp.Line)
	}
}

func TestExtractAliasedModule(t *testing.T) {
	imps := parseImports(t, "import numpy as np\n")
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	imp := imps[0]
	if imp.Source != "numpy" {
		t.Errorf("source: want %q, got %q", "numpy", imp.Source)
	}
	if imp.Alias != "np" {
		t.Errorf("alias: want %q, got %q", "np", imp.Alias)
	}
	if !imp.External {
		t.Errorf("external: want true")
	}
	if len(imp.Names) != 0 {
		t.Errorf("names: want [], got %v", imp.Names)
	}
}

func TestExtractNestedModule(t *testing.T) {
	imps := parseImports(t, "import os.path\n")
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	if imps[0].Source != "os.path" {
		t.Errorf("source: want %q, got %q", "os.path", imps[0].Source)
	}
	if !imps[0].External {
		t.Errorf("external: want true")
	}
}

func TestExtractCommaImport(t *testing.T) {
	imps := parseImports(t, "import os, sys\n")
	if len(imps) != 2 {
		t.Fatalf("want 2 imports, got %d", len(imps))
	}
	if imps[0].Source != "os" {
		t.Errorf("import 0 source: want %q, got %q", "os", imps[0].Source)
	}
	if imps[1].Source != "sys" {
		t.Errorf("import 1 source: want %q, got %q", "sys", imps[1].Source)
	}
	if imps[0].Line != imps[1].Line {
		t.Errorf("both imports should be on the same line")
	}
}

func TestExtractFromImport(t *testing.T) {
	imps := parseImports(t, "from typing import List\n")
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	imp := imps[0]
	if imp.Source != "typing" {
		t.Errorf("source: want %q, got %q", "typing", imp.Source)
	}
	if !strSliceEqual(imp.Names, []string{"List"}) {
		t.Errorf("names: want [List], got %v", imp.Names)
	}
	if imp.External != true {
		t.Errorf("external: want true")
	}
}

func TestExtractFromImportMulti(t *testing.T) {
	imps := parseImports(t, "from typing import List, Dict\n")
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	if !strSliceEqual(imps[0].Names, []string{"List", "Dict"}) {
		t.Errorf("names: want [List Dict], got %v", imps[0].Names)
	}
}

func TestExtractFromImportParenthesized(t *testing.T) {
	src := "from typing import (\n    List,\n    Dict,\n    Optional,\n)\n"
	imps := parseImports(t, src)
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	if !strSliceEqual(imps[0].Names, []string{"List", "Dict", "Optional"}) {
		t.Errorf("names: want [List Dict Optional], got %v", imps[0].Names)
	}
}

func TestExtractStarImport(t *testing.T) {
	imps := parseImports(t, "from os import *\n")
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	imp := imps[0]
	if imp.Source != "os" {
		t.Errorf("source: want %q, got %q", "os", imp.Source)
	}
	if !imp.IsStar {
		t.Errorf("is_star: want true")
	}
	if len(imp.Names) != 0 {
		t.Errorf("names: want [] for star import, got %v", imp.Names)
	}
	if !imp.External {
		t.Errorf("external: want true")
	}
}

func TestExtractRelativeImport(t *testing.T) {
	imps := parseImports(t, "from . import utils\n")
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	imp := imps[0]
	if imp.Source != "." {
		t.Errorf("source: want %q, got %q", ".", imp.Source)
	}
	if !imp.IsRelative {
		t.Errorf("is_relative: want true")
	}
	if imp.External {
		t.Errorf("external: want false for relative import")
	}
	if !strSliceEqual(imp.Names, []string{"utils"}) {
		t.Errorf("names: want [utils], got %v", imp.Names)
	}
}

func TestExtractRelativeImportWithSubmodule(t *testing.T) {
	imps := parseImports(t, "from .config import settings\n")
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	imp := imps[0]
	if imp.Source != ".config" {
		t.Errorf("source: want %q, got %q", ".config", imp.Source)
	}
	if !imp.IsRelative {
		t.Errorf("is_relative: want true")
	}
	if !strSliceEqual(imp.Names, []string{"settings"}) {
		t.Errorf("names: want [settings], got %v", imp.Names)
	}
}

func TestExtractParentRelativeImport(t *testing.T) {
	imps := parseImports(t, "from ..models import User\n")
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	imp := imps[0]
	if imp.Source != "..models" {
		t.Errorf("source: want %q, got %q", "..models", imp.Source)
	}
	if !imp.IsRelative {
		t.Errorf("is_relative: want true")
	}
	if !strSliceEqual(imp.Names, []string{"User"}) {
		t.Errorf("names: want [User], got %v", imp.Names)
	}
}

func TestExtractMultipleImports(t *testing.T) {
	src := "import os\nfrom typing import List\nfrom fastapi import FastAPI\n"
	imps := parseImports(t, src)
	if len(imps) != 3 {
		t.Fatalf("want 3 imports, got %d: %+v", len(imps), imps)
	}
	// Order preserved, line numbers correct.
	if imps[0].Source != "os" || imps[0].Line != 1 {
		t.Errorf("import 0: want {os, line 1}, got {%s, line %d}", imps[0].Source, imps[0].Line)
	}
	if imps[1].Source != "typing" || imps[1].Line != 2 {
		t.Errorf("import 1: want {typing, line 2}, got {%s, line %d}", imps[1].Source, imps[1].Line)
	}
	if imps[2].Source != "fastapi" || imps[2].Line != 3 {
		t.Errorf("import 2: want {fastapi, line 3}, got {%s, line %d}", imps[2].Source, imps[2].Line)
	}
}

func TestExtractThirdParty(t *testing.T) {
	imps := parseImports(t, "from fastapi import FastAPI\n")
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	if !imps[0].External {
		t.Errorf("external: want true for fastapi (not in stdlib, not relative)")
	}
	if imps[0].Source != "fastapi" {
		t.Errorf("source: want %q, got %q", "fastapi", imps[0].Source)
	}
}

func TestNoImports(t *testing.T) {
	imps := parseImports(t, "x = 1\ndef hello():\n    return x\n")
	if len(imps) != 0 {
		t.Errorf("want 0 imports, got %d: %+v", len(imps), imps)
	}
}

func TestImportWithComments(t *testing.T) {
	src := "# stdlib\nimport os\n# third-party\nfrom fastapi import FastAPI\n"
	imps := parseImports(t, src)
	if len(imps) != 2 {
		t.Fatalf("want 2 imports (comments skipped), got %d: %+v", len(imps), imps)
	}
	if imps[0].Source != "os" {
		t.Errorf("import 0 source: want %q, got %q", "os", imps[0].Source)
	}
	if imps[0].Line != 2 {
		t.Errorf("import 0 line: want 2, got %d", imps[0].Line)
	}
	if imps[1].Source != "fastapi" {
		t.Errorf("import 1 source: want %q, got %q", "fastapi", imps[1].Source)
	}
	if imps[1].Line != 4 {
		t.Errorf("import 1 line: want 4, got %d", imps[1].Line)
	}
}

// strSliceEqual returns true if a and b have the same elements in the same order.
func strSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
