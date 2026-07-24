package typescript

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// ChainedCall represents a call of the shape object.property.method(...),
// e.g. prisma.user.findMany(). Extraction is ORM-agnostic — callers interpret
// Object/Property/Method as needed.
type ChainedCall struct {
	Object   string
	Property string
	Method   string
	Line     int // 1-indexed
}

// ChainedCalls walks the full AST recursively and returns every call whose
// callee is exactly a three-level member expression: identifier.identifier.identifier(...).
// Shallower (a.b()) or more complex (a().b(), a[b].c()) shapes are skipped.
func (t *Tree) ChainedCalls() []ChainedCall {
	var out []ChainedCall
	walkForChainedCalls(t.tree.RootNode(), t.source, &out)
	return out
}

// walkForChainedCalls mirrors walkForCalls in api_calls.go: recurse into every
// named child regardless of whether the current node matched.
func walkForChainedCalls(node *sitter.Node, src []byte, out *[]ChainedCall) {
	if node.Type() == "call_expression" {
		if call, ok := extractChainedCall(node, src); ok {
			*out = append(*out, call)
		}
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		walkForChainedCalls(node.NamedChild(i), src, out)
	}
}

// extractChainedCall matches call_expression nodes shaped like:
//
//	call_expression
//	  member_expression (function)
//	    member_expression (object)
//	      identifier (object)          -> "prisma"
//	      property_identifier (property) -> "user"
//	    property_identifier (property) -> "findMany"
//
// Anything else (a.b(), a().b(), a[b].c()) returns ok=false.
func extractChainedCall(node *sitter.Node, src []byte) (ChainedCall, bool) {
	if node.NamedChildCount() < 1 {
		return ChainedCall{}, false
	}
	fn := node.NamedChild(0)
	if fn.Type() != "member_expression" || fn.NamedChildCount() < 2 {
		return ChainedCall{}, false
	}
	method := fn.NamedChild(1)
	if method.Type() != "property_identifier" {
		return ChainedCall{}, false
	}

	inner := fn.NamedChild(0)
	if inner.Type() != "member_expression" || inner.NamedChildCount() < 2 {
		return ChainedCall{}, false
	}
	object := inner.NamedChild(0)
	property := inner.NamedChild(1)
	if object.Type() != "identifier" || property.Type() != "property_identifier" {
		return ChainedCall{}, false
	}

	return ChainedCall{
		Object:   object.Content(src),
		Property: property.Content(src),
		Method:   method.Content(src),
		Line:     int(node.StartPoint().Row) + 1,
	}, true
}
