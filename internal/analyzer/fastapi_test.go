package analyzer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func enrichFastAPI(t *testing.T, files []analyzer.FileInfo) *analyzer.FastAPIAnalysis {
	t.Helper()
	analysis := &analyzer.ProjectAnalysis{
		Project: analyzer.ProjectInfo{
			Root:       "/fake",
			Name:       "test",
			Languages:  []string{},
			Frameworks: []string{},
		},
		Files: files,
	}
	d := analyzer.NewFastAPIDetector()
	if err := d.EnrichAnalysis(analysis); err != nil {
		t.Fatalf("EnrichAnalysis: %v", err)
	}
	raw, ok := analysis.Frameworks["fastapi"]
	if !ok {
		t.Fatal("frameworks.fastapi not set")
	}
	data, _ := json.Marshal(raw)
	var info analyzer.FastAPIAnalysis
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("unmarshaling FastAPIAnalysis: %v", err)
	}
	return &info
}

func pyFile(path string, imports []analyzer.Import, decls []analyzer.Declaration, calls []analyzer.ModuleCall) analyzer.FileInfo {
	return analyzer.FileInfo{
		Path:         path,
		Language:     "python",
		Imports:      imports,
		Declarations: decls,
		ModuleCalls:  calls,
	}
}

func fastapiImport() []analyzer.Import {
	return []analyzer.Import{{Source: "fastapi", Names: []string{"FastAPI"}, External: true}}
}

// ── Detect (on-disk) ─────────────────────────────────────────────────────────

func TestFastAPIDetectTrue(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.py"), []byte("from fastapi import FastAPI\napp = FastAPI()\n"), 0644); err != nil {
		t.Fatal(err)
	}
	d := analyzer.NewFastAPIDetector()
	if !d.Detect(dir) {
		t.Error("want Detect=true for project importing fastapi")
	}
}

func TestFastAPIDetectFalse(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('hello')\n"), 0644); err != nil {
		t.Fatal(err)
	}
	d := analyzer.NewFastAPIDetector()
	if d.Detect(dir) {
		t.Error("want Detect=false for project without fastapi")
	}
}

func TestEnrichAnalysis_NotFastAPIProject(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("main.py", []analyzer.Import{{Source: "os", External: true}}, nil, nil),
	}
	analysis := &analyzer.ProjectAnalysis{Files: files}
	d := analyzer.NewFastAPIDetector()
	if err := d.EnrichAnalysis(analysis); err != nil {
		t.Fatalf("EnrichAnalysis: %v", err)
	}
	if _, ok := analysis.Frameworks["fastapi"]; ok {
		t.Error("want frameworks.fastapi unset for non-FastAPI project")
	}
}

// ── App detection ────────────────────────────────────────────────────────────

func TestDetectApp(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("main.py", fastapiImport(), []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 3, ValueRepr: "FastAPI()"},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Apps) != 1 {
		t.Fatalf("want 1 app, got %d: %+v", len(info.Apps), info.Apps)
	}
	if info.Apps[0].Name != "app" || info.Apps[0].File != "main.py" || info.Apps[0].Line != 3 {
		t.Errorf("app: got %+v", info.Apps[0])
	}
}

