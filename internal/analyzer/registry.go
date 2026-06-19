package analyzer

// Registry holds registered adapters and dispatches files to the right one.
type Registry struct {
	adapters []LanguageAdapter
	basic    LanguageAdapter // fallback when no registered adapter matches
}

// NewRegistry returns a Registry with the basic fallback adapter pre-installed.
func NewRegistry() *Registry {
	return &Registry{basic: basicAdapter{}}
}

// Register adds an adapter. Adapters are checked in registration order;
// the first whose Detect returns true wins.
func (r *Registry) Register(adapter LanguageAdapter) {
	r.adapters = append(r.adapters, adapter)
}

// Dispatch returns the first adapter whose Detect returns true, or the basic
// fallback if none match.
func (r *Registry) Dispatch(path string, content []byte) LanguageAdapter {
	for _, a := range r.adapters {
		if a.Detect(path, content) {
			return a
		}
	}
	return r.basic
}
