package python

// APICallInfo holds the extracted data for a single detected HTTP call.
// Scope for M10.5 is deliberately narrow — only direct library calls
// (requests.<method>, httpx.<method>, urllib.request.urlopen) are detected.
// Client-variable tracing (client = httpx.AsyncClient(); client.get(...)) is
// deferred to a later milestone to avoid over-detection/false positives.
type APICallInfo struct {
	Method     string // "GET", "POST", etc.
	URL        string // extracted URL text (literal, f-string-as-is, or raw identifier name)
	Library    string // "requests", "httpx", "urllib"
	Line       int    // 1-indexed
	Confidence string // "high" (literal/f-string URL) or "medium" (bare identifier URL)
}

// httpVerbMethods maps a Python method name to its uppercased HTTP verb.
// Only these method names are recognized on requests./httpx. — anything else
// (e.g. requests.Session()) is not an HTTP call and is skipped.
var httpVerbMethods = map[string]string{
	"get":     "GET",
	"post":    "POST",
	"put":     "PUT",
	"delete":  "DELETE",
	"patch":   "PATCH",
	"head":    "HEAD",
	"options": "OPTIONS",
}

// APICalls walks the full AST recursively and returns all detected HTTP calls.
// Calls can appear at any depth (inside functions, async functions, with-blocks),
// unlike imports/declarations which are module-level only.
func (t *Tree) APICalls() []APICallInfo {
	root := t.RootNode()
	if root == nil {
		return nil
	}
	var out []APICallInfo
	walkForAPICalls(root, &out)
	return out
}

func walkForAPICalls(node *Node, out *[]APICallInfo) {
	if node.Type() == "call" {
		if info, ok := extractPythonAPICall(node); ok {
			*out = append(*out, info)
		}
	}
	for i := 0; i < node.NamedChildCount(); i++ {
		walkForAPICalls(node.NamedChild(i), out)
	}
}

// extractPythonAPICall recognizes requests.<method>(...), httpx.<method>(...),
// and urllib.request.urlopen(...) call shapes.
func extractPythonAPICall(call *Node) (APICallInfo, bool) {
	if call.NamedChildCount() < 2 {
		return APICallInfo{}, false
	}
	callee := call.NamedChild(0)
	argList := call.NamedChild(1)
	if callee == nil || callee.Type() != "attribute" || callee.NamedChildCount() < 2 {
		return APICallInfo{}, false
	}
	obj := callee.NamedChild(0)
	prop := callee.NamedChild(1).Content()

	library, method := classifyAPICallee(obj, prop)
	if library == "" {
		return APICallInfo{}, false
	}

	if argList == nil || argList.Type() != "argument_list" || argList.NamedChildCount() < 1 {
		return APICallInfo{}, false
	}
	url, confidence, ok := extractPythonURL(argList.NamedChild(0))
	if !ok {
		return APICallInfo{}, false
	}

	return APICallInfo{
		Method:     method,
		URL:        url,
		Library:    library,
		Line:       call.StartLine(),
		Confidence: confidence,
	}, true
}

// classifyAPICallee determines the library and HTTP method for a callee's
// object/property pair. Returns ("", "") when the pattern isn't recognized.
func classifyAPICallee(obj *Node, prop string) (library, method string) {
	switch obj.Type() {
	case "identifier":
		name := obj.Content()
		if name != "requests" && name != "httpx" {
			return "", ""
		}
		m, ok := httpVerbMethods[prop]
		if !ok {
			return "", ""
		}
		return name, m

	case "attribute":
		// urllib.request.urlopen — obj is the nested "urllib.request" attribute.
		if obj.NamedChildCount() < 2 {
			return "", ""
		}
		if obj.NamedChild(0).Content() != "urllib" || obj.NamedChild(1).Content() != "request" {
			return "", ""
		}
		if prop != "urlopen" {
			return "", ""
		}
		return "urllib", "GET"
	}
	return "", ""
}

// extractPythonURL extracts a URL and confidence from a call argument.
// String literals and f-strings are returned as-is (including {param} braces
// from f-string interpolation) at "high" confidence. Bare identifiers are
// returned as their raw name at "medium" confidence. Anything else is skipped.
func extractPythonURL(node *Node) (url, confidence string, ok bool) {
	switch node.Type() {
	case "string":
		return extractStringLiteralText(node), "high", true
	case "identifier":
		return node.Content(), "medium", true
	}
	return "", "", false
}

// extractStringLiteralText concatenates a string node's literal content and
// f-string interpolation parts, skipping the surrounding quote tokens.
// Interpolations (e.g. {user_id}) are kept verbatim, including braces — this
// makes f-string URLs naturally compatible with route path-parameter matching.
func extractStringLiteralText(node *Node) string {
	text := ""
	for i := 0; i < node.NamedChildCount(); i++ {
		c := node.NamedChild(i)
		switch c.Type() {
		case "string_content", "interpolation":
			text += c.Content()
		}
	}
	return text
}
