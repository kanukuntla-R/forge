package analyzer

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FastAPIAnalysis holds all FastAPI structural data extracted from the analysis.
type FastAPIAnalysis struct {
	Apps     []FastAPIApp    `json:"apps"`
	Routers  []FastAPIRouter `json:"routers"`
	Routes   []FastAPIRoute  `json:"routes"`
	APICalls []PythonAPICall `json:"api_calls"`
}

// PythonAPICall represents a detected outgoing HTTP call, optionally matched
// to a FastAPI route within the same project.
type PythonAPICall struct {
	Method     string `json:"method"`
	URL        string `json:"url"`
	Library    string `json:"library"`
	Line       int    `json:"line"`
	File       string `json:"file"`
	Confidence string `json:"confidence"`
	ToRoute    string `json:"to_route,omitempty"` // matched route path; empty if unmatched
	ToFile     string `json:"to_file,omitempty"`  // file where the matched route is defined
}

// FastAPIApp represents a `FastAPI()` instantiation.
type FastAPIApp struct {
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

// FastAPIRouter represents an `APIRouter()` instantiation.
type FastAPIRouter struct {
	Name       string `json:"name"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Prefix     string `json:"prefix"`     // resolved prefix, empty if no prefix
	Reachable  bool   `json:"reachable"`  // true if some app/router calls include_router(this)
	Confidence string `json:"confidence"` // "high" (literal prefix), "medium" (resolved variable), "low" (unresolvable)
}

// FastAPIRoute represents a single route handler.
type FastAPIRoute struct {
	Method     string `json:"method"`
	Path       string `json:"path"` // full path including router prefix
	Handler    string `json:"handler"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	IsAsync    bool   `json:"is_async"`
	RouterName string `json:"router_name"` // "app" or the router variable name
	Reachable  bool   `json:"reachable"`
	Confidence string `json:"confidence"`
}

// fastapiDetector implements FrameworkDetector for FastAPI projects.
type fastapiDetector struct{}

// NewFastAPIDetector returns a FrameworkDetector for FastAPI.
func NewFastAPIDetector() FrameworkDetector {
	return &fastapiDetector{}
}

func (d *fastapiDetector) Name() string { return "fastapi" }

// Detect does a cheap on-disk scan for "fastapi" appearing in any .py file.
// This is deliberately coarse (a substring scan, not a parse) — the authoritative
// check is isFastAPIProject, which runs in EnrichAnalysis against real parsed imports.
func (d *fastapiDetector) Detect(projectRoot string) bool {
	found := false
	_ = filepath.WalkDir(projectRoot, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if found {
			return filepath.SkipAll
		}
		if de.IsDir() {
			base := de.Name()
			if IsIgnoredDir(base) || (strings.HasPrefix(base, ".") && base != ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".py") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if strings.Contains(string(content), "fastapi") {
			found = true
		}
		return nil
	})
	return found
}

// isFastAPIProject is the authoritative check, run against already-parsed imports.
func isFastAPIProject(analysis *ProjectAnalysis) bool {
	for _, file := range analysis.Files {
		for _, imp := range file.Imports {
			if imp.Source == "fastapi" || strings.HasPrefix(imp.Source, "fastapi.") {
				return true
			}
		}
	}
	return false
}

// EnrichAnalysis populates analysis.Frameworks["fastapi"] by scanning declarations,
// decorators, and module-level calls for FastAPI apps, routers, and routes.
func (d *fastapiDetector) EnrichAnalysis(analysis *ProjectAnalysis) error {
	if !isFastAPIProject(analysis) {
		return nil
	}

	info := &FastAPIAnalysis{
		Apps:     []FastAPIApp{},
		Routers:  []FastAPIRouter{},
		Routes:   []FastAPIRoute{},
		APICalls: []PythonAPICall{},
	}

	moduleIndex := buildModuleIndex(analysis.Files)

	// Step 1: collect module-level variables per file (for prefix resolution),
	// and identify apps/routers.
	fileVars := make(map[string]map[string]string) // file -> varName -> ValueRepr
	routerHasOwnPrefix := make(map[string]bool)    // "file|name" -> prefix came from its own APIRouter(...)
	for _, file := range analysis.Files {
		vars := make(map[string]string)
		for _, decl := range file.Declarations {
			if decl.Type != "variable" {
				continue
			}
			vars[decl.Name] = decl.ValueRepr
		}
		fileVars[file.Path] = vars

		for _, decl := range file.Declarations {
			if decl.Type != "variable" {
				continue
			}
			switch {
			case strings.HasPrefix(decl.ValueRepr, "FastAPI("):
				info.Apps = append(info.Apps, FastAPIApp{
					Name: decl.Name,
					File: file.Path,
					Line: decl.Line,
				})
			case strings.HasPrefix(decl.ValueRepr, "APIRouter("):
				prefix, confidence, hasOwn := resolveRouterPrefix(decl.ValueRepr, vars)
				info.Routers = append(info.Routers, FastAPIRouter{
					Name:       decl.Name,
					File:       file.Path,
					Line:       decl.Line,
					Prefix:     prefix,
					Confidence: confidence,
				})
				routerHasOwnPrefix[routerKey(file.Path, decl.Name)] = hasOwn
			}
		}
	}

	// Step 2: mark routers reachable via include_router() calls, and — for
	// routers that don't declare their own prefix — resolve the prefix from
	// the include_router() call site instead (a common FastAPI pattern: the
	// router itself is bare, and prefix="..." is supplied where it's mounted).
	// Router variables are commonly named identically across files (every
	// submodule calls its router "router"), so matches must be disambiguated
	// by which file the dotted reference (e.g. users.router) actually points
	// to via the caller's imports — not by name alone.
	for _, file := range analysis.Files {
		vars := fileVars[file.Path]
		for _, call := range file.ModuleCalls {
			if !strings.HasSuffix(call.FunctionRepr, ".include_router") {
				continue
			}
			routerArg := firstPositionalArg(call.ArgsRepr)
			routerFile, name, ok := resolveIncludedRouter(routerArg, file.Path, file.Imports, moduleIndex, info.Routers)
			if !ok {
				continue
			}
			markRouterReachable(info.Routers, routerFile, name)
			key := routerKey(routerFile, name)
			if !routerHasOwnPrefix[key] {
				if prefix, confidence, found := resolvePrefixArg(call.ArgsRepr, vars); found {
					setRouterPrefix(info.Routers, routerFile, name, prefix, confidence)
					routerHasOwnPrefix[key] = true
				}
			}
		}
	}

	// Step 3: build routes from decorated declarations.
	for _, file := range analysis.Files {
		for _, decl := range file.Declarations {
			for _, dec := range decl.Decorators {
				rd, ok := parseRouteDecorator(dec)
				if !ok {
					continue
				}
				routerName, prefix, confidence, reachable, isApp, found := resolveRouteTarget(rd.varName, file.Path, info.Apps, info.Routers)
				if !found {
					continue
				}
				_ = isApp

				methods := rd.methods
				for _, method := range methods {
					info.Routes = append(info.Routes, FastAPIRoute{
						Method:     method,
						Path:       combinePath(prefix, rd.path),
						Handler:    decl.Name,
						File:       file.Path,
						Line:       decl.Line,
						IsAsync:    decl.Type == "async_function",
						RouterName: routerName,
						Reachable:  reachable,
						Confidence: confidence,
					})
				}
			}
		}
	}

	// Step 4: gather Python HTTP calls from all files and match them to routes.
	// Library is only set on calls the Python adapter extracted (requests/httpx/
	// urllib) — TypeScript's fetch/axios/ky calls leave it empty and are skipped.
	for _, file := range analysis.Files {
		for _, call := range file.Calls {
			if call.Library == "" {
				continue
			}
			apiCall := PythonAPICall{
				Method:     call.Method,
				URL:        call.Target,
				Library:    call.Library,
				Line:       call.Line,
				File:       file.Path,
				Confidence: call.Confidence,
			}
			if route, ok := matchRoute(apiCall, info.Routes); ok {
				apiCall.ToRoute = route.Path
				apiCall.ToFile = route.File
			}
			info.APICalls = append(info.APICalls, apiCall)
		}
	}

	if analysis.Frameworks == nil {
		analysis.Frameworks = make(map[string]any)
	}
	analysis.Frameworks["fastapi"] = info
	analysis.Project.Frameworks = append(analysis.Project.Frameworks, "fastapi")
	return nil
}

// ── Prefix resolution ────────────────────────────────────────────────────────

// resolveRouterPrefix extracts and resolves the `prefix=` argument from an
// APIRouter(...) ValueRepr string. hasOwn reports whether a prefix= argument
// was present at all (as opposed to the router being bare, prefix resolved
// later from wherever it's include_router()'d).
func resolveRouterPrefix(valueRepr string, vars map[string]string) (prefix, confidence string, hasOwn bool) {
	prefix, confidence, found := resolvePrefixArg(valueRepr, vars)
	if !found {
		return "", "high", false
	}
	return prefix, confidence, true
}

// resolvePrefixArg extracts and resolves a `prefix=` keyword argument from raw
// call/constructor argument text (e.g. `prefix="/users"` or `prefix=PREFIX`).
// vars holds other module-level variables in the relevant file, used to
// resolve `prefix=SOME_VAR` references.
func resolvePrefixArg(raw string, vars map[string]string) (prefix, confidence string, found bool) {
	val, isLiteral, ok := extractKeywordArg(raw, "prefix")
	if !ok {
		return "", "", false
	}
	if isLiteral {
		return val, "high", true
	}
	// val is a bare identifier; try to resolve it against module-level variables.
	if v, ok := vars[val]; ok {
		if lit, ok := stringLiteralValue(v); ok {
			return lit, "medium", true
		}
	}
	return val, "low", true
}

// extractKeywordArg finds `key=...` inside a call's ValueRepr text and returns
// its raw value. isLiteral is true when the value is a quoted string literal
// (quotes stripped from the returned value); false for a bare identifier token.
func extractKeywordArg(valueRepr, key string) (value string, isLiteral bool, found bool) {
	idx := strings.Index(valueRepr, key+"=")
	if idx == -1 {
		return "", false, false
	}
	rest := strings.TrimSpace(valueRepr[idx+len(key)+1:])
	if rest == "" {
		return "", false, false
	}
	if rest[0] == '"' || rest[0] == '\'' {
		quote := rest[0]
		end := strings.IndexByte(rest[1:], quote)
		if end == -1 {
			return "", false, false
		}
		return rest[1 : 1+end], true, true
	}
	end := strings.IndexAny(rest, ",)")
	if end == -1 {
		end = len(rest)
	}
	return strings.TrimSpace(rest[:end]), false, true
}

// stringLiteralValue strips the surrounding quotes from a raw ValueRepr if it
// is a plain quoted string literal (e.g. `"/api/v1"` -> `/api/v1`).
func stringLiteralValue(valueRepr string) (string, bool) {
	if len(valueRepr) < 2 {
		return "", false
	}
	first := valueRepr[0]
	last := valueRepr[len(valueRepr)-1]
	if (first == '"' || first == '\'') && first == last {
		return valueRepr[1 : len(valueRepr)-1], true
	}
	return "", false
}

// ── include_router() resolution ─────────────────────────────────────────────

// routerKey builds a composite key so routers (commonly all named "router"
// across different submodules) can be looked up unambiguously by file+name.
func routerKey(file, name string) string { return file + "|" + name }

// buildModuleIndex maps each Python file's dotted module path to its file path,
// e.g. "src/routes/users.py" -> module "src.routes.users", and
// "src/routes/__init__.py" -> module "src.routes". Used to resolve which file
// a `from X import Y` statement's Y actually refers to.
func buildModuleIndex(files []FileInfo) map[string]string {
	idx := make(map[string]string, len(files))
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".py") {
			continue
		}
		mod := strings.TrimSuffix(f.Path, ".py")
		mod = strings.ReplaceAll(mod, "/", ".")
		mod = strings.TrimSuffix(mod, ".__init__")
		idx[mod] = f.Path
	}
	return idx
}

