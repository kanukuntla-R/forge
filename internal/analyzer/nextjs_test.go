package analyzer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func enrichNextjs(t *testing.T, files []analyzer.FileInfo) *analyzer.NextjsInfo {
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
	d := analyzer.NewNextjsDetector()
	if err := d.EnrichAnalysis(analysis); err != nil {
		t.Fatalf("EnrichAnalysis: %v", err)
	}
	if err := d.MatchAPICalls(analysis); err != nil {
		t.Fatalf("MatchAPICalls: %v", err)
	}
	raw, ok := analysis.Frameworks["nextjs"]
	if !ok {
		t.Fatal("frameworks.nextjs not set")
	}
	// Round-trip through JSON to get *NextjsInfo from map[string]any.
	data, _ := json.Marshal(raw)
	var info analyzer.NextjsInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("unmarshaling NextjsInfo: %v", err)
	}
	return &info
}

func tsFile(path string) analyzer.FileInfo {
	return analyzer.FileInfo{Path: path, Language: "typescript"}
}

func writeConfigFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// ── Detect tests ─────────────────────────────────────────────────────────────

func TestNextjsDetectByConfigFile(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, dir, "next.config.mjs", `export default {}`)
	d := analyzer.NewNextjsDetector()
	if !d.Detect(dir) {
		t.Error("want Detect=true for project with next.config.mjs")
	}
}

func TestNextjsDetectByPackageJSON(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies":{"next":"14.0.0","react":"18.0.0"}}`
	writeConfigFile(t, dir, "package.json", pkg)
	d := analyzer.NewNextjsDetector()
	if !d.Detect(dir) {
		t.Error("want Detect=true for project with next in dependencies")
	}
}

func TestNextjsDetectFalse(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, dir, "package.json", `{"dependencies":{"react":"18.0.0"}}`)
	d := analyzer.NewNextjsDetector()
	if d.Detect(dir) {
		t.Error("want Detect=false for non-Next.js project")
	}
}

// ── EnrichAnalysis tests ──────────────────────────────────────────────────────

func TestNextjsPagesExtracted(t *testing.T) {
	info := enrichNextjs(t, []analyzer.FileInfo{
		tsFile("app/page.tsx"),
		tsFile("app/about/page.tsx"),
		tsFile("app/dashboard/[id]/page.tsx"),
	})
	cases := []struct{ file, path string }{
		{"app/page.tsx", "/"},
		{"app/about/page.tsx", "/about"},
		{"app/dashboard/[id]/page.tsx", "/dashboard/:id"},
	}
	if len(info.Pages) != len(cases) {
		t.Fatalf("want %d pages, got %d", len(cases), len(info.Pages))
	}
	for i, c := range cases {
		if info.Pages[i].File != c.file {
			t.Errorf("page %d file: want %q, got %q", i, c.file, info.Pages[i].File)
		}
		if info.Pages[i].Path != c.path {
			t.Errorf("page %d path: want %q, got %q", i, c.path, info.Pages[i].Path)
		}
	}
}

func TestNextjsRoutesExtracted(t *testing.T) {
	info := enrichNextjs(t, []analyzer.FileInfo{
		{
			Path:     "app/api/users/route.ts",
			Language: "typescript",
			Exports:  []analyzer.Export{{Name: "GET"}, {Name: "POST"}, {Name: "handler"}},
		},
		{
			Path:     "app/api/products/[id]/route.ts",
			Language: "typescript",
			Exports:  []analyzer.Export{{Name: "GET"}},
		},
	})
	if len(info.Routes) != 2 {
		t.Fatalf("want 2 routes, got %d", len(info.Routes))
	}
	if info.Routes[0].Path != "/api/users" {
		t.Errorf("route 0 path: want /api/users, got %q", info.Routes[0].Path)
	}
	if !sliceContains(info.Routes[0].Methods, "GET") || !sliceContains(info.Routes[0].Methods, "POST") {
		t.Errorf("route 0 methods: want [GET POST], got %v", info.Routes[0].Methods)
	}
	if sliceContains(info.Routes[0].Methods, "handler") {
		t.Error("non-HTTP export 'handler' must not appear in methods")
	}
	if info.Routes[1].Path != "/api/products/:id" {
		t.Errorf("route 1 path: want /api/products/:id, got %q", info.Routes[1].Path)
	}
}