func TestDetectMultipleApps(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("a.py", fastapiImport(), []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
		}, nil),
		pyFile("b.py", nil, []analyzer.Declaration{
			{Name: "app2", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Apps) != 2 {
		t.Fatalf("want 2 apps, got %d: %+v", len(info.Apps), info.Apps)
	}
}

func TestDetectNamedApp(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("main.py", fastapiImport(), []analyzer.Declaration{
			{Name: "application", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Apps) != 1 || info.Apps[0].Name != "application" {
		t.Fatalf("want app named application, got %+v", info.Apps)
	}
}

// ── Router detection ─────────────────────────────────────────────────────────

func TestDetectRouterNoPrefix(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("routers/users.py", fastapiImport(), []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: "APIRouter()"},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routers) != 1 {
		t.Fatalf("want 1 router, got %d: %+v", len(info.Routers), info.Routers)
	}
	if info.Routers[0].Prefix != "" {
		t.Errorf("prefix: want empty, got %q", info.Routers[0].Prefix)
	}
}

func TestDetectRouterWithLiteralPrefix(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("routers/users.py", fastapiImport(), []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: `APIRouter(prefix="/users")`},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routers) != 1 {
		t.Fatalf("want 1 router, got %d", len(info.Routers))
	}
	if info.Routers[0].Prefix != "/users" {
		t.Errorf("prefix: want %q, got %q", "/users", info.Routers[0].Prefix)
	}
	if info.Routers[0].Confidence != "high" {
		t.Errorf("confidence: want high, got %q", info.Routers[0].Confidence)
	}
}

func TestDetectRouterWithVariablePrefix(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("routers/users.py", fastapiImport(), []analyzer.Declaration{
			{Name: "PREFIX", Type: "variable", Line: 1, ValueRepr: `"/api"`},
			{Name: "router", Type: "variable", Line: 2, ValueRepr: "APIRouter(prefix=PREFIX)"},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routers) != 1 {
		t.Fatalf("want 1 router, got %d", len(info.Routers))
	}
	if info.Routers[0].Prefix != "/api" {
		t.Errorf("prefix: want %q, got %q", "/api", info.Routers[0].Prefix)
	}
	if info.Routers[0].Confidence != "medium" {
		t.Errorf("confidence: want medium, got %q", info.Routers[0].Confidence)
	}
}

func TestDetectRouterWithUnresolvableVariable(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("routers/users.py", fastapiImport(), []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: "APIRouter(prefix=UNDEFINED_PREFIX)"},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routers) != 1 {
		t.Fatalf("want 1 router, got %d", len(info.Routers))
	}
	if info.Routers[0].Prefix != "UNDEFINED_PREFIX" {
		t.Errorf("prefix: want %q, got %q", "UNDEFINED_PREFIX", info.Routers[0].Prefix)
	}
	if info.Routers[0].Confidence != "low" {
		t.Errorf("confidence: want low, got %q", info.Routers[0].Confidence)
	}
}

// ── Route detection ──────────────────────────────────────────────────────────

func TestDetectSimpleRoute(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("main.py", fastapiImport(), []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
			{Name: "health", Type: "function", Line: 3, Decorators: []string{`app.get("/health")`}},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routes) != 1 {
		t.Fatalf("want 1 route, got %d: %+v", len(info.Routes), info.Routes)
	}
	r := info.Routes[0]
	if r.Method != "GET" || r.Path != "/health" || r.Handler != "health" {
		t.Errorf("route: got %+v", r)
	}
	if !r.Reachable {
		t.Error("want app-direct route reachable=true")
	}
}

func TestDetectAllHTTPMethods(t *testing.T) {
	methods := []string{"get", "post", "put", "delete", "patch"}
	var decls []analyzer.Declaration
	decls = append(decls, analyzer.Declaration{Name: "app", Type: "variable", Line: 1, ValueRepr: "FastAPI()"})
	for i, m := range methods {
		decls = append(decls, analyzer.Declaration{
			Name: "h" + m, Type: "function", Line: i + 2,
			Decorators: []string{"app." + m + `("/x")`},
		})
	}
	files := []analyzer.FileInfo{pyFile("main.py", fastapiImport(), decls, nil)}
	info := enrichFastAPI(t, files)
	if len(info.Routes) != len(methods) {
		t.Fatalf("want %d routes, got %d: %+v", len(methods), len(info.Routes), info.Routes)
	}
	for i, m := range methods {
		want := strings.ToUpper(m)
		if info.Routes[i].Method != want {
			t.Errorf("route %d method: want %q, got %q", i, want, info.Routes[i].Method)
		}
	}
}

func TestDetectAsyncRoute(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("main.py", fastapiImport(), []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
			{Name: "fetch", Type: "async_function", Line: 3, Decorators: []string{`app.get("/x")`}},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routes) != 1 || !info.Routes[0].IsAsync {
		t.Fatalf("want 1 async route, got %+v", info.Routes)
	}
}

func TestDetectRouteOnRouter(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("routers/users.py", fastapiImport(), []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: `APIRouter(prefix="/users")`},
			{Name: "list_users", Type: "function", Line: 3, Decorators: []string{`router.get("/x")`}},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routes) != 1 {
		t.Fatalf("want 1 route, got %d", len(info.Routes))
	}
	if info.Routes[0].Path != "/users/x" {
		t.Errorf("path: want %q, got %q", "/users/x", info.Routes[0].Path)
	}
}