// resolveModuleAlias finds which file the given module alias refers to, by
// looking for the import that introduced it in callerImports (e.g. `from
// src.routes import users` introduces alias "users") and resolving the
// resulting dotted module path against moduleIndex.
func resolveModuleAlias(alias string, callerImports []Import, moduleIndex map[string]string) (file string, ok bool) {
	for _, imp := range callerImports {
		if imp.IsRelative {
			continue
		}
		for _, n := range imp.Names {
			if n == alias {
				if f, ok := moduleIndex[imp.Source+"."+alias]; ok {
					return f, true
				}
			}
		}
		if imp.Alias == alias {
			if f, ok := moduleIndex[imp.Source]; ok {
				return f, true
			}
		}
	}
	return "", false
}

// resolveIncludedRouter matches an include_router() argument against known
// routers. Attribute references (users.router) are resolved via the caller's
// imports to a specific file first; bare identifiers prefer a same-file match.
// Both fall back to matching by trailing name alone (assume unique names)
// when the precise resolution fails.
func resolveIncludedRouter(target, callerFile string, callerImports []Import, moduleIndex map[string]string, routers []FastAPIRouter) (routerFile, name string, ok bool) {
	if idx := strings.LastIndex(target, "."); idx >= 0 {
		alias := target[:idx]
		trailing := target[idx+1:]
		if file, resolved := resolveModuleAlias(alias, callerImports, moduleIndex); resolved {
			for _, r := range routers {
				if r.Name == trailing && r.File == file {
					return r.File, r.Name, true
				}
			}
		}
		for _, r := range routers {
			if r.Name == trailing {
				return r.File, r.Name, true
			}
		}
		return "", "", false
	}
	for _, r := range routers {
		if r.Name == target && r.File == callerFile {
			return r.File, r.Name, true
		}
	}
	for _, r := range routers {
		if r.Name == target {
			return r.File, r.Name, true
		}
	}
	return "", "", false
}

