package analyzer

import "strings"

// ── URL normalization ────────────────────────────────────────────────────────

// localDevHosts are the only hosts NormalizeURL will strip. An external host
// (api.example.com, a customer's domain, etc.) is left untouched — forge has
// no way to know whether that's actually part of this project, so the call
// simply won't match any route. That's the honest outcome, not a bug.
var localDevHosts = []string{"localhost", "127.0.0.1", "0.0.0.0"}

// NormalizeURL strips scheme+host from an absolute URL when the host is a
// recognized local dev host, and strips any query string or fragment.
// Trailing slashes are preserved (they matter for exact-match comparison).
//
// Examples:
//
//	http://localhost:8000/products/  → /products/
//	http://localhost:8000/products?x=1 → /products
//	http://api.example.com/users     → http://api.example.com/users (untouched)
//	/api/cart                        → /api/cart (untouched)
func NormalizeURL(rawURL string) string {
	rest, isAbsolute := splitScheme(rawURL)
	if isAbsolute {
		host, path := hostAndPath(rest)
		if !isLocalDevHost(host) {
			return rawURL
		}
		rawURL = path
	}
	if i := strings.IndexAny(rawURL, "?#"); i >= 0 {
		rawURL = rawURL[:i]
	}
	return rawURL
}

func splitScheme(rawURL string) (rest string, isAbsolute bool) {
	for _, p := range []string{"https://", "http://"} {
		if strings.HasPrefix(rawURL, p) {
			return rawURL[len(p):], true
		}
	}
	return rawURL, false
}

// hostAndPath splits a post-scheme remainder into host and path. IPv6
// literals ("[::1]:8000/x") are bracket-aware; everything else falls back to
// the first "/" after the host.
func hostAndPath(rest string) (host, path string) {
	if strings.HasPrefix(rest, "[") {
		if end := strings.IndexByte(rest, ']'); end >= 0 {
			afterBracket := rest[end+1:]
			if slash := strings.IndexByte(afterBracket, '/'); slash >= 0 {
				return rest[:end+1] + afterBracket[:slash], afterBracket[slash:]
			}
			return rest, "/"
		}
	}
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		return rest[:slash], rest[slash:]
	}
	return rest, "/"
}

func isLocalDevHost(host string) bool {
	for _, h := range localDevHosts {
		if host == h || strings.HasPrefix(host, h+":") {
			return true
		}
	}
	return false
}

// wasNormalizedFromDevHost reports whether rawURL is absolute with a
// recognized local dev host — i.e. NormalizeURL actually stripped it.
func wasNormalizedFromDevHost(rawURL string) bool {
	rest, isAbsolute := splitScheme(rawURL)
	if !isAbsolute {
		return false
	}
	host, _ := hostAndPath(rest)
	return isLocalDevHost(host)
}

// ── Path-parameter matching ──────────────────────────────────────────────────

// isRouteParamSegment recognizes a route's own placeholder syntax:
// ":name" (Next.js, from [name]/route.ts) or "{name}" (FastAPI).
func isRouteParamSegment(seg string) bool {
	return strings.HasPrefix(seg, ":") || isBraceWrapped(seg)
}

func isBraceWrapped(seg string) bool {
	return len(seg) > 1 && strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")
}

// segmentsMatch compares one route path segment against one call path
// segment. Only the ROUTE's own declared placeholder (":name" or "{name}")
// wildcards — it absorbs any call segment, concrete or templated. A call
// segment being templated (":id", ":?", or, for Python calls, "{name}")
// does NOT by itself grant a wildcard: if the route segment is a fixed
// literal ("search"), a variable call segment can't be verified to equal
// it, so the honest answer is no match, not a guess. This also matters for
// route pools with adjacent static and dynamic siblings (e.g. "/search" and
// "/{id}") — without this, an unresolved call segment could snap onto the
// wrong sibling depending on route declaration order.
//
// A call segment using brace syntax ("{name}") only counts as templated when
// allowCallBraceTemplate is true (Python f-string interpolations render this
// way intentionally); otherwise a literal "{x}" in a call URL is almost
// always a typo, not a placeholder, so it's compared literally instead.
func segmentsMatch(routeSeg, callSeg string, allowCallBraceTemplate bool) bool {
	if isBraceWrapped(callSeg) && !allowCallBraceTemplate {
		return routeSeg == callSeg
	}
	if isRouteParamSegment(routeSeg) {
		return true
	}
	return routeSeg == callSeg
}