func TestDetectRouteWithPathParams(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("routers/users.py", fastapiImport(), []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: `APIRouter(prefix="/users")`},
			{Name: "get_user", Type: "function", Line: 3, Decorators: []string{`router.get("/{id}")`}},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routes) != 1 || info.Routes[0].Path != "/users/{id}" {
		t.Fatalf("want /users/{id}, got %+v", info.Routes)
	}
}

func TestSkipNonRouteDecorators(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("main.py", fastapiImport(), []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
			{Name: "foo", Type: "function", Line: 3, Decorators: []string{"requires_auth"}},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routes) != 0 {
		t.Errorf("want 0 routes, got %d: %+v", len(info.Routes), info.Routes)
	}
}

func TestDetectAPIRouteWithMethods(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("main.py", fastapiImport(), []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
			{Name: "handler", Type: "function", Line: 3, Decorators: []string{`app.api_route("/x", methods=["GET", "POST"])`}},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routes) != 2 {
		t.Fatalf("want 2 routes, got %d: %+v", len(info.Routes), info.Routes)
	}
	if info.Routes[0].Method != "GET" || info.Routes[1].Method != "POST" {
		t.Errorf("methods: got %q, %q", info.Routes[0].Method, info.Routes[1].Method)
	}
	if info.Routes[0].Path != "/x" || info.Routes[1].Path != "/x" {
		t.Errorf("paths: got %q, %q", info.Routes[0].Path, info.Routes[1].Path)
	}
}

// ── Prefix + path combining ──────────────────────────────────────────────────

func TestPrefixPathCombining_TrailingSlash(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("routers/users.py", fastapiImport(), []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: `APIRouter(prefix="/users/")`},
			{Name: "get_user", Type: "function", Line: 3, Decorators: []string{`router.get("/{id}")`}},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routes) != 1 || info.Routes[0].Path != "/users/{id}" {
		t.Fatalf("want /users/{id}, got %+v", info.Routes)
	}
}

func TestPrefixPathCombining_RootPath(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("routers/users.py", fastapiImport(), []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: `APIRouter(prefix="/users")`},
			{Name: "list_users", Type: "function", Line: 3, Decorators: []string{`router.get("/")`}},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routes) != 1 || info.Routes[0].Path != "/users/" {
		t.Fatalf("want /users/, got %+v", info.Routes)
	}
}

// ── Reachability ─────────────────────────────────────────────────────────────

func TestReachableRouter_IncludeRouterSameFile(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("main.py", fastapiImport(), []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
			{Name: "router", Type: "variable", Line: 2, ValueRepr: `APIRouter(prefix="/users")`},
		}, []analyzer.ModuleCall{
			{FunctionRepr: "app.include_router", ArgsRepr: "router", Line: 4},
		}),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routers) != 1 || !info.Routers[0].Reachable {
		t.Fatalf("want router reachable, got %+v", info.Routers)
	}
}

func TestReachableRouter_IncludeRouterCrossFile(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("main.py", fastapiImport(), []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
		}, []analyzer.ModuleCall{
			{FunctionRepr: "app.include_router", ArgsRepr: "users.router", Line: 3},
		}),
		pyFile("routers/users.py", nil, []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: `APIRouter(prefix="/users")`},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routers) != 1 || !info.Routers[0].Reachable {
		t.Fatalf("want router reachable (cross-file), got %+v", info.Routers)
	}
}