func markRouterReachable(routers []FastAPIRouter, file, name string) {
	for i := range routers {
		if routers[i].Name == name && routers[i].File == file {
			routers[i].Reachable = true
		}
	}
}

func setRouterPrefix(routers []FastAPIRouter, file, name, prefix, confidence string) {
	for i := range routers {
		if routers[i].Name == name && routers[i].File == file {
			routers[i].Prefix = prefix
			routers[i].Confidence = confidence
		}
	}
}

// firstPositionalArg returns the first top-level argument in raw argument text
// that isn't a keyword argument (i.e. has no top-level `=`), e.g. for
// `users.router, prefix="/users", tags=["users"]` it returns "users.router".
func firstPositionalArg(raw string) string {
	for _, part := range splitTopLevelArgs(raw) {
		if !strings.Contains(part, "=") {
			return part
		}
	}
	return ""
}

// splitTopLevelArgs splits raw call-argument text on commas, ignoring commas
// nested inside brackets/parens/quotes (e.g. the comma inside `tags=["a","b"]`).
func splitTopLevelArgs(raw string) []string {
	var parts []string
	depth := 0
	var quote byte
	start := 0
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(raw[start:i]))
				start = i + 1
			}
		}
	}
	if last := strings.TrimSpace(raw[start:]); last != "" {
		parts = append(parts, last)
	}
	return parts
}

