package analyzer

// FrameworkDetector recognizes a specific framework and enriches the analysis.
// Implementations are added in M8.3+; this interface is defined here so the
// architecture is in place from day one.
type FrameworkDetector interface {
	Name() string
	Detect(projectRoot string) bool
	EnrichAnalysis(analysis *ProjectAnalysis) error

	// MatchAPICalls correlates detected HTTP calls against known routes. It
	// runs after EnrichAnalysis has run for every detected framework, so
	// implementations can match calls against other frameworks' routes too.
	MatchAPICalls(analysis *ProjectAnalysis) error
}
