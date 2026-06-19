package typescript

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// ExportInfo holds the extracted data for a single export.
// It maps 1:1 to analyzer.Export once consumed by the adapter.
type ExportInfo struct {
	Name string // "default", "foo", "*", etc.
	Type string // "function", "class", "interface", "type", "variable", "default",
	//            "aggregated", "reexport", "wildcard_reexport", "namespace_reexport"
	Line int // 1-indexed
}

// Exports walks the program root's named children and returns all top-level
// export_statement nodes as structured ExportInfo values.
func (t *Tree) Exports() []ExportInfo {
	root := t.tree.RootNode()
	var out []ExportInfo
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() == "export_statement" {
			out = append(out, extractExports(child, t.source)...)
		}
	}
	return out
}

// extractExports returns zero or more ExportInfo values from a single export_statement.
// Multiple results occur for lexical exports with multiple declarators
// (e.g. export const a = 1, b = 2) or aggregate exports (export { a, b }).
func extractExports(node *sitter.Node, src []byte) []ExportInfo {
	line := int(node.StartPoint().Row) + 1

	// Default export: any anonymous "default" child signals all default forms
	// (export default function, class, expression). The inner shape doesn't matter.
	if hasAnonChild(node, "default") {
		return []ExportInfo{{Name: "default", Type: "default", Line: line}}
	}

	// Walk named children to classify the export form.
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "function_declaration":
			return []ExportInfo{{
				Name: namedChildContent(child, "identifier", src),
				Type: "function",
				Line: line,
			}}
		case "class_declaration":
			return []ExportInfo{{
				Name: namedChildContent(child, "type_identifier", src),
				Type: "class",
				Line: line,
			}}
		case "interface_declaration":
			return []ExportInfo{{
				Name: namedChildContent(child, "type_identifier", src),
				Type: "interface",
				Line: line,
			}}
		case "type_alias_declaration":
			return []ExportInfo{{
				Name: namedChildContent(child, "type_identifier", src),
				Type: "type",
				Line: line,
			}}
		case "lexical_declaration", "variable_declaration":
			return lexicalExports(child, src, line)
		case "namespace_export":
			// export * as utils from "./x"
			return []ExportInfo{{
				Name: namedChildContent(child, "identifier", src),
				Type: "namespace_reexport",
				Line: line,
			}}
		case "export_clause":
			exportType := "aggregated"
			if exportSource(node, src) != "" {
				exportType = "reexport"
			}
			return clauseExports(child, src, exportType, line)
		}
	}

	// Wildcard re-export: export * from "./x"
	// The * is anonymous, so the only named child of the export_statement is the string.
	if exportSource(node, src) != "" {
		return []ExportInfo{{Name: "*", Type: "wildcard_reexport", Line: line}}
	}
	return nil
}

func lexicalExports(node *sitter.Node, src []byte, line int) []ExportInfo {
	var out []ExportInfo
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "variable_declarator" {
			if name := namedChildContent(child, "identifier", src); name != "" {
				out = append(out, ExportInfo{Name: name, Type: "variable", Line: line})
			}
		}
	}
	return out
}

func clauseExports(clause *sitter.Node, src []byte, exportType string, line int) []ExportInfo {
	var out []ExportInfo
	for i := 0; i < int(clause.NamedChildCount()); i++ {
		child := clause.NamedChild(i)
		if child.Type() == "export_specifier" {
			if name := specifierPublicName(child, src); name != "" {
				out = append(out, ExportInfo{Name: name, Type: exportType, Line: line})
			}
		}
	}
	return out
}

// specifierPublicName returns the public (consumer-facing) name from an export_specifier.
// For `{ foo as renamed }` it returns "renamed"; for `{ foo }` it returns "foo".
// This is the opposite of specifierName (in imports.go) which returns the source name.
func specifierPublicName(specifier *sitter.Node, src []byte) string {
	var last string
	for i := 0; i < int(specifier.NamedChildCount()); i++ {
		child := specifier.NamedChild(i)
		if child.Type() == "identifier" {
			last = child.Content(src)
		}
	}
	return last
}

// namedChildContent finds the first named child of the given type and returns its content.
func namedChildContent(node *sitter.Node, childType string, src []byte) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == childType {
			return child.Content(src)
		}
	}
	return ""
}

// exportSource finds the string named child of an export_statement and returns the
// string_fragment content (module path without quotes).
func exportSource(node *sitter.Node, src []byte) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "string" {
			return namedChildContent(child, "string_fragment", src)
		}
	}
	return ""
}

// hasAnonChild returns true if node has an anonymous child with the given type string.
func hasAnonChild(node *sitter.Node, typ string) bool {
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(i)
		if !c.IsNamed() && c.Type() == typ {
			return true
		}
	}
	return false
}
