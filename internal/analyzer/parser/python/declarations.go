package python

import "strings"

// DeclarationInfo holds the extracted data for a single top-level Python declaration.
// It maps 1:1 to analyzer.Declaration once consumed by the adapter.
type DeclarationInfo struct {
	Name       string
	Kind       string   // "function", "async_function", "class", "variable"
	Line       int      // 1-indexed line of the def/class/assignment (not the first decorator)
	Decorators []string // nil when none (omitted from JSON via omitempty)
	Bases      []string // nil when no base classes
	ValueRepr  string   // for variables: truncated RHS call expression
}

// Declarations walks the module root and returns all top-level declarations
// as structured DeclarationInfo values.
// Nested declarations (methods, inner functions, inner classes) are not visited.
func (t *Tree) Declarations() []DeclarationInfo {
	root := t.RootNode()
	if root == nil {
		return nil
	}
	var out []DeclarationInfo
	for i := 0; i < root.NamedChildCount(); i++ {
		child := root.NamedChild(i)
		if child == nil {
			continue
		}
		if d, ok := extractDeclaration(child); ok {
			out = append(out, d)
		}
	}
	return out
}

// extractDeclaration converts one root-level node into a DeclarationInfo, if applicable.
func extractDeclaration(node *Node) (DeclarationInfo, bool) {
	switch node.Type() {
	case "function_definition":
		return extractFunctionDecl(node, nil), true
	case "class_definition":
		return extractClassDecl(node, nil), true
	case "expression_statement":
		return extractVariableDecl(node)
	case "decorated_definition":
		return extractDecoratedDecl(node)
	}
	return DeclarationInfo{}, false
}

// extractFunctionDecl builds a DeclarationInfo from a function_definition node.
// decorators is nil for plain (non-decorated) functions.
func extractFunctionDecl(node *Node, decorators []string) DeclarationInfo {
	name := ""
	for i := 0; i < node.NamedChildCount(); i++ {
		if c := node.NamedChild(i); c != nil && c.Type() == "identifier" {
			name = c.Content()
			break
		}
	}
	kind := "function"
	if node.HasAsyncKeyword() {
		kind = "async_function"
	}
	return DeclarationInfo{
		Name:       name,
		Kind:       kind,
		Line:       node.StartLine(),
		Decorators: decorators,
	}
}

// extractClassDecl builds a DeclarationInfo from a class_definition node.
// decorators is nil for plain (non-decorated) classes.
func extractClassDecl(node *Node, decorators []string) DeclarationInfo {
	name := ""
	var bases []string
	for i := 0; i < node.NamedChildCount(); i++ {
		c := node.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Type() {
		case "identifier":
			if name == "" {
				name = c.Content()
			}
		case "argument_list":
			for j := 0; j < c.NamedChildCount(); j++ {
				if gc := c.NamedChild(j); gc != nil && gc.Type() == "identifier" {
					bases = append(bases, gc.Content())
				}
			}
		}
	}
	return DeclarationInfo{
		Name:       name,
		Kind:       "class",
		Line:       node.StartLine(),
		Decorators: decorators,
		Bases:      bases,
	}
}

// extractVariableDecl handles expression_statement nodes that wrap a simple assignment
// whose RHS is a call, string, or numeric literal (e.g., `app = FastAPI()`,
// `PREFIX = "/api/v1"`, `MAX_RETRIES = 3`). Collection literals (list, dict, tuple)
// are skipped — too varied in shape to be useful as a flat ValueRepr.
func extractVariableDecl(node *Node) (DeclarationInfo, bool) {
	if node.NamedChildCount() == 0 {
		return DeclarationInfo{}, false
	}
	// expression_statement → assignment
	assign := node.NamedChild(0)
	if assign == nil || assign.Type() != "assignment" {
		return DeclarationInfo{}, false
	}
	if assign.NamedChildCount() < 2 {
		return DeclarationInfo{}, false
	}
	// LHS must be a simple identifier (not a tuple, subscript, etc.)
	lhs := assign.NamedChild(0)
	if lhs == nil || lhs.Type() != "identifier" {
		return DeclarationInfo{}, false
	}
	// RHS: last named child handles both plain (`a = Foo()`) and annotated (`a: T = Foo()`).
	// tree-sitter-python doesn't expose a named field for the RHS of assignment, so we
	// use positional access to the last named child — a grammar limitation, not a style choice.
	rhs := assign.NamedChild(assign.NamedChildCount() - 1)
	if rhs == nil {
		return DeclarationInfo{}, false
	}
	switch rhs.Type() {
	case "call", "string", "integer", "float":
	default:
		return DeclarationInfo{}, false
	}

	v := rhs.Content()
	const maxValueRepr = 200
	if len(v) > maxValueRepr {
		v = v[:maxValueRepr] + "..."
	}

	return DeclarationInfo{
		Name:      lhs.Content(),
		Kind:      "variable",
		Line:      node.StartLine(),
		ValueRepr: v,
	}, true
}

// extractDecoratedDecl handles decorated_definition nodes.
// It collects all decorator expressions, then delegates to extractFunctionDecl
// or extractClassDecl for the wrapped definition, passing the decorators along.
// The resulting declaration has the line number of the def/class, not the first decorator.
func extractDecoratedDecl(node *Node) (DeclarationInfo, bool) {
	var decorators []string
	var defNode *Node

	for i := 0; i < node.NamedChildCount(); i++ {
		c := node.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Type() {
		case "decorator":
			// decorator content is "@expression" — strip the leading @ to get the expression.
			text := c.Content()
			if len(text) > 1 {
				decorators = append(decorators, strings.TrimSpace(text[1:]))
			}
		case "function_definition", "class_definition":
			defNode = c
		}
	}

	if defNode == nil {
		return DeclarationInfo{}, false
	}

	switch defNode.Type() {
	case "function_definition":
		return extractFunctionDecl(defNode, decorators), true
	case "class_definition":
		return extractClassDecl(defNode, decorators), true
	}
	return DeclarationInfo{}, false
}
