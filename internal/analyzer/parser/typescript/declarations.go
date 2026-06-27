package typescript

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// DeclarationInfo holds the extracted data for a single top-level declaration.
// It maps 1:1 to analyzer.Declaration once consumed by the adapter.
type DeclarationInfo struct {
	Name string
	Type string // "function", "component", "class", "interface", "type", "variable"
	Line int    // 1-indexed
}

// Declarations walks the program root's named children and returns all
// top-level declaration nodes as structured DeclarationInfo values.
// Nested declarations (functions inside functions, class methods) are not
// counted — only root-level nodes are visited.
func (t *Tree) Declarations() []DeclarationInfo {
	root := t.tree.RootNode()
	var out []DeclarationInfo
	for i := 0; i < int(root.NamedChildCount()); i++ {
		out = append(out, extractDeclarations(root.NamedChild(i), t.source)...)
	}
	return out
}

// extractDeclarations returns zero or more DeclarationInfo values from a node.
// It handles declaration node types directly and recurses one level into
// export_statement wrappers to capture exported declarations.
func extractDeclarations(node *sitter.Node, src []byte) []DeclarationInfo {
	line := int(node.StartPoint().Row) + 1
	switch node.Type() {
	case "function_declaration":
		name := namedChildContent(node, "identifier", src)
		if name == "" {
			return nil
		}
		return []DeclarationInfo{{Name: name, Type: funcType(name), Line: line}}

	case "class_declaration":
		name := namedChildContent(node, "type_identifier", src)
		if name == "" {
			return nil
		}
		return []DeclarationInfo{{Name: name, Type: "class", Line: line}}

	case "interface_declaration":
		name := namedChildContent(node, "type_identifier", src)
		if name == "" {
			return nil
		}
		return []DeclarationInfo{{Name: name, Type: "interface", Line: line}}

	case "type_alias_declaration":
		name := namedChildContent(node, "type_identifier", src)
		if name == "" {
			return nil
		}
		return []DeclarationInfo{{Name: name, Type: "type", Line: line}}

	case "lexical_declaration", "variable_declaration":
		return varDeclarations(node, src, line)

	case "export_statement":
		return exportDeclarations(node, src)
	}
	return nil
}

// exportDeclarations recurses into the named children of an export_statement
// and returns declarations produced by any declaration-type child.
//
// Handles: export function, export class, export const, export default function foo().
// Returns nil for: re-exports (export { ... } from "..."), anonymous default exports
// (export default function() {}), and expression default exports (export default 42).
//
// Note: export default function foo() {} produces {Name:"foo", Type:"function"}
// because the function_declaration node is a named child and has an identifier.
func exportDeclarations(node *sitter.Node, src []byte) []DeclarationInfo {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		if decls := extractDeclarations(node.NamedChild(i), src); len(decls) > 0 {
			return decls
		}
	}
	return nil
}

// varDeclarations extracts declarations from lexical_declaration or
// variable_declaration nodes (const/let/var). Each variable_declarator child
// becomes one declaration. Arrow functions and known React wrapper calls
// assigned to PascalCase names are classified as components.
func varDeclarations(node *sitter.Node, src []byte, line int) []DeclarationInfo {
	var out []DeclarationInfo
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() != "variable_declarator" {
			continue
		}
		name := namedChildContent(child, "identifier", src)
		if name == "" {
			continue
		}
		typ := "variable"
		if isComponent(name) {
			val := declaratorValue(child)
			if val != nil && (val.Type() == "arrow_function" || isComponentCallExpression(val, src)) {
				typ = "component"
			}
		}
		out = append(out, DeclarationInfo{Name: name, Type: typ, Line: line})
	}
	return out
}

// declaratorValue returns the value node (child 1) of a variable_declarator, or nil.
// Named children are [identifier, value]; the = sign is anonymous and invisible to NamedChild.
func declaratorValue(declarator *sitter.Node) *sitter.Node {
	if declarator.NamedChildCount() < 2 {
		return nil
	}
	return declarator.NamedChild(1)
}

// declaratorValueType returns the tree-sitter type of the value node in a
// variable_declarator, or "" if none. Kept for backward compatibility.
func declaratorValueType(declarator *sitter.Node) string {
	val := declaratorValue(declarator)
	if val == nil {
		return ""
	}
	return val.Type()
}

// componentWrappers is the set of function names that produce a React component
// when the result is assigned to a PascalCase variable.
var componentWrappers = map[string]bool{
	"forwardRef": true,
	"memo":       true,
	"lazy":       true,
}

// isComponentCallExpression returns true if the node is a call_expression whose
// function is a known React component wrapper: React.forwardRef, React.memo,
// React.lazy, or the bare forms forwardRef, memo, lazy.
// Mirrors the member_expression traversal pattern in api_calls.go.
func isComponentCallExpression(node *sitter.Node, src []byte) bool {
	if node == nil || node.Type() != "call_expression" {
		return false
	}
	// call_expression named children: [function, arguments] (same as api_calls.go:55)
	if node.NamedChildCount() < 1 {
		return false
	}
	fn := node.NamedChild(0)
	switch fn.Type() {
	case "member_expression":
		// React.forwardRef, React.memo, React.lazy
		// Named children: [object(identifier), property(property_identifier)] — confirmed by api_calls.go:139-140
		if fn.NamedChildCount() < 2 {
			return false
		}
		obj := fn.NamedChild(0).Content(src)
		prop := fn.NamedChild(1).Content(src)
		return obj == "React" && componentWrappers[prop]
	case "identifier":
		// bare forwardRef, memo, lazy
		return componentWrappers[fn.Content(src)]
	}
	return false
}

// funcType returns "component" for PascalCase names, "function" otherwise.
func funcType(name string) string {
	if isComponent(name) {
		return "component"
	}
	return "function"
}

// isComponent returns true when name starts with an uppercase ASCII letter,
// following the React PascalCase component convention.
func isComponent(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}
