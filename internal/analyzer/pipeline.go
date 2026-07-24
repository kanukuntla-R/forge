package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
)

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

	detectors := []FrameworkDetector{NewNextjsDetector(), NewFastAPIDetector()}

	// Pass 1: build each detected framework's structure (apps/routers/routes).
	for _, d := range detectors {
		if d.Detect(projectRoot) {
			if err := d.EnrichAnalysis(result.Analysis); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("framework %s: %w", d.Name(), err))
			}
		}
	}

	// Pass 2: match API calls, now that every framework's routes exist —
	// needed for cross-framework matching regardless of detector order.
	for _, d := range detectors {
		if err := d.MatchAPICalls(result.Analysis); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("framework %s: %w", d.Name(), err))
		}
	}

	// Pass 3: database detection, runs after frameworks so future work can
	// correlate routes with the tables they touch.
	files := make(map[string][]byte, len(result.Analysis.Files))
	for _, f := range result.Analysis.Files {
		content, err := os.ReadFile(filepath.Join(projectRoot, f.Path))
		if err != nil {
			continue
		}
		files[f.Path] = content
	}
	result.Analysis.Databases = DetectDatabases(result.Analysis, files, defaultDatabaseDetectors())

	return result.Analysis, result.Errors, nil
}

// defaultDatabaseDetectors returns the production set of database detectors.
func defaultDatabaseDetectors() []DatabaseDetector {
	return []DatabaseDetector{NewPrismaDetector()}
}
