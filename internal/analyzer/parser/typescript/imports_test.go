package typescript_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer/parser/typescript"
)

func newTestParser(t *testing.T) *typescript.Parser {
	t.Helper()
	return typescript.NewParser()
}

func parseImports(t *testing.T, src string) []typescript.ImportInfo {
	t.Helper()
	p := newTestParser(t)
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return tree.Imports()
}

func TestImportsNamed(t *testing.T) {
	imps := parseImports(t, `import { foo, bar } from "./utils"`)
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	imp := imps[0]
	if imp.Source != "./utils" {
		t.Errorf("source: want %q, got %q", "./utils", imp.Source)
	}
	if !sliceEqual(imp.Names, []string{"foo", "bar"}) {
		t.Errorf("names: want [foo bar], got %v", imp.Names)
	}
	if imp.External {
		t.Errorf("external: want false, got true")
	}
}

func TestImportsDefault(t *testing.T) {
	imps := parseImports(t, `import React from "react"`)
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	imp := imps[0]
	if imp.Source != "react" {
		t.Errorf("source: want %q, got %q", "react", imp.Source)
	}
	if !sliceEqual(imp.Names, []string{"default"}) {
		t.Errorf("names: want [default], got %v", imp.Names)
	}
	if !imp.External {
		t.Errorf("external: want true, got false")
	}
}

func TestImportsDefaultAndNamed(t *testing.T) {
	imps := parseImports(t, `import React, { useState, useEffect } from "react"`)
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	if !sliceEqual(imps[0].Names, []string{"default", "useState", "useEffect"}) {
		t.Errorf("names: want [default useState useEffect], got %v", imps[0].Names)
	}
}

func TestImportsNamespace(t *testing.T) {
	imps := parseImports(t, `import * as utils from "./utils"`)
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	if !sliceEqual(imps[0].Names, []string{"*"}) {
		t.Errorf("names: want [*], got %v", imps[0].Names)
	}
}

func TestImportsSideEffect(t *testing.T) {
	imps := parseImports(t, `import "./styles.css"`)
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	imp := imps[0]
	if imp.Source != "./styles.css" {
		t.Errorf("source: want %q, got %q", "./styles.css", imp.Source)
	}
	if imp.Names == nil {
		t.Error("Names must not be nil for side-effect import (want empty slice)")
	}
	if len(imp.Names) != 0 {
		t.Errorf("names: want [], got %v", imp.Names)
	}
}

func TestImportsTypeOnly(t *testing.T) {
	imps := parseImports(t, `import type { User } from "./types"`)
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	imp := imps[0]
	if imp.Source != "./types" {
		t.Errorf("source: want %q, got %q", "./types", imp.Source)
	}
	if !sliceEqual(imp.Names, []string{"User"}) {
		t.Errorf("names: want [User], got %v", imp.Names)
	}
	if imp.External {
		t.Errorf("external: want false, got true")
	}
}

func TestImportsRenamed(t *testing.T) {
	imps := parseImports(t, `import { foo as bar } from "./utils"`)
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	// We record the original name (foo), not the alias (bar).
	if !sliceEqual(imps[0].Names, []string{"foo"}) {
		t.Errorf("names: want [foo], got %v", imps[0].Names)
	}
}

func TestImportsMixedTypeAndValue(t *testing.T) {
	imps := parseImports(t, `import { type Props, helper } from "./utils"`)
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	if !sliceEqual(imps[0].Names, []string{"Props", "helper"}) {
		t.Errorf("names: want [Props helper], got %v", imps[0].Names)
	}
}

func TestImportsExternalDetection(t *testing.T) {
	src := `import { foo } from "./utils"
import React from "react"
import { x } from "@supabase/supabase-js"
import { z } from "@/components/Z"
import "../shared/styles"`

	imps := parseImports(t, src)
	if len(imps) != 5 {
		t.Fatalf("want 5 imports, got %d", len(imps))
	}

	cases := []struct {
		source   string
		external bool
	}{
		{"./utils", false},
		{"react", true},
		{"@supabase/supabase-js", true},
		{"@/components/Z", false},
		{"../shared/styles", false},
	}
	for i, c := range cases {
		if imps[i].Source != c.source {
			t.Errorf("import %d: source want %q, got %q", i, c.source, imps[i].Source)
		}
		if imps[i].External != c.external {
			t.Errorf("import %d (%s): external want %v, got %v", i, c.source, c.external, imps[i].External)
		}
	}
}

func TestImportsSingleQuotes(t *testing.T) {
	imps := parseImports(t, `import { foo } from './utils'`)
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	if imps[0].Source != "./utils" {
		t.Errorf("source: want %q, got %q", "./utils", imps[0].Source)
	}
}

func TestImportsLineNumbers(t *testing.T) {
	src := `import { a } from "./a"
import { b } from "./b"
import { c } from "./c"`
	imps := parseImports(t, src)
	if len(imps) != 3 {
		t.Fatalf("want 3 imports, got %d", len(imps))
	}
	for i, imp := range imps {
		want := i + 1
		if imp.Line != want {
			t.Errorf("import %d: line want %d, got %d", i, want, imp.Line)
		}
	}
}

func TestImportsDynamicSkipped(t *testing.T) {
	// Dynamic import() is a call_expression, not an import_statement.
	imps := parseImports(t, `const x = await import("./utils")`)
	if len(imps) != 0 {
		t.Errorf("want 0 imports for dynamic import(), got %d: %v", len(imps), imps)
	}
}

func TestImportsCommentedOutSkipped(t *testing.T) {
	src := `// import { foo } from "./utils"
import { bar } from "./real"`
	imps := parseImports(t, src)
	if len(imps) != 1 {
		t.Fatalf("want 1 import, got %d", len(imps))
	}
	if imps[0].Source != "./real" {
		t.Errorf("source: want %q, got %q", "./real", imps[0].Source)
	}
}

func TestImportsMalformedReturnsBest(t *testing.T) {
	// Broken syntax: missing closing brace. tree-sitter recovers; we shouldn't panic.
	imps := parseImports(t, `import { foo from "./utils"`)
	// Don't assert specific output — just no panic.
	_ = imps
}

// sliceEqual returns true if a and b have the same elements in the same order.
func sliceEqual(a, b []string) bool {
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