// ── Route decorator parsing ──────────────────────────────────────────────────

var fastapiHTTPMethods = map[string]bool{
	"get": true, "post": true, "put": true, "delete": true,
	"patch": true, "head": true, "options": true, "api_route": true,
}

type routeDecorator struct {
	varName string
	path    string
	methods []string // one HTTP method, or several for api_route(methods=[...])
}

// parseRouteDecorator recognizes `<var>.<method>("/path", ...)` decorator text
// and extracts the object variable, HTTP method(s), and path.
func parseRouteDecorator(dec string) (routeDecorator, bool) {
	dotIdx := strings.IndexByte(dec, '.')
	if dotIdx < 0 {
		return routeDecorator{}, false
	}
	varName := dec[:dotIdx]
	rest := dec[dotIdx+1:]

	parenIdx := strings.IndexByte(rest, '(')
	if parenIdx < 0 || !strings.HasSuffix(rest, ")") {
		return routeDecorator{}, false
	}
	methodName := rest[:parenIdx]
	if !fastapiHTTPMethods[methodName] {
		return routeDecorator{}, false
	}
	args := rest[parenIdx+1 : len(rest)-1]

	path, ok := firstStringArg(args)
	if !ok {
		return routeDecorator{}, false
	}

	d := routeDecorator{varName: varName, path: path}
	if methodName == "api_route" {
		d.methods = extractMethodsList(args)
		if len(d.methods) == 0 {
			return routeDecorator{}, false
		}
	} else {
		d.methods = []string{strings.ToUpper(methodName)}
	}
	return d, true
}

