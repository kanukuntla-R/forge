package python

// ImportInfo holds the extracted data for a single Python import statement.
// It maps 1:1 to analyzer.Import once consumed by the adapter.
type ImportInfo struct {
	Source     string
	Names      []string // never nil; [] for plain module imports
	Alias      string   // set for `import X as Y` (top-level alias only)
	IsRelative bool     // true when source starts with dots (from . / from ..)
	IsStar     bool     // true for `from X import *`
	External   bool     // false only for relative imports
	Line       int      // 1-indexed
}

// Imports walks the module root and returns all top-level import statements
// as structured ImportInfo values.
func (t *Tree) Imports() []ImportInfo {
	root := t.RootNode()
	if root == nil {
		return nil
	}
	var out []ImportInfo
	for i := 0; i < root.NamedChildCount(); i++ {
		child := root.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "import_statement":
			out = append(out, extractPlainImports(child)...)
		case "import_from_statement":
			if imp, ok := extractFromImport(child); ok {
				out = append(out, imp)
			}
		}
	}
	return out
}

// extractPlainImports handles `import X`, `import X, Y`, and `import X as Y`.
// One import_statement can introduce multiple modules (comma-separated), so
// this returns a slice.
func extractPlainImports(node *Node) []ImportInfo {
	var out []ImportInfo
	line := node.StartLine()
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "dotted_name":
			out = append(out, ImportInfo{
				Source:   child.Content(),
				Names:    []string{},
				External: true,
				Line:     line,
			})
		case "aliased_import":
			// aliased_import has two named children: dotted_name + identifier (alias).
			source, alias := "", ""
			for j := 0; j < child.NamedChildCount(); j++ {
				gc := child.NamedChild(j)
				if gc == nil {
					continue
				}
				switch gc.Type() {
				case "dotted_name":
					source = gc.Content()
				case "identifier":
					alias = gc.Content()
				}
			}
			if source != "" {
				out = append(out, ImportInfo{
					Source:   source,
					Names:    []string{},
					Alias:    alias,
					External: true,
					Line:     line,
				})
			}
		}
	}
	return out
}

// extractFromImport handles `from X import Y` (absolute) and `from .X import Y` (relative).
func extractFromImport(node *Node) (ImportInfo, bool) {
	if node.NamedChildCount() == 0 {
		return ImportInfo{}, false
	}

	first := node.NamedChild(0)
	if first == nil {
		return ImportInfo{}, false
	}

	var source string
	isRelative := false

	switch first.Type() {
	case "dotted_name":
		source = first.Content()
	case "relative_import":
		// relative_import.Content() gives ".", ".config", "..models" directly.
		isRelative = true
		source = first.Content()
	default:
		return ImportInfo{}, false
	}

	var names []string
	star := false

	for i := 1; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "dotted_name":
			names = append(names, child.Content())
		case "aliased_import":
			// `from typing import List as L` — record "List", drop alias.
			for j := 0; j < child.NamedChildCount(); j++ {
				gc := child.NamedChild(j)
				if gc != nil && gc.Type() == "dotted_name" {
					names = append(names, gc.Content())
					break
				}
			}
		case "wildcard_import":
			star = true
		}
	}

	if names == nil {
		names = []string{}
	}

	return ImportInfo{
		Source:     source,
		Names:      names,
		IsRelative: isRelative,
		IsStar:     star,
		External:   !isRelative,
		Line:       node.StartLine(),
	}, true
}
