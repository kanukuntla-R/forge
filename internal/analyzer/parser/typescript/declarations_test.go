package typescript_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer/parser/typescript"
)

func parseDeclarations(t *testing.T, src string) []typescript.DeclarationInfo {
	t.Helper()
	p := typescript.NewParser()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return tree.Declarations()
}

func assertDecl(t *testing.T, d typescript.DeclarationInfo, name, typ string) {
	t.Helper()
	if d.Name != name {
		t.Errorf("Name: want %q, got %q", name, d.Name)
	}
	if d.Type != typ {
		t.Errorf("Type: want %q, got %q", typ, d.Type)
	}
}

func TestDeclarationsFunction(t *testing.T) {
	decls := parseDeclarations(t, `function helper() {}`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "helper", "function")
}

func TestDeclarationsAsyncFunction(t *testing.T) {
	decls := parseDeclarations(t, `async function fetchData() {}`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "fetchData", "function")
}

func TestDeclarationsComponent(t *testing.T) {
	decls := parseDeclarations(t, `function Header() {}`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "Header", "component")
}

func TestDeclarationsArrowComponent(t *testing.T) {
	decls := parseDeclarations(t, `const Header = () => null`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "Header", "component")
}

func TestDeclarationsArrowFunction(t *testing.T) {
	decls := parseDeclarations(t, `const helper = () => null`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "helper", "variable")
}

func TestDeclarationsArrowFunctionNoArgs(t *testing.T) {
	decls := parseDeclarations(t, `const x = () => null`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "x", "variable")
}

func TestDeclarationsClass(t *testing.T) {
	decls := parseDeclarations(t, `class Foo {}`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "Foo", "class")
}

func TestDeclarationsInterface(t *testing.T) {
	decls := parseDeclarations(t, `interface Props {}`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "Props", "interface")
}

func TestDeclarationsTypeAlias(t *testing.T) {
	decls := parseDeclarations(t, `type User = string`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "User", "type")
}

func TestDeclarationsConstObject(t *testing.T) {
	// PascalCase but not an arrow function — stays "variable".
	decls := parseDeclarations(t, `const Config = { x: 1 }`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "Config", "variable")
}

func TestDeclarationsMultipleConst(t *testing.T) {
	decls := parseDeclarations(t, `const a = 1, b = 2`)
	if len(decls) != 2 {
		t.Fatalf("want 2 declarations, got %d", len(decls))
	}
	assertDecl(t, decls[0], "a", "variable")
	assertDecl(t, decls[1], "b", "variable")
}

func TestDeclarationsLet(t *testing.T) {
	decls := parseDeclarations(t, `let counter = 0`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "counter", "variable")
}

func TestDeclarationsVar(t *testing.T) {
	decls := parseDeclarations(t, `var legacy = 0`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "legacy", "variable")
}

func TestDeclarationsExportedFunction(t *testing.T) {
	decls := parseDeclarations(t, `export function foo() {}`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "foo", "function")
}

func TestDeclarationsExportedComponent(t *testing.T) {
	decls := parseDeclarations(t, `export function Header() {}`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "Header", "component")
}

func TestDeclarationsExportedConst(t *testing.T) {
	decls := parseDeclarations(t, `export const Header = () => null`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "Header", "component")
}

func TestDeclarationsDefaultAnonExportNotCounted(t *testing.T) {
	// export default function() {} → anonymous, no name → zero declarations.
	decls := parseDeclarations(t, `export default function() {}`)
	if len(decls) != 0 {
		t.Errorf("want 0 declarations for anonymous default export, got %d: %v", len(decls), decls)
	}
}

func TestDeclarationsDefaultNamedExportCounted(t *testing.T) {
	// export default function foo() {} → function_declaration with identifier "foo".
	// The function has a real name at the top level; it counts as a declaration.
	decls := parseDeclarations(t, `export default function foo() {}`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	assertDecl(t, decls[0], "foo", "function")
}

func TestDeclarationsNotNested(t *testing.T) {
	src := `function outer() {
  function inner() {}
  const helper = () => null
}`
	decls := parseDeclarations(t, src)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration (outer only), got %d: %v", len(decls), decls)
	}
	assertDecl(t, decls[0], "outer", "function")
}

func TestDeclarationsLineNumbers(t *testing.T) {
	src := `function foo() {}
class Bar {}
const baz = 42`
	decls := parseDeclarations(t, src)
	if len(decls) != 3 {
		t.Fatalf("want 3 declarations, got %d", len(decls))
	}
	for i, d := range decls {
		if d.Line != i+1 {
			t.Errorf("decl %d: want line %d, got %d", i, i+1, d.Line)
		}
	}
}

func TestDeclarationsClassMethodsNotCounted(t *testing.T) {
	decls := parseDeclarations(t, `class Foo { method() {} }`)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration (Foo only), got %d: %v", len(decls), decls)
	}
	assertDecl(t, decls[0], "Foo", "class")
}
