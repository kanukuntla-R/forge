package typescript

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// CallInfo holds extracted HTTP call information.
type CallInfo struct {
	Target     string // URL string extracted from the call
	Method     string // "GET", "POST", etc.; always set (default "GET")
	Kind       string // "fetch", "axios", "ky", or "heuristic"
	Line       int    // 1-indexed
	Confidence string // "high", "medium", or "low"
}

// APICalls walks the full AST recursively and returns all detected HTTP calls.
// Unlike imports/exports which are always top-level, API calls appear anywhere
// in the AST (inside functions, effects, callbacks), requiring a full recursive walk.
func (t *Tree) APICalls() []CallInfo {
	var out []CallInfo
	walkForCalls(t.tree.RootNode(), t.source, &out)
	return out
}

// walkForCalls recursively visits every node in the tree. When it encounters a
// call_expression, it tries to extract an API call from it. Recursion continues
// into named children regardless of whether the current node matched.
func walkForCalls(node *sitter.Node, src []byte, out *[]CallInfo) {
	if node.Type() == "call_expression" {
		if call, ok := extractAPICall(node, src); ok {
			*out = append(*out, call)
		}
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		walkForCalls(node.NamedChild(i), src, out)
	}
}

// extractAPICall attempts to build a CallInfo from a single call_expression node.
// The flow is:
//  1. Identify kind and method-from-name via the function node (identifier or member_expression).
//  2. Extract the URL from the first argument (string or template literal; skip variables).
//  3. If kind is unknown (empty), apply the heuristic fallback: check isAPIURL; if it
//     passes, mark kind="heuristic". If it fails, discard the call entirely.
//  4. Determine method: from the call name (axios.post → POST) or from the options
//     object (second arg with { method: "POST" }) or default to "GET".
//  5. Compute confidence.
func extractAPICall(node *sitter.Node, src []byte) (CallInfo, bool) {
	// call_expression named children: [function, arguments]
	if node.NamedChildCount() < 2 {
		return CallInfo{}, false
	}
	funcNode := node.NamedChild(0) // identifier or member_expression
	argsNode := node.NamedChild(1) // arguments

	// Step 1: identify kind and method from the function being called.
	kind, methodFromName := identifyCallKind(funcNode, src)

	// Step 2: extract URL from first argument.
	if argsNode.NamedChildCount() < 1 {
		return CallInfo{}, false
	}
	url, urlConf, ok := extractURL(argsNode.NamedChild(0), src)
	if !ok {
		return CallInfo{}, false // variable, complex expr, or non-string → skip
	}

	// Step 3: heuristic fallback for unknown function names.
	// identifyCallKind returns ("", "") when the callee is not a known HTTP client.
	// If the URL looks API-like, we still record the call as kind="heuristic".
	// If it doesn't, we discard it entirely (too many false positives).
	if kind == "" {
		if !isAPIURL(url) {
			return CallInfo{}, false
		}
		kind = "heuristic"
	}

	// Step 4: determine HTTP method.
	// Priority: method from call name (axios.post) > options object > default GET.
	method := methodFromName
	if method == "" && argsNode.NamedChildCount() >= 2 {
		method = extractMethodFromOptions(argsNode.NamedChild(1), src)
	}
	if method == "" {
		method = "GET"
	}

	// Step 5: compute confidence.
	confidence := callConfidence(kind, urlConf)

	return CallInfo{
		Target:     url,
		Method:     method,
		Kind:       kind,
		Line:       int(node.StartPoint().Row) + 1,
		Confidence: confidence,
	}, true
}

// ── Function identification ─────────────────────────────────────────────────

var knownHTTPClients = map[string]bool{
	"fetch": true,
	"axios": true,
	"ky":    true,
}

var methodNameMap = map[string]string{
	"get":     "GET",
	"post":    "POST",
	"put":     "PUT",
	"patch":   "PATCH",
	"delete":  "DELETE",
	"options": "OPTIONS",
	"head":    "HEAD",
}

