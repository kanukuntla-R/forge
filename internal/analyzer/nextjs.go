package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// NextjsInfo holds all Next.js App Router structural data extracted from the analysis.
type NextjsInfo struct {
	Pages      []NextjsPage      `json:"pages"`
	Routes     []NextjsRoute     `json:"routes"`
	Layouts    []NextjsLayout    `json:"layouts"`
	Components []NextjsComponent `json:"components"`
}

// NextjsPage represents a page in the App Router (app/**/page.{tsx,ts,jsx,js}).
type NextjsPage struct {
	ID   string `json:"id"`
	Path string `json:"path"` // URL path with :param transformations
	File string `json:"file"`
}

// NextjsRoute represents an API route handler (app/**/route.{ts,js}).
type NextjsRoute struct {
	ID      string   `json:"id"`
	Path    string   `json:"path"`
	File    string   `json:"file"`
	Methods []string `json:"methods"`
}

// NextjsLayout represents a layout file (app/**/layout.{tsx,ts,jsx,js}).
type NextjsLayout struct {
	ID   string `json:"id"`
	File string `json:"file"`
}

// NextjsComponent represents a file in the components/ directory.
type NextjsComponent struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	File   string   `json:"file"`
	UsedBy []string `json:"used_by"`
}

// nextjsDetector implements FrameworkDetector for Next.js App Router projects.
type nextjsDetector struct{}

// NewNextjsDetector returns a FrameworkDetector for Next.js.
func NewNextjsDetector() FrameworkDetector {
	return &nextjsDetector{}
}

func (d *nextjsDetector) Name() string { return "nextjs" }

// Detect returns true if the project is a Next.js project.
// Checks for next.config.{js,mjs,ts} or "next" in package.json dependencies.
func (d *nextjsDetector) Detect(projectRoot string) bool {
	for _, name := range []string{"next.config.js", "next.config.mjs", "next.config.ts"} {
		if _, err := os.Stat(filepath.Join(projectRoot, name)); err == nil {
			return true
		}
	}
	return hasNextInPackageJSON(filepath.Join(projectRoot, "package.json"))
}

// EnrichAnalysis populates analysis.Frameworks["nextjs"] by scanning the file list
// for App Router conventions (pages, routes, layouts, components).
func (d *nextjsDetector) EnrichAnalysis(analysis *ProjectAnalysis) error {
	info := &NextjsInfo{
		Pages:      []NextjsPage{},
		Routes:     []NextjsRoute{},
		Layouts:    []NextjsLayout{},
		Components: []NextjsComponent{},
	}

	for _, file := range analysis.Files {
		switch {
		case isNextjsPage(file.Path):
			info.Pages = append(info.Pages, NextjsPage{
				ID:   file.Path,
				Path: appDirURL(file.Path),
				File: file.Path,
			})
		case isNextjsRoute(file.Path):
			info.Routes = append(info.Routes, NextjsRoute{
				ID:      file.Path,
				Path:    appDirURL(file.Path),
				File:    file.Path,
				Methods: routeHTTPMethods(file),
			})
		case isNextjsLayout(file.Path):
			info.Layouts = append(info.Layouts, NextjsLayout{
				ID:   file.Path,
				File: file.Path,
			})
		case isNextjsComponent(file.Path):
			info.Components = append(info.Components, NextjsComponent{
				ID:     file.Path,
				Name:   componentBaseName(file.Path),
				File:   file.Path,
				UsedBy: []string{},
			})
		}
	}

	info.Components = computeComponentUsage(analysis.Files, info.Components)

	if analysis.Frameworks == nil {
		analysis.Frameworks = make(map[string]any)
	}
	analysis.Frameworks["nextjs"] = info
	analysis.Project.Frameworks = append(analysis.Project.Frameworks, "nextjs")
	return nil
}

// ── Classification helpers ──────────────────────────────────────────────────