func TestNextjsRouteOutsideApiDir(t *testing.T) {
	info := enrichNextjs(t, []analyzer.FileInfo{
		{
			Path:     "app/dashboard/route.ts",
			Language: "typescript",
			Exports:  []analyzer.Export{{Name: "GET"}},
		},
	})
	if len(info.Routes) != 1 {
		t.Fatalf("want 1 route for app/dashboard/route.ts, got %d", len(info.Routes))
	}
	if info.Routes[0].Path != "/dashboard" {
		t.Errorf("route path: want /dashboard, got %q", info.Routes[0].Path)
	}
}

func TestNextjsLayoutsExtracted(t *testing.T) {
	info := enrichNextjs(t, []analyzer.FileInfo{
		tsFile("app/layout.tsx"),
		tsFile("app/about/layout.tsx"),
	})
	if len(info.Layouts) != 2 {
		t.Fatalf("want 2 layouts, got %d", len(info.Layouts))
	}
	if info.Layouts[0].File != "app/layout.tsx" {
		t.Errorf("layout 0: want app/layout.tsx, got %q", info.Layouts[0].File)
	}
}

func TestNextjsComponentsExtracted(t *testing.T) {
	info := enrichNextjs(t, []analyzer.FileInfo{
		tsFile("components/UserCard.tsx"),
		tsFile("components/ui/button.tsx"),
		tsFile("app/page.tsx"), // not a component
	})
	if len(info.Components) != 2 {
		t.Fatalf("want 2 components, got %d", len(info.Components))
	}
	if info.Components[0].Name != "UserCard" {
		t.Errorf("component 0 name: want UserCard, got %q", info.Components[0].Name)
	}
	if info.Components[1].Name != "button" {
		t.Errorf("component 1 name: want button, got %q", info.Components[1].Name)
	}
}

func TestNextjsComponentUsedByPopulated(t *testing.T) {
	info := enrichNextjs(t, []analyzer.FileInfo{
		{
			Path:     "components/Card.tsx",
			Language: "typescript",
		},
		{
			Path:     "app/page.tsx",
			Language: "typescript",
			Imports: []analyzer.Import{
				{Source: "@/components/Card", Resolved: "components/Card.tsx", Names: []string{"Card"}},
			},
		},
	})
	if len(info.Components) != 1 {
		t.Fatalf("want 1 component, got %d", len(info.Components))
	}
	if len(info.Components[0].UsedBy) != 1 || info.Components[0].UsedBy[0] != "app/page.tsx" {
		t.Errorf("UsedBy: want [app/page.tsx], got %v", info.Components[0].UsedBy)
	}
}

func TestNextjsDynamicSegments(t *testing.T) {
	info := enrichNextjs(t, []analyzer.FileInfo{
		tsFile("app/blog/[slug]/page.tsx"),
		tsFile("app/users/[id]/posts/[postId]/page.tsx"),
	})
	if len(info.Pages) != 2 {
		t.Fatalf("want 2 pages, got %d", len(info.Pages))
	}
	if info.Pages[0].Path != "/blog/:slug" {
		t.Errorf("want /blog/:slug, got %q", info.Pages[0].Path)
	}
	if info.Pages[1].Path != "/users/:id/posts/:postId" {
		t.Errorf("want /users/:id/posts/:postId, got %q", info.Pages[1].Path)
	}
}

func TestNextjsRootPage(t *testing.T) {
	info := enrichNextjs(t, []analyzer.FileInfo{tsFile("app/page.tsx")})
	if len(info.Pages) != 1 {
		t.Fatalf("want 1 page, got %d", len(info.Pages))
	}
	if info.Pages[0].Path != "/" {
		t.Errorf("root page path: want /, got %q", info.Pages[0].Path)
	}
}