// identifyCallKind returns the HTTP client kind and, for method calls like
// axios.get or ky.post, the HTTP method derived from the property name.
// Returns ("", "") for unknown function callers — the heuristic path handles them.
func identifyCallKind(funcNode *sitter.Node, src []byte) (kind, method string) {
	switch funcNode.Type() {
	case "identifier":
		name := funcNode.Content(src)
		if knownHTTPClients[name] {
			return name, "" // method from options or defaults to GET
		}
		return "", "" // unknown: caller tries heuristic

	case "member_expression":
		// e.g. axios.get, ky.post, axios.PATCH
		// Named children: [object(identifier), property(property_identifier)]
		if funcNode.NamedChildCount() < 2 {
			return "", ""
		}
		obj  := funcNode.NamedChild(0).Content(src)
		prop := funcNode.NamedChild(1).Content(src)
		if knownHTTPClients[obj] {
			// Lowercase lookup to handle axios.PATCH → "PATCH"
			m := methodNameMap[strings.ToLower(prop)]
			if m == "" {
				m = strings.ToUpper(prop) // unknown property: uppercase as-is
			}
			return obj, m
		}
	}
	return "", ""
}

// ── URL extraction ──────────────────────────────────────────────────────────

// extractURL extracts a URL string and a confidence tag from a call argument.
// Returns ok=false for variables, call expressions, or other non-string nodes.
func extractURL(node *sitter.Node, src []byte) (url, confidence string, ok bool) {
	switch node.Type() {
	case "string":
		for i := 0; i < int(node.NamedChildCount()); i++ {
			if node.NamedChild(i).Type() == "string_fragment" {
				return node.NamedChild(i).Content(src), "literal", true
			}
		}
	case "template_string":
		return extractTemplateURL(node, src)
	}
	// identifier, call_expression, member_expression, etc. → skip
	return "", "", false
}

// extractTemplateURL processes a template_string node.
// Static parts become literal text; interpolated expressions become :name
// (for simple identifiers like ${id}) or :? (for complex expressions like ${user.id}).
func extractTemplateURL(node *sitter.Node, src []byte) (string, string, bool) {
	var parts []string
	interpolated := false

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "string_fragment":
			parts = append(parts, child.Content(src))
		case "template_substitution":
			interpolated = true
			if child.NamedChildCount() > 0 && child.NamedChild(0).Type() == "identifier" {
				parts = append(parts, ":"+child.NamedChild(0).Content(src))
			} else {
				parts = append(parts, ":?") // ${user.id}, ${fn()}, etc.
			}
		}
	}

	url := strings.Join(parts, "")
	if url == "" {
		return "", "", false
	}
	conf := "literal"
	if interpolated {
		conf = "interpolated"
	}
	return url, conf, true
}

// ── Method extraction from options object ────────────────────────────────────

// extractMethodFromOptions inspects a call's second argument for an options object
// containing { method: "POST" } and returns the uppercased method value.
func extractMethodFromOptions(node *sitter.Node, src []byte) string {
	if node.Type() != "object" {
		return ""
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		pair := node.NamedChild(i)
		if pair.Type() != "pair" || pair.NamedChildCount() < 2 {
			continue
		}
		key := pair.NamedChild(0)
		val := pair.NamedChild(1)
		if key.Content(src) == "method" && val.Type() == "string" {
			for j := 0; j < int(val.NamedChildCount()); j++ {
				if val.NamedChild(j).Type() == "string_fragment" {
					return strings.ToUpper(val.NamedChild(j).Content(src))
				}
			}
		}
	}
	return ""
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// isAPIURL returns true when the URL looks like an API endpoint.
// Used by the heuristic fallback to filter out false positives.
func isAPIURL(url string) bool {
	return strings.HasPrefix(url, "/api/") ||
		strings.HasPrefix(url, "http://") ||
		strings.HasPrefix(url, "https://") ||
		strings.HasPrefix(url, "//")
}

// callConfidence returns "high" for known patterns with literal URLs,
// "medium" for interpolated templates or heuristic calls.
func callConfidence(kind, urlConf string) string {
	if kind == "heuristic" {
		return "medium"
	}
	if urlConf == "literal" {
		return "high"
	}
	return "medium" // interpolated template literal
}
