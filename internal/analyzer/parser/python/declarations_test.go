package python_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer/parser/python"
)

func parseDeclarations(t *testing.T, src string) []python.DeclarationInfo {
	t.Helper()
	p := python.NewParser()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return tree.Declarations()
}

func TestExtractSimpleFunction(t *testing.T) {
	decls := parseDeclarations(t, "def hello():\n    pass\n")
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	d := decls[0]
	if d.Name != "hello" {
		t.Errorf("name: want %q, got %q", "hello", d.Name)
	}
	if d.Kind != "function" {
		t.Errorf("kind: want %q, got %q", "function", d.Kind)
	}
	if d.Line != 1 {
		t.Errorf("line: want 1, got %d", d.Line)
	}
	if len(d.Decorators) != 0 {
		t.Errorf("decorators: want none, got %v", d.Decorators)
	}
}

func TestExtractAsyncFunction(t *testing.T) {
	decls := parseDeclarations(t, "async def fetch():\n    pass\n")
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	if decls[0].Kind != "async_function" {
		t.Errorf("kind: want async_function, got %q", decls[0].Kind)
	}
	if decls[0].Name != "fetch" {
		t.Errorf("name: want %q, got %q", "fetch", decls[0].Name)
	}
}

func TestExtractFunctionWithTypeHints(t *testing.T) {
	decls := parseDeclarations(t, "def hello(name: str) -> str:\n    return name\n")
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	if decls[0].Name != "hello" {
		t.Errorf("name: want %q, got %q", "hello", decls[0].Name)
	}
	if decls[0].Kind != "function" {
		t.Errorf("kind: want function, got %q", decls[0].Kind)
	}
}

func TestExtractDecoratedFunction(t *testing.T) {
	src := "@app.get(\"/users\")\ndef list_users():\n    pass\n"
	decls := parseDeclarations(t, src)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	d := decls[0]
	if d.Name != "list_users" {
		t.Errorf("name: want %q, got %q", "list_users", d.Name)
	}
	if d.Kind != "function" {
		t.Errorf("kind: want function, got %q", d.Kind)
	}
	if d.Line != 2 {
		t.Errorf("line: want 2 (def line, not decorator line), got %d", d.Line)
	}
	if len(d.Decorators) != 1 {
		t.Fatalf("want 1 decorator, got %d: %v", len(d.Decorators), d.Decorators)
	}
	if d.Decorators[0] != `app.get("/users")` {
		t.Errorf("decorator: want %q, got %q", `app.get("/users")`, d.Decorators[0])
	}
}

func TestExtractMultipleDecorators(t *testing.T) {
	src := "@app.get(\"/users\")\n@requires_auth\ndef list_users():\n    pass\n"
	decls := parseDeclarations(t, src)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	d := decls[0]
	if len(d.Decorators) != 2 {
		t.Fatalf("want 2 decorators, got %d: %v", len(d.Decorators), d.Decorators)
	}
	if d.Decorators[0] != `app.get("/users")` {
		t.Errorf("decorator 0: want %q, got %q", `app.get("/users")`, d.Decorators[0])
	}
	if d.Decorators[1] != "requires_auth" {
		t.Errorf("decorator 1: want %q, got %q", "requires_auth", d.Decorators[1])
	}
}

func TestExtractDecoratedAsyncFunction(t *testing.T) {
	src := "@app.post(\"/items\")\nasync def create_item():\n    pass\n"
	decls := parseDeclarations(t, src)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	d := decls[0]
	if d.Kind != "async_function" {
		t.Errorf("kind: want async_function, got %q", d.Kind)
	}
	if len(d.Decorators) != 1 || d.Decorators[0] != `app.post("/items")` {
		t.Errorf("decorators: want [app.post(\"/items\")], got %v", d.Decorators)
	}
}