var nextjsPageNames = map[string]bool{
	"page.tsx": true, "page.ts": true, "page.jsx": true, "page.js": true,
}
var nextjsLayoutNames = map[string]bool{
	"layout.tsx": true, "layout.ts": true, "layout.jsx": true, "layout.js": true,
}
var nextjsRouteNames = map[string]bool{
	"route.ts": true, "route.js": true,
}

func pathBaseName(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return path
	}
	return path[idx+1:]
}

func isNextjsPage(path string) bool {
	return strings.HasPrefix(path, "app/") && nextjsPageNames[pathBaseName(path)]
}

func isNextjsLayout(path string) bool {
	return strings.HasPrefix(path, "app/") && nextjsLayoutNames[pathBaseName(path)]
}

// isNextjsRoute matches any route.ts/route.js under app/ — not restricted to app/api/
// because Next.js allows route handlers anywhere in the app directory.
func isNextjsRoute(path string) bool {
	return strings.HasPrefix(path, "app/") && nextjsRouteNames[pathBaseName(path)]
}

func isNextjsComponent(path string) bool {
	return strings.HasPrefix(path, "components/") && isTSExtension(path)
}

func isTSExtension(path string) bool {
	return strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") ||
		strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".jsx")
}

// ── URL path transformation ─────────────────────────────────────────────────

// appDirURL returns the URL path for any file under app/.
// It strips "app/" and the filename, then transforms [segment] → :segment.
//   - app/page.tsx                   → "/"
//   - app/about/page.tsx             → "/about"
//   - app/dashboard/[id]/page.tsx    → "/dashboard/:id"
//   - app/api/users/route.ts         → "/api/users"
//   - app/api/products/[id]/route.ts → "/api/products/:id"
//   - app/dashboard/route.ts         → "/dashboard"
func appDirURL(filePath string) string {
	rel := strings.TrimPrefix(filePath, "app/")
	dir := ""
	if idx := strings.LastIndex(rel, "/"); idx >= 0 {
		dir = rel[:idx]
	}
	if dir == "" {
		return "/"
	}
	return "/" + transformDynamicSegments(dir)
}

func transformDynamicSegments(dir string) string {
	segs := strings.Split(dir, "/")
	for i, seg := range segs {
		if strings.HasPrefix(seg, "[") && strings.HasSuffix(seg, "]") {
			segs[i] = ":" + seg[1:len(seg)-1]
		}
	}
	return strings.Join(segs, "/")
}

// ── Route method extraction ─────────────────────────────────────────────────

var httpMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true,
	"DELETE": true, "OPTIONS": true, "HEAD": true,
}

func routeHTTPMethods(file FileInfo) []string {
	var methods []string
	for _, exp := range file.Exports {
		if httpMethods[exp.Name] {
			methods = append(methods, exp.Name)
		}
	}
	if methods == nil {
		return []string{}
	}
	return methods
}

// ── Component naming and usage ──────────────────────────────────────────────

func componentBaseName(filePath string) string {
	base := pathBaseName(filePath)
	if idx := strings.LastIndex(base, "."); idx >= 0 {
		return base[:idx]
	}
	return base
}

// computeComponentUsage fills each component's UsedBy slice with the paths of
// files that import it (determined by resolved import paths).
func computeComponentUsage(files []FileInfo, components []NextjsComponent) []NextjsComponent {
	idx := make(map[string]int, len(components))
	for i, c := range components {
		idx[c.File] = i
	}
	for _, f := range files {
		for _, imp := range f.Imports {
			if imp.Resolved == "" {
				continue
			}
			if ci, ok := idx[imp.Resolved]; ok {
				components[ci].UsedBy = append(components[ci].UsedBy, f.Path)
				break // count each importing file once
			}
		}
	}
	return components
}

// ── Detection helpers ───────────────────────────────────────────────────────

type packageJSONForDetection struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func hasNextInPackageJSON(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var pkg packageJSONForDetection
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	_, inDeps := pkg.Dependencies["next"]
	_, inDev := pkg.DevDependencies["next"]
	return inDeps || inDev
}
