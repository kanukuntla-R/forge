package typescript

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// ChainSegment is one link in a flattened fluent chain: a call (Args holds
// the raw source text of each argument) or a bare property access with no
// call, e.g. "query" and "users" in db.query.users.findMany().
type ChainSegment struct {
	Name string
	Args []string // nil if this segment has no call (property access) or the call took no arguments
}

// MethodChain represents a fluent chain of calls/property accesses rooted at
// a single identifier, e.g. db.select().from(users) or db.query.users.findMany().
// Unlike ChainedCalls, chain depth and call arity are unconstrained — needed
// for ORMs like Drizzle whose query shape isn't a fixed a.b.c() call.
type MethodChain struct {
	Root  string
	Calls []ChainSegment // ordered root-outward: db.select().from(x) -> [select, from]
	Line  int            // 1-indexed
}

// MethodChains walks the full AST and returns every fluent chain rooted at an
// identifier.
func (t *Tree) MethodChains() []MethodChain {
	var out []MethodChain
	walkForMethodChains(t.tree.RootNode(), t.source, &out)
	return out
}

// walkForMethodChains recurses into every named child. Only the outermost
// call_expression of each chain is flattened — isChainInner skips a call
// whose result feeds directly into another chained call, since flattenChain
// already walks back through it.
func walkForMethodChains(node *sitter.Node, src []byte, out *[]MethodChain) {
	if node.Type() == "call_expression" && !isChainInner(node) {
		if chain, ok := flattenChain(node, src); ok {
			*out = append(*out, chain)
		}
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		walkForMethodChains(node.NamedChild(i), src, out)
	}
}

// isChainInner reports whether node's result is consumed directly as the
// object of an outer chained call, e.g. db.select() inside db.select().from(x).
func isChainInner(node *sitter.Node) bool {
	parent := node.Parent()
	if parent == nil || parent.Type() != "member_expression" {
		return false
	}
	grandparent := parent.Parent()
	if grandparent == nil || grandparent.Type() != "call_expression" {
		return false
	}
	return grandparent.NamedChild(0) == parent
}

// flattenChain walks a call_expression backward through its chain of calls
// and property accesses down to a root identifier, collecting one
// ChainSegment per level. Returns ok=false for shapes that don't resolve to
// identifier(.property)*(.call(...))+.
func flattenChain(node *sitter.Node, src []byte) (MethodChain, bool) {
	line := int(node.StartPoint().Row) + 1
	var segments []ChainSegment

	cur := node
	for {
		if cur.Type() != "call_expression" || cur.NamedChildCount() < 1 {
			return MethodChain{}, false
		}
		fn := cur.NamedChild(0)
		if fn.Type() != "member_expression" || fn.NamedChildCount() < 2 {
			return MethodChain{}, false
		}
		prop := fn.NamedChild(1)
		if prop.Type() != "property_identifier" {
			return MethodChain{}, false
		}
		segments = append([]ChainSegment{{Name: prop.Content(src), Args: callArgs(cur, src)}}, segments...)

		obj := fn.NamedChild(0)
		switch obj.Type() {
		case "identifier":
			return MethodChain{Root: obj.Content(src), Calls: segments, Line: line}, true
		case "call_expression":
			cur = obj
		case "member_expression":
			props, root, ok := flattenMemberChain(obj, src)
			if !ok {
				return MethodChain{}, false
			}
			for i := len(props) - 1; i >= 0; i-- {
				segments = append([]ChainSegment{{Name: props[i]}}, segments...)
			}
			return MethodChain{Root: root, Calls: segments, Line: line}, true
		default:
			return MethodChain{}, false
		}
	}
}

// flattenMemberChain unwraps a chain of property accesses (no calls) down to
// its root identifier, e.g. db.query.users -> (["query", "users"], "db").
func flattenMemberChain(node *sitter.Node, src []byte) (props []string, root string, ok bool) {
	if node.Type() == "identifier" {
		return nil, node.Content(src), true
	}
	if node.Type() != "member_expression" || node.NamedChildCount() < 2 {
		return nil, "", false
	}
	prop := node.NamedChild(1)
	if prop.Type() != "property_identifier" {
		return nil, "", false
	}
	innerProps, root, ok := flattenMemberChain(node.NamedChild(0), src)
	if !ok {
		return nil, "", false
	}
	return append(innerProps, prop.Content(src)), root, true
}

// callArgs returns the raw source text of each argument to a call_expression.
func callArgs(node *sitter.Node, src []byte) []string {
	if node.NamedChildCount() < 2 {
		return nil
	}
	argsNode := node.NamedChild(1)
	if argsNode.NamedChildCount() == 0 {
		return nil
	}
	args := make([]string, argsNode.NamedChildCount())
	for i := range args {
		args[i] = argsNode.NamedChild(i).Content(src)
	}
	return args
}
