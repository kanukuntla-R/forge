package analyzer_test

import (
	"encoding/json"
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

// enrichBoth runs both framework detectors' EnrichAnalysis, then both
// MatchAPICalls, mirroring the real pipeline order in pipeline.go — this is
// what makes cross-framework matching possible regardless of detector order.
func enrichBoth(t *testing.T, files []analyzer.FileInfo) (*analyzer.NextjsInfo, *analyzer.FastAPIAnalysis) {
	t.Helper()
	analysis := &analyzer.ProjectAnalysis{
		Project: analyzer.ProjectInfo{Root: "/fake", Name: "test", Languages: []string{}, Frameworks: []string{}},
		Files:   files,
	}
	detectors := []analyzer.FrameworkDetector{analyzer.NewNextjsDetector(), analyzer.NewFastAPIDetector()}
	for _, d := range detectors {
		if err := d.EnrichAnalysis(analysis); err != nil {
			t.Fatalf("EnrichAnalysis(%s): %v", d.Name(), err)
		}
	}
	for _, d := range detectors {
		if err := d.MatchAPICalls(analysis); err != nil {
			t.Fatalf("MatchAPICalls(%s): %v", d.Name(), err)
		}
	}

	var nextjs analyzer.NextjsInfo
	if raw, ok := analysis.Frameworks["nextjs"]; ok {
		data, _ := json.Marshal(raw)
		if err := json.Unmarshal(data, &nextjs); err != nil {
			t.Fatalf("unmarshaling NextjsInfo: %v", err)
		}
	}
	var fastapi analyzer.FastAPIAnalysis
	if raw, ok := analysis.Frameworks["fastapi"]; ok {
		data, _ := json.Marshal(raw)
		if err := json.Unmarshal(data, &fastapi); err != nil {
			t.Fatalf("unmarshaling FastAPIAnalysis: %v", err)
		}
	}
	return &nextjs, &fastapi
}

func fastapiServerFile(routeDecorator string) analyzer.FileInfo {
	return analyzer.FileInfo{
		Path:     "server.py",
		Language: "python",
		Imports:  fastapiImport(),
		Declarations: []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
			{Name: "handler", Type: "function", Line: 3, Decorators: []string{routeDecorator}},
		},
	}
}

// ── Cross-framework matching ────────────────────────────────────────────────

func TestNextJSCallMatchesFastAPIRoute(t *testing.T) {
	nextjs, _ := enrichBoth(t, []analyzer.FileInfo{
		fastapiServerFile(`app.get("/products")`),
		pageWithCalls("app/page.tsx", apiCall("http://localhost:8000/products", "GET", "fetch", "high")),
	})
	if len(nextjs.APICalls) != 1 {
		t.Fatalf("want 1 api_call, got %d: %+v", len(nextjs.APICalls), nextjs.APICalls)
	}
	c := nextjs.APICalls[0]
	if c.ToRoute != "/products" {
		t.Errorf("to_route: want /products, got %q", c.ToRoute)
	}
	if c.ToFramework != "fastapi" {
		t.Errorf("to_framework: want fastapi, got %q", c.ToFramework)
	}
	if c.Confidence != "high" {
		t.Errorf("confidence: want high (local dev host), got %q", c.Confidence)
	}
}

func TestPythonCallMatchesNextJSRoute(t *testing.T) {
	// A FastAPI file must be present somewhere in the project for the FastAPI
	// detector to activate at all (isFastAPIProject gates it) — realistic for
	// a hybrid project where client.py is a script alongside the backend.
	_, fastapi := enrichBoth(t, []analyzer.FileInfo{
		routeFile("app/api/cart/route.ts", "GET"),
		fastapiServerFile(`app.get("/health")`),
		{Path: "client.py", Language: "python", Calls: []analyzer.Call{
			httpCall("http://localhost:3000/api/cart", "GET", "requests", "high"),
		}},
	})
	if len(fastapi.APICalls) != 1 {
		t.Fatalf("want 1 api_call, got %d: %+v", len(fastapi.APICalls), fastapi.APICalls)
	}
	c := fastapi.APICalls[0]
	if c.ToRoute != "/api/cart" {
		t.Errorf("to_route: want /api/cart, got %q", c.ToRoute)
	}
	if c.ToFramework != "nextjs" {
		t.Errorf("to_framework: want nextjs, got %q", c.ToFramework)
	}
	if c.Confidence != "high" {
		t.Errorf("confidence: want high (local dev host), got %q", c.Confidence)
	}
}

func TestNextJSCallPrefersNextJSOverFastAPI(t *testing.T) {
	nextjs, _ := enrichBoth(t, []analyzer.FileInfo{
		routeFile("app/api/users/route.ts", "GET"),
		fastapiServerFile(`app.get("/api/users")`),
		pageWithCalls("app/page.tsx", apiCall("/api/users", "GET", "fetch", "high")),
	})
	if len(nextjs.APICalls) != 1 {
		t.Fatalf("want 1 api_call, got %d: %+v", len(nextjs.APICalls), nextjs.APICalls)
	}
	c := nextjs.APICalls[0]
	if c.ToRoute != "/api/users" {
		t.Errorf("to_route: want /api/users, got %q", c.ToRoute)
	}
	if c.ToFramework != "" {
		t.Errorf("to_framework: want empty (native match), got %q", c.ToFramework)
	}
}

