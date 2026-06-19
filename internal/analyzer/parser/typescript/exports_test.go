package typescript_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer/parser/typescript"
)

func parseExports(t *testing.T, src string) []typescript.ExportInfo {
	t.Helper()
	p := typescript.NewParser()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return tree.Exports()
}

func assertExport(t *testing.T, exp typescript.ExportInfo, name, typ string) {
	t.Helper()
	if exp.Name != name {
		t.Errorf("Name: want %q, got %q", name, exp.Name)
	}
	if exp.Type != typ {
		t.Errorf("Type: want %q, got %q", typ, exp.Type)
	}
}

func TestExportsNamedFunction(t *testing.T) {
	exps := parseExports(t, `export function foo() {}`)
	if len(exps) != 1 {
		t.Fatalf("want 1 export, got %d", len(exps))
	}
	assertExport(t, exps[0], "foo", "function")
}

func TestExportsNamedClass(t *testing.T) {
	exps := parseExports(t, `export class Foo {}`)
	if len(exps) != 1 {
		t.Fatalf("want 1 export, got %d", len(exps))
	}
	assertExport(t, exps[0], "Foo", "class")
}

func TestExportsNamedInterface(t *testing.T) {
	exps := parseExports(t, `export interface Props {}`)
	if len(exps) != 1 {
		t.Fatalf("want 1 export, got %d", len(exps))
	}
	assertExport(t, exps[0], "Props", "interface")
}

func TestExportsNamedType(t *testing.T) {
	exps := parseExports(t, `export type User = string`)
	if len(exps) != 1 {
		t.Fatalf("want 1 export, got %d", len(exps))
	}
	assertExport(t, exps[0], "User", "type")
}

func TestExportsNamedVariable(t *testing.T) {
	exps := parseExports(t, `export const baz = 42`)
	if len(exps) != 1 {
		t.Fatalf("want 1 export, got %d", len(exps))
	}
	assertExport(t, exps[0], "baz", "variable")
}

func TestExportsDefaultFunction(t *testing.T) {
	exps := parseExports(t, `export default function() {}`)
	if len(exps) != 1 {
		t.Fatalf("want 1 export, got %d", len(exps))
	}
	assertExport(t, exps[0], "default", "default")
}

func TestExportsDefaultNamedFunction(t *testing.T) {
	// The name "foo" is irrelevant externally; consumers import it as "default".
	exps := parseExports(t, `export default function foo() {}`)
	if len(exps) != 1 {
		t.Fatalf("want 1 export, got %d", len(exps))
	}
	assertExport(t, exps[0], "default", "default")
}

func TestExportsDefaultClass(t *testing.T) {
	exps := parseExports(t, `export default class Foo {}`)
	if len(exps) != 1 {
		t.Fatalf("want 1 export, got %d", len(exps))
	}
	assertExport(t, exps[0], "default", "default")
}

func TestExportsDefaultExpression(t *testing.T) {
	exps := parseExports(t, `export default 42`)
	if len(exps) != 1 {
		t.Fatalf("want 1 export, got %d", len(exps))
	}
	assertExport(t, exps[0], "default", "default")
}

func TestExportsLocalAggregate(t *testing.T) {
	exps := parseExports(t, `export { foo, bar }`)
	if len(exps) != 2 {
		t.Fatalf("want 2 exports, got %d", len(exps))
	}
	assertExport(t, exps[0], "foo", "aggregated")
	assertExport(t, exps[1], "bar", "aggregated")
}

func TestExportsReexportNamed(t *testing.T) {
	exps := parseExports(t, `export { foo, bar } from "./x"`)
	if len(exps) != 2 {
		t.Fatalf("want 2 exports, got %d", len(exps))
	}
	assertExport(t, exps[0], "foo", "reexport")
	assertExport(t, exps[1], "bar", "reexport")
}

func TestExportsReexportWildcard(t *testing.T) {
	exps := parseExports(t, `export * from "./x"`)
	if len(exps) != 1 {
		t.Fatalf("want 1 export, got %d", len(exps))
	}
	assertExport(t, exps[0], "*", "wildcard_reexport")
}

func TestExportsReexportNamespace(t *testing.T) {
	exps := parseExports(t, `export * as utils from "./x"`)
	if len(exps) != 1 {
		t.Fatalf("want 1 export, got %d", len(exps))
	}
	assertExport(t, exps[0], "utils", "namespace_reexport")
}

func TestExportsReexportAlsoCountsAsImport(t *testing.T) {
	src := `export { foo } from "./x"`
	p := typescript.NewParser()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	imps := tree.Imports()
	if len(imps) != 1 {
		t.Fatalf("imports: want 1, got %d", len(imps))
	}
	if imps[0].Source != "./x" {
		t.Errorf("import source: want %q, got %q", "./x", imps[0].Source)
	}
	if !sliceEqual(imps[0].Names, []string{"foo"}) {
		t.Errorf("import names: want [foo], got %v", imps[0].Names)
	}

	exps := tree.Exports()
	if len(exps) != 1 {
		t.Fatalf("exports: want 1, got %d", len(exps))
	}
	assertExport(t, exps[0], "foo", "reexport")
}

func TestExportsLineNumbers(t *testing.T) {
	src := `export function foo() {}
export const bar = 1
export default 42`
	exps := parseExports(t, src)
	if len(exps) != 3 {
		t.Fatalf("want 3 exports, got %d", len(exps))
	}
	for i, exp := range exps {
		if exp.Line != i+1 {
			t.Errorf("export %d: want line %d, got %d", i, i+1, exp.Line)
		}
	}
}

func TestExportsReexportWithRename(t *testing.T) {
	// Public name is "renamed", not "foo".
	exps := parseExports(t, `export { foo as renamed } from "./x"`)
	if len(exps) != 1 {
		t.Fatalf("want 1 export, got %d", len(exps))
	}
	assertExport(t, exps[0], "renamed", "reexport")
}

func TestExportsAggregateWithRenameRecordsPublicName(t *testing.T) {
	// Without from clause: local aggregation. Public name is "renamed".
	exps := parseExports(t, `export { foo as renamed }`)
	if len(exps) != 1 {
		t.Fatalf("want 1 export, got %d", len(exps))
	}
	assertExport(t, exps[0], "renamed", "aggregated")
}

func TestExportsReexportImportRecordsSourceName(t *testing.T) {
	// For re-export { foo as renamed } from "./x":
	//   Import side: we fetch "foo" from "./x"
	//   Export side: we expose "renamed" to consumers
	src := `export { foo as renamed } from "./x"`
	p := typescript.NewParser()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	imps := tree.Imports()
	if len(imps) != 1 {
		t.Fatalf("imports: want 1, got %d", len(imps))
	}
	if !sliceEqual(imps[0].Names, []string{"foo"}) {
		t.Errorf("import names: want [foo] (source name), got %v", imps[0].Names)
	}

	exps := tree.Exports()
	if len(exps) != 1 {
		t.Fatalf("exports: want 1, got %d", len(exps))
	}
	if exps[0].Name != "renamed" {
		t.Errorf("export name: want %q (public name), got %q", "renamed", exps[0].Name)
	}
}
