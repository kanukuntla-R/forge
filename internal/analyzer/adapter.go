package analyzer

// LanguageAdapter handles parsing of files for a specific language.
type LanguageAdapter interface {
	Name() string
	FileExtensions() []string
	Detect(path string, content []byte) bool
	Analyze(path string, content []byte) (*FileAnalysis, error)
}

// FileAnalysis holds extracted information from a single file.
type FileAnalysis struct {
	Imports      []Import
	Exports      []Export
	Declarations []Declaration
	Calls        []Call
	ModuleCalls  []ModuleCall
	Metadata     map[string]any
}

// Import represents a module import.
type Import struct {
	Source     string   `json:"source"`
	Resolved   string   `json:"resolved,omitempty"`
	Names      []string `json:"names"`            // no omitempty: [] is meaningful for side-effect imports
	Alias      string   `json:"alias,omitempty"`  // set for `import X as Y` (top-level alias only)
	IsRelative bool     `json:"is_relative,omitempty"` // true for Python relative imports (from . / from .. )
	IsStar     bool     `json:"is_star,omitempty"`     // true for `from X import *`
	External   bool     `json:"external,omitempty"`
	Line       int      `json:"line,omitempty"`
}

// Export represents a module export.
type Export struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Line int    `json:"line"`
}

// Declaration represents a top-level declaration (function, class, etc.)
type Declaration struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Line       int      `json:"line"`
	Decorators []string `json:"decorators,omitempty"` // Python: decorator expressions (without leading @)
	Bases      []string `json:"bases,omitempty"`      // Python: base class names for class declarations
	ValueRepr  string   `json:"value_repr,omitempty"` // Python: truncated RHS for variable declarations
}

// Call represents a function/API call extracted from the source.
type Call struct {
	Target     string `json:"target"`
	Method     string `json:"method,omitempty"`
	Line       int    `json:"line"`
	Kind       string `json:"kind"`
	Confidence string `json:"confidence"`
	Library    string `json:"library,omitempty"` // Python: "requests", "httpx", "urllib"; unset for TypeScript calls
}

// ModuleCall represents a bare (non-assignment) call statement at module level,
// e.g. Python's `app.include_router(users.router)`. Extraction is unfiltered —
// framework detectors interpret the raw FunctionRepr/ArgsRepr as needed.
type ModuleCall struct {
	FunctionRepr string `json:"function"` // the callable expression, e.g. "app.include_router"
	ArgsRepr     string `json:"args"`     // first positional argument's source text; "" if none
	Line         int    `json:"line"`
}