func TestNextjsProjectFrameworksUpdated(t *testing.T) {
	analysis := &analyzer.ProjectAnalysis{
		Project: analyzer.ProjectInfo{
			Frameworks: []string{},
		},
		Files: []analyzer.FileInfo{tsFile("app/page.tsx")},
	}
	d := analyzer.NewNextjsDetector()
	if err := d.EnrichAnalysis(analysis); err != nil {
		t.Fatalf("EnrichAnalysis: %v", err)
	}
	found := false
	for _, f := range analysis.Project.Frameworks {
		if f == "nextjs" {
			found = true
		}
	}
	if !found {
		t.Errorf("want nextjs in Project.Frameworks, got %v", analysis.Project.Frameworks)
	}
}

// ── API call matching tests ────────────────────────────────────────────────────

func routeFile(path string, exports ...string) analyzer.FileInfo {
	f := analyzer.FileInfo{Path: path, Language: "typescript"}
	for _, exp := range exports {
		f.Exports = append(f.Exports, analyzer.Export{Name: exp})
	}
	return f
}

func pageWithCalls(path string, calls ...analyzer.Call) analyzer.FileInfo {
	return analyzer.FileInfo{Path: path, Language: "typescript", Calls: calls}
}

func apiCall(target, method, kind, conf string) analyzer.Call {
	return analyzer.Call{Target: target, Method: method, Kind: kind, Confidence: conf}
}

func TestNextjsAPICallExactMatch(t *testing.T) {
	info := enrichNextjs(t, []analyzer.FileInfo{
		routeFile("app/api/users/route.ts", "GET", "POST"),
		pageWithCalls("app/page.tsx", apiCall("/api/users", "GET", "fetch", "high")),
	})
	if len(info.APICalls) != 1 {
		t.Fatalf("want 1 api_call, got %d", len(info.APICalls))
	}
	c := info.APICalls[0]
	if c.FromFile != "app/page.tsx" {
		t.Errorf("from_file: want app/page.tsx, got %q", c.FromFile)
	}
	if c.ToRoute != "/api/users" {
		t.Errorf("to_route: want /api/users, got %q", c.ToRoute)
	}
	if c.Confidence != "high" {
		t.Errorf("confidence: want high, got %q", c.Confidence)
	}
}

func TestNextjsAPICallPatternMatch(t *testing.T) {
	info := enrichNextjs(t, []analyzer.FileInfo{
		routeFile("app/api/products/[id]/route.ts", "GET"),
		pageWithCalls("app/page.tsx", apiCall("/api/products/123", "GET", "fetch", "high")),
	})
	if len(info.APICalls) != 1 {
		t.Fatalf("want 1 api_call, got %d", len(info.APICalls))
	}
	c := info.APICalls[0]
	if c.ToRoute != "/api/products/:id" {
		t.Errorf("to_route: want /api/products/:id, got %q", c.ToRoute)
	}
	if c.Confidence != "medium" {
		t.Errorf("confidence: want medium (pattern), got %q", c.Confidence)
	}
}

func TestNextjsAPICallNoMatch(t *testing.T) {
	info := enrichNextjs(t, []analyzer.FileInfo{
		routeFile("app/api/users/route.ts", "GET"),
		pageWithCalls("app/page.tsx", apiCall("/api/missing", "GET", "fetch", "high")),
	})
	if len(info.APICalls) != 1 {
		t.Fatalf("want 1 api_call, got %d", len(info.APICalls))
	}
	c := info.APICalls[0]
	if c.ToRoute != "" {
		t.Errorf("to_route: want empty for unmatched call, got %q", c.ToRoute)
	}
	if c.Confidence != "medium" {
		t.Errorf("confidence: want medium (known kind, no match), got %q", c.Confidence)
	}
}

func TestNextjsAPICallHeuristicMatch(t *testing.T) {
	info := enrichNextjs(t, []analyzer.FileInfo{
		routeFile("app/api/users/route.ts", "GET"),
		pageWithCalls("app/page.tsx", apiCall("/api/users", "GET", "heuristic", "medium")),
	})
	if len(info.APICalls) != 1 {
		t.Fatalf("want 1 api_call, got %d", len(info.APICalls))
	}
	c := info.APICalls[0]
	if c.ToRoute != "/api/users" {
		t.Errorf("to_route: want /api/users, got %q", c.ToRoute)
	}
	if c.Confidence != "medium" {
		t.Errorf("confidence: want medium (heuristic + match), got %q", c.Confidence)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func sliceContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