func TestExtractSimpleClass(t *testing.T) {
	decls := parseDeclarations(t, "class Foo:\n    pass\n")
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	d := decls[0]
	if d.Name != "Foo" {
		t.Errorf("name: want %q, got %q", "Foo", d.Name)
	}
	if d.Kind != "class" {
		t.Errorf("kind: want class, got %q", d.Kind)
	}
	if len(d.Bases) != 0 {
		t.Errorf("bases: want none, got %v", d.Bases)
	}
}

func TestExtractClassWithBases(t *testing.T) {
	decls := parseDeclarations(t, "class Foo(Bar, Baz):\n    pass\n")
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	d := decls[0]
	if !strSliceEqual(d.Bases, []string{"Bar", "Baz"}) {
		t.Errorf("bases: want [Bar Baz], got %v", d.Bases)
	}
}

func TestExtractDecoratedClass(t *testing.T) {
	src := "@dataclass\nclass User:\n    name: str\n"
	decls := parseDeclarations(t, src)
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	d := decls[0]
	if d.Name != "User" {
		t.Errorf("name: want %q, got %q", "User", d.Name)
	}
	if d.Kind != "class" {
		t.Errorf("kind: want class, got %q", d.Kind)
	}
	if len(d.Decorators) != 1 || d.Decorators[0] != "dataclass" {
		t.Errorf("decorators: want [dataclass], got %v", d.Decorators)
	}
}

func TestExtractModuleVariableWithCall(t *testing.T) {
	decls := parseDeclarations(t, "app = FastAPI()\n")
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	d := decls[0]
	if d.Name != "app" {
		t.Errorf("name: want %q, got %q", "app", d.Name)
	}
	if d.Kind != "variable" {
		t.Errorf("kind: want variable, got %q", d.Kind)
	}
	if d.ValueRepr != "FastAPI()" {
		t.Errorf("value_repr: want %q, got %q", "FastAPI()", d.ValueRepr)
	}
}

func TestExtractModuleVariableWithArgs(t *testing.T) {
	decls := parseDeclarations(t, "router = APIRouter(prefix=\"/users\", tags=[\"users\"])\n")
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(decls))
	}
	d := decls[0]
	if d.Name != "router" {
		t.Errorf("name: want %q, got %q", "router", d.Name)
	}
	if d.ValueRepr == "" {
		t.Error("value_repr: want non-empty for call with args")
	}
	// Should start with "APIRouter"
	if len(d.ValueRepr) < 9 || d.ValueRepr[:9] != "APIRouter" {
		t.Errorf("value_repr: want prefix APIRouter, got %q", d.ValueRepr)
	}
}

func TestExtractIntegerConstantVariable(t *testing.T) {
	decls := parseDeclarations(t, "MAX_RETRIES = 3\n")
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration for integer assignment, got %d: %+v", len(decls), decls)
	}
	if decls[0].Name != "MAX_RETRIES" || decls[0].Kind != "variable" {
		t.Errorf("want variable MAX_RETRIES, got %s %s", decls[0].Kind, decls[0].Name)
	}
	if decls[0].ValueRepr != "3" {
		t.Errorf("value_repr: want %q, got %q", "3", decls[0].ValueRepr)
	}
}

func TestExtractFloatConstantVariable(t *testing.T) {
	decls := parseDeclarations(t, "RATE = 1.5\n")
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration for float assignment, got %d: %+v", len(decls), decls)
	}
	if decls[0].ValueRepr != "1.5" {
		t.Errorf("value_repr: want %q, got %q", "1.5", decls[0].ValueRepr)
	}
}

func TestExtractStringConstantVariable(t *testing.T) {
	decls := parseDeclarations(t, "PREFIX = \"/api/v1\"\n")
	if len(decls) != 1 {
		t.Fatalf("want 1 declaration for string assignment, got %d: %+v", len(decls), decls)
	}
	if decls[0].Name != "PREFIX" || decls[0].Kind != "variable" {
		t.Errorf("want variable PREFIX, got %s %s", decls[0].Kind, decls[0].Name)
	}
	if decls[0].ValueRepr != `"/api/v1"` {
		t.Errorf("value_repr: want %q, got %q", `"/api/v1"`, decls[0].ValueRepr)
	}
}