func pathSegmentsMatch(routePath, callPath string, allowCallBraceTemplate bool) bool {
	rs := strings.Split(strings.Trim(routePath, "/"), "/")
	cs := strings.Split(strings.Trim(callPath, "/"), "/")
	if len(rs) != len(cs) {
		return false
	}
	for i := range rs {
		if !segmentsMatch(rs[i], cs[i], allowCallBraceTemplate) {
			return false
		}
	}
	return true
}

// ── Cross-framework route matching ──────────────────────────────────────────

// routeCandidate is a framework-agnostic view of a single route, used to pool
// routes from both frameworks for matching.
type routeCandidate struct {
	Path    string
	Methods []string
	File    string
}

func nextjsRouteCandidates(routes []NextjsRoute) []routeCandidate {
	out := make([]routeCandidate, len(routes))
	for i, r := range routes {
		out[i] = routeCandidate{Path: r.Path, Methods: r.Methods, File: r.File}
	}
	return out
}

func fastapiRouteCandidates(routes []FastAPIRoute) []routeCandidate {
	out := make([]routeCandidate, len(routes))
	for i, r := range routes {
		out[i] = routeCandidate{Path: r.Path, Methods: []string{r.Method}, File: r.File}
	}
	return out
}

func methodIn(method string, methods []string) bool {
	for _, m := range methods {
		if m == method {
			return true
		}
	}
	return false
}

// matchInPool finds the best match for method+rawURL among a single pool of
// routes. Exact matches (post-normalization) are preferred over pattern
// (path-param) matches.
func matchInPool(method, rawURL string, pool []routeCandidate, allowCallBraceTemplate bool) (path, file, matchKind string, found bool) {
	url := NormalizeURL(rawURL)
	for _, c := range pool {
		if methodIn(method, c.Methods) && c.Path == url {
			return c.Path, c.File, "exact", true
		}
	}
	for _, c := range pool {
		if methodIn(method, c.Methods) && pathSegmentsMatch(c.Path, url, allowCallBraceTemplate) {
			return c.Path, c.File, "pattern", true
		}
	}
	return "", "", "", false
}

// MatchAPICall matches a call's method+URL against the caller's own
// framework's routes first, falling back to the other framework's routes.
// toFramework is empty for a native (same-framework) match, or names the
// framework that owns the matched route for a cross-framework match.
func MatchAPICall(method, rawURL, nativeFramework string, nativeRoutes, otherRoutes []routeCandidate, allowCallBraceTemplate bool) (path, file, toFramework, matchKind string, found bool) {
	if p, f, k, ok := matchInPool(method, rawURL, nativeRoutes, allowCallBraceTemplate); ok {
		return p, f, "", k, true
	}
	other := "fastapi"
	if nativeFramework == "fastapi" {
		other = "nextjs"
	}
	if p, f, k, ok := matchInPool(method, rawURL, otherRoutes, allowCallBraceTemplate); ok {
		return p, f, other, k, true
	}
	return "", "", "", "", false
}

// CrossFrameworkConfidence returns the confidence for a cross-framework
// match: "high" when the call's URL pointed at a recognized local dev host
// (a strong, deliberate signal it's calling this project's own backend),
// "medium" otherwise (a bare relative path that happened to also match the
// other framework's routes).
func CrossFrameworkConfidence(rawURL string) string {
	if wasNormalizedFromDevHost(rawURL) {
		return "high"
	}
	return "medium"
}
