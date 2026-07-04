package python_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer/parser/python"
)

func parseAPICalls(t *testing.T, src string) []python.APICallInfo {
	t.Helper()
	p := python.NewParser()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return tree.APICalls()
}

func TestDetectRequestsGet(t *testing.T) {
	calls := parseAPICalls(t, `requests.get("/api/users")`)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d: %+v", len(calls), calls)
	}
	c := calls[0]
	if c.Method != "GET" || c.URL != "/api/users" || c.Library != "requests" {
		t.Errorf("call: got %+v", c)
	}
	if c.Confidence != "high" {
		t.Errorf("confidence: want high, got %q", c.Confidence)
	}
	if c.Line != 1 {
		t.Errorf("line: want 1, got %d", c.Line)
	}
}

func TestDetectRequestsAllMethods(t *testing.T) {
	src := `requests.get("/x")
requests.post("/x")
requests.put("/x")
requests.delete("/x")
requests.patch("/x")
`
	calls := parseAPICalls(t, src)
	if len(calls) != 5 {
		t.Fatalf("want 5 calls, got %d: %+v", len(calls), calls)
	}
	want := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	for i, m := range want {
		if calls[i].Method != m {
			t.Errorf("call %d: want method %q, got %q", i, m, calls[i].Method)
		}
		if calls[i].Library != "requests" {
			t.Errorf("call %d: want library requests, got %q", i, calls[i].Library)
		}
	}
}

func TestDetectHttpxGet(t *testing.T) {
	calls := parseAPICalls(t, `httpx.get("/x")`)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	if calls[0].Method != "GET" || calls[0].Library != "httpx" {
		t.Errorf("call: got %+v", calls[0])
	}
}

func TestDetectUrllibUrlopen(t *testing.T) {
	calls := parseAPICalls(t, `urllib.request.urlopen("/api/x")`)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	c := calls[0]
	if c.Method != "GET" || c.URL != "/api/x" || c.Library != "urllib" {
		t.Errorf("call: got %+v", c)
	}
}

func TestSkipUnknownLibrary(t *testing.T) {
	calls := parseAPICalls(t, `my_custom_client.get("/x")`)
	if len(calls) != 0 {
		t.Errorf("want 0 calls for unknown library, got %d: %+v", len(calls), calls)
	}
}

func TestSkipTestClient(t *testing.T) {
	calls := parseAPICalls(t, `TestClient.get("/x")`)
	if len(calls) != 0 {
		t.Errorf("want 0 calls for TestClient, got %d: %+v", len(calls), calls)
	}
}

func TestSkipChainedClientPattern(t *testing.T) {
	// Deferred to v0.5 per M10.5 scope decision: only direct library calls
	// (requests.<method>, httpx.<method>, urllib.request.urlopen) are detected.
	calls := parseAPICalls(t, `httpx.AsyncClient().get("/x")`)
	if len(calls) != 0 {
		t.Errorf("want 0 calls for chained client pattern (out of scope for M10.5), got %d: %+v", len(calls), calls)
	}
}

func TestSkipUnknownMethodName(t *testing.T) {
	// requests.Session() is not an HTTP verb call.
	calls := parseAPICalls(t, `requests.Session()`)
	if len(calls) != 0 {
		t.Errorf("want 0 calls for non-verb method, got %d: %+v", len(calls), calls)
	}
}

func TestHighConfidenceLiteralURL(t *testing.T) {
	calls := parseAPICalls(t, `requests.get("/api/users")`)
	if len(calls) != 1 || calls[0].Confidence != "high" {
		t.Fatalf("want 1 high-confidence call, got %+v", calls)
	}
}

func TestMediumConfidenceVariableURL(t *testing.T) {
	calls := parseAPICalls(t, "URL = \"/api/users\"\nrequests.get(URL)\n")
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d: %+v", len(calls), calls)
	}
	if calls[0].URL != "URL" {
		t.Errorf("url: want %q (raw identifier text), got %q", "URL", calls[0].URL)
	}
	if calls[0].Confidence != "medium" {
		t.Errorf("confidence: want medium, got %q", calls[0].Confidence)
	}
}

func TestDetectFStringURL(t *testing.T) {
	calls := parseAPICalls(t, "requests.get(f\"/users/{user_id}\")")
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d: %+v", len(calls), calls)
	}
	if calls[0].URL != "/users/{user_id}" {
		t.Errorf("url: want %q, got %q", "/users/{user_id}", calls[0].URL)
	}
	if calls[0].Confidence != "high" {
		t.Errorf("confidence: want high, got %q", calls[0].Confidence)
	}
}

func TestAPICallsLineNumbers(t *testing.T) {
	src := "requests.get(\"/a\")\nhttpx.post(\"/b\")\n"
	calls := parseAPICalls(t, src)
	if len(calls) != 2 {
		t.Fatalf("want 2 calls, got %d", len(calls))
	}
	if calls[0].Line != 1 || calls[1].Line != 2 {
		t.Errorf("lines: want [1 2], got [%d %d]", calls[0].Line, calls[1].Line)
	}
}

func TestAPICallsNestedInFunction(t *testing.T) {
	src := "def get_all_users():\n    return requests.get(\"/users\").json()\n"
	calls := parseAPICalls(t, src)
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d: %+v", len(calls), calls)
	}
	if calls[0].URL != "/users" {
		t.Errorf("url: want /users, got %q", calls[0].URL)
	}
}

func TestAPICallsNoArgsSkipped(t *testing.T) {
	calls := parseAPICalls(t, `requests.get()`)
	if len(calls) != 0 {
		t.Errorf("want 0 calls for no-arg call, got %d: %+v", len(calls), calls)
	}
}
