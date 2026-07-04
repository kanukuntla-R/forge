package python_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer/parser/python"
)

func parseModuleCalls(t *testing.T, src string) []python.CallInfo {
	t.Helper()
	p := python.NewParser()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return tree.ModuleCalls()
}

func TestModuleCall_SimpleIdentifierArg(t *testing.T) {
	calls := parseModuleCalls(t, "app.include_router(auth_router)\n")
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d: %+v", len(calls), calls)
	}
	c := calls[0]
	if c.FunctionRepr != "app.include_router" {
		t.Errorf("function: want %q, got %q", "app.include_router", c.FunctionRepr)
	}
	if c.ArgsRepr != "auth_router" {
		t.Errorf("args: want %q, got %q", "auth_router", c.ArgsRepr)
	}
	if c.Line != 1 {
		t.Errorf("line: want 1, got %d", c.Line)
	}
}

func TestModuleCall_AttributeArg(t *testing.T) {
	calls := parseModuleCalls(t, "app.include_router(users.router)\n")
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	if calls[0].ArgsRepr != "users.router" {
		t.Errorf("args: want %q, got %q", "users.router", calls[0].ArgsRepr)
	}
}

func TestModuleCall_MultipleArgsKeepsFullText(t *testing.T) {
	calls := parseModuleCalls(t, `app.include_router(users.router, prefix="/users", tags=["users"])`+"\n")
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	want := `users.router, prefix="/users", tags=["users"]`
	if calls[0].ArgsRepr != want {
		t.Errorf("args: want %q, got %q", want, calls[0].ArgsRepr)
	}
}

func TestModuleCall_NoArgs(t *testing.T) {
	calls := parseModuleCalls(t, "app.setup()\n")
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	if calls[0].ArgsRepr != "" {
		t.Errorf("args: want empty, got %q", calls[0].ArgsRepr)
	}
	if calls[0].FunctionRepr != "app.setup" {
		t.Errorf("function: want %q, got %q", "app.setup", calls[0].FunctionRepr)
	}
}

func TestModuleCall_BareFunctionCall(t *testing.T) {
	// Not an attribute call (no dot) — e.g. `configure_logging()`.
	calls := parseModuleCalls(t, "configure_logging()\n")
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	if calls[0].FunctionRepr != "configure_logging" {
		t.Errorf("function: want %q, got %q", "configure_logging", calls[0].FunctionRepr)
	}
}

func TestModuleCall_NotExtractedFromAssignment(t *testing.T) {
	// `app = FastAPI()` is an assignment, not a bare call statement — should not appear here.
	calls := parseModuleCalls(t, "app = FastAPI()\n")
	if len(calls) != 0 {
		t.Errorf("want 0 module calls for assignment statement, got %d: %+v", len(calls), calls)
	}
}

func TestModuleCall_MultipleStatements(t *testing.T) {
	src := "app.include_router(users.router)\napp.include_router(auth_router, prefix=\"/auth\")\n"
	calls := parseModuleCalls(t, src)
	if len(calls) != 2 {
		t.Fatalf("want 2 calls, got %d: %+v", len(calls), calls)
	}
	if calls[0].ArgsRepr != "users.router" {
		t.Errorf("call 0 args: want %q, got %q", "users.router", calls[0].ArgsRepr)
	}
	if calls[1].ArgsRepr != `auth_router, prefix="/auth"` {
		t.Errorf("call 1 args: want %q, got %q", `auth_router, prefix="/auth"`, calls[1].ArgsRepr)
	}
	if calls[1].Line != 2 {
		t.Errorf("call 1 line: want 2, got %d", calls[1].Line)
	}
}