// A TypeScript call's literal "{id}" is almost always a typo, not intentional
// templating (TS never produces brace syntax from interpolation — that's
// ":name"/":?"). It must not wildcard-match a FastAPI route's "{param}".
func TestTSLiteralBraceDoesNotWildcardFastAPIRoute(t *testing.T) {
	nextjs, _ := enrichBoth(t, []analyzer.FileInfo{
		fastapiServerFile(`app.get("/products/{product_id}")`),
		pageWithCalls("app/page.tsx", apiCall("/products/{id}", "GET", "fetch", "high")),
	})
	if len(nextjs.APICalls) != 1 {
		t.Fatalf("want 1 api_call, got %d", len(nextjs.APICalls))
	}
	if nextjs.APICalls[0].ToRoute != "" {
		t.Errorf("to_route: want unmatched (literal brace isn't a wildcard), got %+v", nextjs.APICalls[0])
	}
}

// A TS template-interpolation segment (":id") legitimately matches a
// FastAPI route's "{param}" placeholder.
func TestTSInterpolatedSegmentMatchesFastAPIParam(t *testing.T) {
	nextjs, _ := enrichBoth(t, []analyzer.FileInfo{
		fastapiServerFile(`app.get("/products/{product_id}")`),
		pageWithCalls("app/page.tsx", apiCall("http://localhost:8000/products/:id", "GET", "fetch", "interpolated")),
	})
	if len(nextjs.APICalls) != 1 || nextjs.APICalls[0].ToRoute != "/products/{product_id}" {
		t.Fatalf("want matched to /products/{product_id}, got %+v", nextjs.APICalls)
	}
	if nextjs.APICalls[0].ToFramework != "fastapi" {
		t.Errorf("to_framework: want fastapi, got %q", nextjs.APICalls[0].ToFramework)
	}
}

// ── URL normalization ────────────────────────────────────────────────────────

func TestNormalizeHTTP(t *testing.T) {
	if got := analyzer.NormalizeURL("http://localhost:8000/x"); got != "/x" {
		t.Errorf("got %q, want /x", got)
	}
}

func TestNormalizeHTTPS(t *testing.T) {
	if got := analyzer.NormalizeURL("https://localhost:8000/x"); got != "/x" {
		t.Errorf("got %q, want /x", got)
	}
}

func TestNormalizeRelativeUnchanged(t *testing.T) {
	if got := analyzer.NormalizeURL("/api/x"); got != "/api/x" {
		t.Errorf("got %q, want /api/x", got)
	}
}

func TestNormalizePreservesTrailingSlash(t *testing.T) {
	if got := analyzer.NormalizeURL("http://localhost:8000/users/"); got != "/users/" {
		t.Errorf("got %q, want /users/", got)
	}
}

func TestNormalizeRootPath(t *testing.T) {
	if got := analyzer.NormalizeURL("http://localhost:8000/"); got != "/" {
		t.Errorf("got %q, want /", got)
	}
}

func TestNormalizeNoPath(t *testing.T) {
	if got := analyzer.NormalizeURL("http://localhost:8000"); got != "/" {
		t.Errorf("got %q, want /", got)
	}
}

func TestNormalizeStripsQueryString(t *testing.T) {
	if got := analyzer.NormalizeURL("http://localhost:8000/products?limit=10"); got != "/products" {
		t.Errorf("got %q, want /products", got)
	}
}

func TestNormalizeOtherLocalDevHosts(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:5000/health": "/health",
		"http://0.0.0.0:8000/":         "/",
	}
	for in, want := range cases {
		if got := analyzer.NormalizeURL(in); got != want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// External hosts are left untouched — forge can't know whether that's part
// of this project, so it honestly reports no match rather than guessing.
func TestNormalizeExternalHostUntouched(t *testing.T) {
	url := "https://api.example.com/users"
	if got := analyzer.NormalizeURL(url); got != url {
		t.Errorf("got %q, want unchanged %q", got, url)
	}
}

// IPv6 dev hosts aren't in the recognized allowlist (localhost/127.0.0.1/
// 0.0.0.0 only) — kept simple per design decision, so they pass through
// untouched just like any other unrecognized host.
func TestNormalizeIPv6HostUntouched(t *testing.T) {
	url := "http://[::1]:8000/x"
	if got := analyzer.NormalizeURL(url); got != url {
		t.Errorf("got %q, want unchanged %q", got, url)
	}
}

func TestFullStackURLNormalization(t *testing.T) {
	cases := map[string]string{
		"http://localhost:8000/products/":    "/products/",
		"http://localhost:8000/products?x=1": "/products",
		"http://127.0.0.1:5000/health":       "/health",
		"http://0.0.0.0:8000/":               "/",
		"http://localhost:8000":              "/",
		"/api/cart":                          "/api/cart",
		"http://api.example.com/users":       "http://api.example.com/users",
	}
	for in, want := range cases {
		if got := analyzer.NormalizeURL(in); got != want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}