func TestReachableRouter_SameNameDifferentFilesDisambiguatedByImport(t *testing.T) {
	// Realistic pattern: every router submodule names its variable "router".
	// Resolution must use the import that brought in "users"/"health" to tell
	// them apart, not just match the first router named "router" it finds.
	mainImports := append(fastapiImport(),
		analyzer.Import{Source: "src.routes", Names: []string{"health"}, External: true},
		analyzer.Import{Source: "src.routes", Names: []string{"users"}, External: true},
	)
	files := []analyzer.FileInfo{
		pyFile("main.py", mainImports, []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
		}, []analyzer.ModuleCall{
			{FunctionRepr: "app.include_router", ArgsRepr: "health.router", Line: 3},
			{FunctionRepr: "app.include_router", ArgsRepr: `users.router, prefix="/users"`, Line: 4},
		}),
		pyFile("src/routes/health.py", nil, []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: "APIRouter()"},
			{Name: "health_check", Type: "async_function", Line: 3, Decorators: []string{`router.get("/health")`}},
		}, nil),
		pyFile("src/routes/users.py", nil, []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: "APIRouter()"},
			{Name: "list_users", Type: "async_function", Line: 3, Decorators: []string{`router.get("/")`}},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routers) != 2 {
		t.Fatalf("want 2 routers, got %d: %+v", len(info.Routers), info.Routers)
	}
	byFile := map[string]analyzer.FastAPIRouter{}
	for _, r := range info.Routers {
		byFile[r.File] = r
	}
	health, ok := byFile["src/routes/health.py"]
	if !ok || health.Prefix != "" || !health.Reachable {
		t.Errorf("health router: want prefix=\"\" reachable=true, got %+v", health)
	}
	users, ok := byFile["src/routes/users.py"]
	if !ok || users.Prefix != "/users" || !users.Reachable {
		t.Errorf("users router: want prefix=/users reachable=true, got %+v", users)
	}

	byFileRoute := map[string]string{}
	for _, r := range info.Routes {
		byFileRoute[r.File] = r.Path
	}
	if byFileRoute["src/routes/health.py"] != "/health" {
		t.Errorf("health route path: want /health, got %q", byFileRoute["src/routes/health.py"])
	}
	if byFileRoute["src/routes/users.py"] != "/users/" {
		t.Errorf("users route path: want /users/, got %q", byFileRoute["src/routes/users.py"])
	}
}

func TestRouterPrefixFromIncludeRouterCall(t *testing.T) {
	// Real-world pattern (matches the python-fastapi blueprint): the router itself
	// has no prefix, and the prefix is supplied at the include_router() call site.
	files := []analyzer.FileInfo{
		pyFile("main.py", fastapiImport(), []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
		}, []analyzer.ModuleCall{
			{FunctionRepr: "app.include_router", ArgsRepr: `users.router, prefix="/users", tags=["users"]`, Line: 3},
		}),
		pyFile("routers/users.py", nil, []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: "APIRouter()"},
			{Name: "list_users", Type: "async_function", Line: 3, Decorators: []string{`router.get("/")`}},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routers) != 1 {
		t.Fatalf("want 1 router, got %d: %+v", len(info.Routers), info.Routers)
	}
	if info.Routers[0].Prefix != "/users" {
		t.Errorf("prefix: want %q, got %q", "/users", info.Routers[0].Prefix)
	}
	if info.Routers[0].Confidence != "high" {
		t.Errorf("confidence: want high, got %q", info.Routers[0].Confidence)
	}
	if !info.Routers[0].Reachable {
		t.Error("want router reachable")
	}
	if len(info.Routes) != 1 || info.Routes[0].Path != "/users/" {
		t.Fatalf("want route path /users/, got %+v", info.Routes)
	}
}

func TestRouterOwnPrefixTakesPrecedenceOverIncludeRouter(t *testing.T) {
	// If the router already declares its own prefix, include_router()'s prefix
	// (if any) must not override it.
	files := []analyzer.FileInfo{
		pyFile("main.py", fastapiImport(), []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
		}, []analyzer.ModuleCall{
			{FunctionRepr: "app.include_router", ArgsRepr: `users.router, prefix="/v2/users"`, Line: 3},
		}),
		pyFile("routers/users.py", nil, []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: `APIRouter(prefix="/users")`},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routers) != 1 || info.Routers[0].Prefix != "/users" {
		t.Fatalf("want router's own prefix /users preserved, got %+v", info.Routers)
	}
}

func TestUnreachableRouter(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("routers/users.py", fastapiImport(), []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: `APIRouter(prefix="/users")`},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routers) != 1 || info.Routers[0].Reachable {
		t.Fatalf("want router unreachable, got %+v", info.Routers)
	}
}

