package typescript_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer/parser/typescript"
)

func parseCalls(t *testing.T, src string) []typescript.CallInfo {
	t.Helper()
	p := typescript.NewParser()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return tree.APICalls()
}

func assertCall(t *testing.T, c typescript.CallInfo, target, method, kind, confidence string) {
	t.Helper()
	if c.Target != target {
		t.Errorf("Target: want %q, got %q", target, c.Target)
	}
	if c.Method != method {
		t.Errorf("Method: want %q, got %q", method, c.Method)
	}
	if c.Kind != kind {
		t.Errorf("Kind: want %q, got %q", kind, c.Kind)
	}
	if c.Confidence != confidence {
		t.Errorf("Confidence: want %q, got %q", confidence, c.Confidence)
	}
}

func TestAPICallsFetchLiteral(t *testing.T) {
	calls := parseCalls(t, `fetch("/api/users")`)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	assertCall(t, calls[0], "/api/users", "GET", "fetch", "high")
}

func TestAPICallsFetchWithMethod(t *testing.T) {
	calls := parseCalls(t, `fetch("/api/users", { method: "POST" })`)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	assertCall(t, calls[0], "/api/users", "POST", "fetch", "high")
}

func TestAPICallsFetchTemplateLiteral(t *testing.T) {
	calls := parseCalls(t, "fetch(`/api/users`)")
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	assertCall(t, calls[0], "/api/users", "GET", "fetch", "high")
}

func TestAPICallsFetchTemplateWithInterp(t *testing.T) {
	calls := parseCalls(t, "fetch(`/api/products/${id}`)")
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	assertCall(t, calls[0], "/api/products/:id", "GET", "fetch", "medium")
}

func TestAPICallsFetchTemplateComplexExpr(t *testing.T) {
	// ${user.id} is a member_expression → placeholder :?
	calls := parseCalls(t, "fetch(`/api/products/${user.id}`)")
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	if calls[0].Target != "/api/products/:?" {
		t.Errorf("target: want /api/products/:?, got %q", calls[0].Target)
	}
	if calls[0].Confidence != "medium" {
		t.Errorf("confidence: want medium, got %q", calls[0].Confidence)
	}
}

func TestAPICallsAxiosFunction(t *testing.T) {
	calls := parseCalls(t, `axios("/api/users")`)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	assertCall(t, calls[0], "/api/users", "GET", "axios", "high")
}

func TestAPICallsAxiosGet(t *testing.T) {
	calls := parseCalls(t, `axios.get("/api/users")`)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	assertCall(t, calls[0], "/api/users", "GET", "axios", "high")
}

func TestAPICallsAxiosPost(t *testing.T) {
	calls := parseCalls(t, `axios.post("/api/users", body)`)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	assertCall(t, calls[0], "/api/users", "POST", "axios", "high")
}

func TestAPICallsKyFunction(t *testing.T) {
	calls := parseCalls(t, `ky("/api/users")`)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	assertCall(t, calls[0], "/api/users", "GET", "ky", "high")
}

func TestAPICallsKyGet(t *testing.T) {
	calls := parseCalls(t, `ky.get("/api/users")`)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	assertCall(t, calls[0], "/api/users", "GET", "ky", "high")
}

func TestAPICallsHeuristicURL(t *testing.T) {
	calls := parseCalls(t, `apiClient("/api/users")`)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	assertCall(t, calls[0], "/api/users", "GET", "heuristic", "medium")
}

func TestAPICallsHeuristicSkipsNonURL(t *testing.T) {
	calls := parseCalls(t, `someFunc("not a url")`)
	if len(calls) != 0 {
		t.Errorf("want 0 calls for non-URL string, got %d: %v", len(calls), calls)
	}
}

func TestAPICallsHeuristicSkipsVariable(t *testing.T) {
	// fetch(url) — variable, not a string literal
	calls := parseCalls(t, `fetch(url)`)
	if len(calls) != 0 {
		t.Errorf("want 0 calls for variable URL, got %d", len(calls))
	}
}

func TestAPICallsLineNumbers(t *testing.T) {
	src := `fetch("/api/a")
axios.get("/api/b")
ky("/api/c")`
	calls := parseCalls(t, src)
	if len(calls) != 3 {
		t.Fatalf("want 3 calls, got %d", len(calls))
	}
	for i, call := range calls {
		if call.Line != i+1 {
			t.Errorf("call %d: want line %d, got %d", i, i+1, call.Line)
		}
	}
}

func TestAPICallsAxiosUppercaseMethod(t *testing.T) {
	// axios.PATCH — property name is uppercase in source; should normalize to PATCH
	calls := parseCalls(t, `axios.PATCH("/api/users", body)`)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	if calls[0].Method != "PATCH" {
		t.Errorf("method: want PATCH, got %q", calls[0].Method)
	}
	if calls[0].Kind != "axios" {
		t.Errorf("kind: want axios, got %q", calls[0].Kind)
	}
}

func TestAPICallsFetchWithDeleteMethod(t *testing.T) {
	calls := parseCalls(t, `fetch("/api/users", { method: "DELETE", headers: {} })`)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	assertCall(t, calls[0], "/api/users", "DELETE", "fetch", "high")
}

func TestAPICallsNestedInAsyncFunction(t *testing.T) {
	src := `export default async function Home() {
  const data = await fetch("/api/users")
  return null
}`
	calls := parseCalls(t, src)
	if len(calls) != 1 {
		t.Fatalf("want 1 call from nested fetch, got %d", len(calls))
	}
	assertCall(t, calls[0], "/api/users", "GET", "fetch", "high")
}