// firstStringArg returns the contents of the first quoted string in args.
func firstStringArg(args string) (string, bool) {
	for i := 0; i < len(args); i++ {
		c := args[i]
		if c == '"' || c == '\'' {
			end := strings.IndexByte(args[i+1:], c)
			if end == -1 {
				return "", false
			}
			return args[i+1 : i+1+end], true
		}
	}
	return "", false
}

// extractMethodsList parses `methods=["GET", "POST"]` into ["GET", "POST"].
func extractMethodsList(args string) []string {
	idx := strings.Index(args, "methods=")
	if idx == -1 {
		return nil
	}
	rest := args[idx+len("methods="):]
	start := strings.IndexByte(rest, '[')
	end := strings.IndexByte(rest, ']')
	if start == -1 || end == -1 || end < start {
		return nil
	}
	inner := rest[start+1 : end]
	var methods []string
	for _, p := range strings.Split(inner, ",") {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"'`)
		if p != "" {
			methods = append(methods, strings.ToUpper(p))
		}
	}
	return methods
}

// resolveRouteTarget looks up the app/router a decorator's variable refers to,
// preferring a match in the same file before falling back to any file.
func resolveRouteTarget(varName, file string, apps []FastAPIApp, routers []FastAPIRouter) (routerName, prefix, confidence string, reachable, isApp, found bool) {
	for _, a := range apps {
		if a.Name == varName && a.File == file {
			return a.Name, "", "high", true, true, true
		}
	}
	for _, r := range routers {
		if r.Name == varName && r.File == file {
			return r.Name, r.Prefix, r.Confidence, r.Reachable, false, true
		}
	}
	for _, a := range apps {
		if a.Name == varName {
			return a.Name, "", "high", true, true, true
		}
	}
	for _, r := range routers {
		if r.Name == varName {
			return r.Name, r.Prefix, r.Confidence, r.Reachable, false, true
		}
	}
	return "", "", "", false, false, false
}

// combinePath joins a router prefix and a route's own path.
func combinePath(prefix, path string) string {
	if prefix == "" {
		return path
	}
	p := strings.TrimSuffix(prefix, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return p + path
}

// ── API call route matching ─────────────────────────────────────────────────

// matchRoute finds the first route matching call's method and (normalized) URL.
func matchRoute(call PythonAPICall, routes []FastAPIRoute) (FastAPIRoute, bool) {
	url := normalizeCallURL(call.URL)
	for _, route := range routes {
		if route.Method != call.Method {
			continue
		}
		if pathsMatch(url, route.Path) {
			return route, true
		}
	}
	return FastAPIRoute{}, false
}

// normalizeCallURL strips a scheme://host:port prefix, leaving just the path.
func normalizeCallURL(url string) string {
	idx := strings.Index(url, "://")
	if idx < 0 {
		return url
	}
	rest := url[idx+3:]
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		return rest[slash:]
	}
	return "/"
}

// pathsMatch compares two paths segment by segment. A segment wrapped in
// braces ({id}) matches any segment on the other side — this covers both a
// route's own path parameters and f-string placeholders on the call side
// (requests.get(f"/users/{user_id}")), which use the identical {name} syntax.
func pathsMatch(callURL, routePath string) bool {
	callSegs := strings.Split(strings.Trim(callURL, "/"), "/")
	routeSegs := strings.Split(strings.Trim(routePath, "/"), "/")
	if len(callSegs) != len(routeSegs) {
		return false
	}
	for i := range routeSegs {
		if isPathParam(routeSegs[i]) || isPathParam(callSegs[i]) {
			continue
		}
		if routeSegs[i] != callSegs[i] {
			return false
		}
	}
	return true
}

func isPathParam(seg string) bool {
	return strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")
}
