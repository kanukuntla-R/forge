package python

// CallInfo holds the extracted data for a bare (non-assignment) module-level
// call statement, e.g. `app.include_router(users.router)`.
// Extraction is unfiltered — every module-level call statement is captured,
// not just ones recognized by a specific framework detector.
type CallInfo struct {
	FunctionRepr string // the callable expression, e.g. "app.include_router" or "configure_logging"
	ArgsRepr     string // raw text of all arguments, e.g. `users.router, prefix="/users"`; "" if none
	Line         int    // 1-indexed
}

// ModuleCalls walks the module root and returns all bare call-expression
// statements. Assignments (`app = FastAPI()`) are not call statements and are
// handled separately by Declarations().
func (t *Tree) ModuleCalls() []CallInfo {
	root := t.RootNode()
	if root == nil {
		return nil
	}
	var out []CallInfo
	for i := 0; i < root.NamedChildCount(); i++ {
		stmt := root.NamedChild(i)
		if stmt == nil || stmt.Type() != "expression_statement" {
			continue
		}
		if stmt.NamedChildCount() == 0 {
			continue
		}
		call := stmt.NamedChild(0)
		if call == nil || call.Type() != "call" {
			continue
		}
		if info, ok := extractCallInfo(call); ok {
			out = append(out, info)
		}
	}
	return out
}

// extractCallInfo converts a `call` node into a CallInfo. The callable is
// either an `attribute` (obj.method) or a plain `identifier` (bare function).
func extractCallInfo(call *Node) (CallInfo, bool) {
	if call.NamedChildCount() == 0 {
		return CallInfo{}, false
	}
	callee := call.NamedChild(0)
	if callee == nil {
		return CallInfo{}, false
	}
	if callee.Type() != "attribute" && callee.Type() != "identifier" {
		return CallInfo{}, false
	}

	args := ""
	if call.NamedChildCount() > 1 {
		argList := call.NamedChild(1)
		if argList != nil && argList.Type() == "argument_list" {
			// argList.Content() is the full "(...)" text; strip the parens to get
			// the raw argument text exactly as written, keyword args included.
			text := argList.Content()
			if len(text) >= 2 {
				args = text[1 : len(text)-1]
			}
		}
	}

	return CallInfo{
		FunctionRepr: callee.Content(),
		ArgsRepr:     args,
		Line:         call.StartLine(),
	}, true
}
