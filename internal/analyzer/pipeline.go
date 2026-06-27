package analyzer

import "fmt"

// AnalyzeProject runs the full analysis pipeline for the given project root:
// walk → resolve imports → framework detection.
// Returns the analysis, non-fatal per-file warnings, and any fatal error.
func AnalyzeProject(projectRoot string) (*ProjectAnalysis, []error, error) {
	reg := NewRegistry()
	result, err := Walk(projectRoot, reg)
	if err != nil {
		return nil, nil, fmt.Errorf("walking %s: %w", projectRoot, err)
	}

	resolver, _ := NewResolver(projectRoot, result.Analysis)
	resolver.Resolve(result.Analysis)

	for _, d := range []FrameworkDetector{NewNextjsDetector()} {
		if d.Detect(projectRoot) {
			if err := d.EnrichAnalysis(result.Analysis); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("framework %s: %w", d.Name(), err))
			}
		}
	}

	return result.Analysis, result.Errors, nil
}
