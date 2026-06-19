package typescript

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// ImportInfo holds the extracted data for a single import statement.
// It maps 1:1 to analyzer.Import once consumed by the adapter.
type ImportInfo struct {
	Source   string
	Names    []string // never nil: empty slice for side-effect imports
	External bool
	Line     int // 1-indexed
}

// Imports walks the program root's named children and returns all top-level
// import_statement nodes as structured ImportInfo values.
// Re-export statements (export { ... } from "...") are also included because
// they introduce a dependency on the source module.
// Dynamic imports (import(...)) are call expressions in the AST — they are
// not import_statement nodes and are skipped automatically.
func (t *Tree) Imports() []ImportInfo {
	root := t.tree.RootNode()
	var out []ImportInfo
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		switch child.Type() {
		case "import_statement":
			if imp, ok := extractImport(child, t.source); ok {
				out = append(out, imp)
			}
		case "export_statement":
			if imp, ok := extractReexportImport(child, t.source); ok {
				out = append(out, imp)
			}
		}
	}
	return out
}

// extractReexportImport builds an ImportInfo for export_statement nodes that
// have a source string (e.g. export { foo } from "./x", export * from "./x").
// Returns false when the export has no source (not a re-export).
func extractReexportImport(node *sitter.Node, src []byte) (ImportInfo, bool) {
	source := exportSource(node, src) // defined in exports.go, same package
	if source == "" {
		return ImportInfo{}, false
	}

	var names []string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "export_clause":
			for j := 0; j < int(child.NamedChildCount()); j++ {
				spec := child.NamedChild(j)
				if spec.Type() == "export_specifier" {
					// Import side: record the source name (first identifier).
					if name := specifierName(spec, src); name != "" {
						names = append(names, name)
					}
				}
			}
		case "namespace_export":
			names = []string{"*"}
		}
	}
	if names == nil {
		names = []string{"*"} // wildcard re-export: export * from "./x"
	}

	return ImportInfo{
		Source:   source,
		Names:    names,
		External: isExternal(source),
		Line:     int(node.StartPoint().Row) + 1,
	}, true
}

// extractImport builds an ImportInfo from a single import_statement node.
func extractImport(node *sitter.Node, src []byte) (ImportInfo, bool) {
	source := importSource(node, src)
	if source == "" {
		return ImportInfo{}, false
	}
	names := importNames(node, src)
	if names == nil {
		names = []string{} // side-effect import: explicit empty slice, never nil
	}
	return ImportInfo{
		Source:   source,
		Names:    names,
		External: isExternal(source),
		Line:     int(node.StartPoint().Row) + 1,
	}, true
}

// importSource finds the string named child and returns the string_fragment
// (the module path without surrounding quotes).
func importSource(node *sitter.Node, src []byte) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "string" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				frag := child.NamedChild(j)
				if frag.Type() == "string_fragment" {
					return frag.Content(src)
				}
			}
		}
	}
	return ""
}

// importNames finds the import_clause named child and extracts the imported
// name list. Returns nil when there is no clause (side-effect import).
func importNames(node *sitter.Node, src []byte) []string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "import_clause" {
			return clauseNames(child, src)
		}
	}
	return nil
}

func clauseNames(clause *sitter.Node, src []byte) []string {
	var names []string
	for i := 0; i < int(clause.NamedChildCount()); i++ {
		child := clause.NamedChild(i)
		switch child.Type() {
		case "identifier":
			// Default import (e.g. `import React from "react"`).
			names = append(names, "default")
		case "named_imports":
			names = append(names, namedImportNames(child, src)...)
		case "namespace_import":
			// Namespace import (e.g. `import * as utils from "./utils"`).
			names = append(names, "*")
		}
	}
	return names
}

func namedImportNames(node *sitter.Node, src []byte) []string {
	var names []string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "import_specifier" {
			if name := specifierName(child, src); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// specifierName returns the original (pre-alias) name from an import_specifier.
//
// For `{ foo as bar }`, tree-sitter emits two identifier named children:
// the first is the original name, the second is the alias. We always take the first.
//
// The `type` keyword in `{ type Props }` is an anonymous node and invisible to
// NamedChild — so mixed type+value specifiers need no special handling here.
func specifierName(specifier *sitter.Node, src []byte) string {
	for i := 0; i < int(specifier.NamedChildCount()); i++ {
		child := specifier.NamedChild(i)
		if child.Type() == "identifier" {
			return child.Content(src)
		}
	}
	return ""
}

// isExternal returns true when source does not look like a local path.
// This is a heuristic for M8.2b. M8.2e will refine it using tsconfig.json aliases.
func isExternal(source string) bool {
	for _, prefix := range []string{"./", "../", "/", "~/", "@/"} {
		if strings.HasPrefix(source, prefix) {
			return false
		}
	}
	return true
}