func TestRoutesInheritReachability(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("main.py", fastapiImport(), []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 1, ValueRepr: "FastAPI()"},
			{Name: "router", Type: "variable", Line: 2, ValueRepr: `APIRouter(prefix="/users")`},
			{Name: "list_users", Type: "function", Line: 5, Decorators: []string{`router.get("/")`}},
		}, []analyzer.ModuleCall{
			{FunctionRepr: "app.include_router", ArgsRepr: "router", Line: 4},
		}),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routes) != 1 || !info.Routes[0].Reachable {
		t.Fatalf("want route reachable, got %+v", info.Routes)
	}
}

func TestUnreachableRouterRoutesNotReachable(t *testing.T) {
	files := []analyzer.FileInfo{
		pyFile("routers/users.py", fastapiImport(), []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 1, ValueRepr: `APIRouter(prefix="/users")`},
			{Name: "list_users", Type: "function", Line: 3, Decorators: []string{`router.get("/")`}},
		}, nil),
	}
	info := enrichFastAPI(t, files)
	if len(info.Routes) != 1 || info.Routes[0].Reachable {
		t.Fatalf("want route unreachable, got %+v", info.Routes)
	}
}

// ── Integration ──────────────────────────────────────────────────────────────

func TestRealisticFastAPIProject(t *testing.T) {
	mainImports := append(fastapiImport(),
		analyzer.Import{Source: "routers", Names: []string{"users"}, External: true},
		analyzer.Import{Source: "routers", Names: []string{"auth"}, External: true},
	)
	files := []analyzer.FileInfo{
		pyFile("main.py", mainImports, []analyzer.Declaration{
			{Name: "app", Type: "variable", Line: 3, ValueRepr: "FastAPI()"},
			{Name: "health", Type: "function", Line: 5, Decorators: []string{`app.get("/health")`}},
		}, []analyzer.ModuleCall{
			{FunctionRepr: "app.include_router", ArgsRepr: "users.router", Line: 7},
			{FunctionRepr: "app.include_router", ArgsRepr: "auth.router", Line: 8},
		}),
		pyFile("routers/users.py", nil, []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 3, ValueRepr: `APIRouter(prefix="/users")`},
			{Name: "list_users", Type: "async_function", Line: 5, Decorators: []string{`router.get("/")`}},
			{Name: "get_user", Type: "async_function", Line: 9, Decorators: []string{`router.get("/{user_id}")`}},
			{Name: "create_user", Type: "async_function", Line: 13, Decorators: []string{`router.post("/")`}},
		}, nil),
		pyFile("routers/auth.py", nil, []analyzer.Declaration{
			{Name: "router", Type: "variable", Line: 3, ValueRepr: `APIRouter(prefix="/auth")`},
			{Name: "register", Type: "async_function", Line: 5, Decorators: []string{`router.post("/register")`}},
			{Name: "login", Type: "async_function", Line: 9, Decorators: []string{`router.post("/login")`}},
		}, nil),
	}
	info := enrichFastAPI(t, files)

	if len(info.Apps) != 1 {
		t.Fatalf("want 1 app, got %d: %+v", len(info.Apps), info.Apps)
	}
	if len(info.Routers) != 2 {
		t.Fatalf("want 2 routers, got %d: %+v", len(info.Routers), info.Routers)
	}
	for _, r := range info.Routers {
		if !r.Reachable {
			t.Errorf("router %s: want reachable, got unreachable", r.Name)
		}
	}
	if len(info.Routes) != 6 {
		t.Fatalf("want 6 routes, got %d: %+v", len(info.Routes), info.Routes)
	}
	for _, r := range info.Routes {
		if !r.Reachable {
			t.Errorf("route %s %s: want reachable", r.Method, r.Path)
		}
	}

	wantPaths := map[string]bool{
		"/health": false, "/users/": false, "/users/{user_id}": false,
		"/auth/register": false, "/auth/login": false,
	}
	for _, r := range info.Routes {
		if _, ok := wantPaths[r.Path]; ok {
			wantPaths[r.Path] = true
		}
	}
	for p, seen := range wantPaths {
		if !seen {
			t.Errorf("expected path %q not found in routes: %+v", p, info.Routes)
		}
	}
}
