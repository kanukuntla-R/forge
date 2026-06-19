package analyzer

// basicAdapter is the fallback adapter used when no language-specific adapter
// matches a file. It records the file's existence but performs no parsing.
type basicAdapter struct{}

func (basicAdapter) Name() string                           { return "basic" }
func (basicAdapter) FileExtensions() []string               { return nil }
func (basicAdapter) Detect(_ string, _ []byte) bool         { return true }
func (basicAdapter) Analyze(_ string, _ []byte) (*FileAnalysis, error) {
	return &FileAnalysis{}, nil
}