func TestSkipListVariable(t *testing.T) {
	decls := parseDeclarations(t, "TAGS = [\"a\", \"b\"]\n")
	if len(decls) != 0 {
		t.Errorf("want 0 declarations for list assignment, got %d: %+v", len(decls), decls)
	}
}

func TestSkipDictVariable(t *testing.T) {
	decls := parseDeclarations(t, "CFG = {\"x\": 1}\n")
	if len(decls) != 0 {
		t.Errorf("want 0 declarations for dict assignment, got %d: %+v", len(decls), decls)
	}
}

func TestSkipTupleVariable(t *testing.T) {
	decls := parseDeclarations(t, "PT = (1, 2)\n")
	if len(decls) != 0 {
		t.Errorf("want 0 declarations for tuple assignment, got %d: %+v", len(decls), decls)
	}
}

func TestExtractMultipleDeclarations(t *testing.T) {
	src := "app = FastAPI()\n\nclass UserBase:\n    pass\n\ndef health():\n    return {}\n"
	decls := parseDeclarations(t, src)
	if len(decls) != 3 {
		t.Fatalf("want 3 declarations, got %d: %+v", len(decls), decls)
	}
	if decls[0].Kind != "variable" || decls[0].Name != "app" {
		t.Errorf("decl 0: want variable app, got %s %s", decls[0].Kind, decls[0].Name)
	}
	if decls[1].Kind != "class" || decls[1].Name != "UserBase" {
		t.Errorf("decl 1: want class UserBase, got %s %s", decls[1].Kind, decls[1].Name)
	}
	if decls[2].Kind != "function" || decls[2].Name != "health" {
		t.Errorf("decl 2: want function health, got %s %s", decls[2].Kind, decls[2].Name)
	}
}

func TestExtractRealFastAPIFile(t *testing.T) {
	src := `from fastapi import FastAPI, APIRouter

app = FastAPI(title="Test API")
router = APIRouter(prefix="/users", tags=["users"])

class UserBase:
    pass

@dataclass
class User(UserBase):
    name: str

@app.get("/health")
def health():
    return {"status": "ok"}

@router.post("/", response_model=User)
async def create_user(user: dict):
    return user

def internal_helper(x: int) -> int:
    return x * 2
`
	decls := parseDeclarations(t, src)
	if len(decls) != 7 {
		t.Fatalf("want 7 declarations, got %d: %+v", len(decls), decls)
	}

	want := []struct {
		name string
		kind string
	}{
		{"app", "variable"},
		{"router", "variable"},
		{"UserBase", "class"},
		{"User", "class"},
		{"health", "function"},
		{"create_user", "async_function"},
		{"internal_helper", "function"},
	}

	for i, w := range want {
		if decls[i].Name != w.name {
			t.Errorf("decl %d name: want %q, got %q", i, w.name, decls[i].Name)
		}
		if decls[i].Kind != w.kind {
			t.Errorf("decl %d kind: want %q, got %q", i, w.kind, decls[i].Kind)
		}
	}

	// Spot-check decorators
	user := decls[3]
	if len(user.Decorators) != 1 || user.Decorators[0] != "dataclass" {
		t.Errorf("User.decorators: want [dataclass], got %v", user.Decorators)
	}
	if !strSliceEqual(user.Bases, []string{"UserBase"}) {
		t.Errorf("User.bases: want [UserBase], got %v", user.Bases)
	}

	health := decls[4]
	if len(health.Decorators) != 1 {
		t.Errorf("health.decorators: want 1, got %v", health.Decorators)
	}

	createUser := decls[5]
	if len(createUser.Decorators) != 1 {
		t.Errorf("create_user.decorators: want 1, got %v", createUser.Decorators)
	}
}

func TestNoDeclarations(t *testing.T) {
	src := "import os\nfrom typing import List\n# just a comment\n"
	decls := parseDeclarations(t, src)
	if len(decls) != 0 {
		t.Errorf("want 0 declarations, got %d: %+v", len(decls), decls)
	}
}
