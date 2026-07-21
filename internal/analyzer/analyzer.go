package analyzer

// ProjectAnalysis is the top-level type written to .forge/analysis.json.
type ProjectAnalysis struct {
	Version      string           `json:"version"`
	GeneratedAt  string           `json:"generated_at"`
	CachedFrom   string           `json:"cached_from,omitempty"`
	ContentHash  string           `json:"content_hash,omitempty"`
	Project      ProjectInfo      `json:"project"`
	Files        []FileInfo       `json:"files"`
	Frameworks   map[string]any   `json:"frameworks,omitempty"`
	ExternalDeps []ExternalDep    `json:"external_dependencies,omitempty"`
	Databases    []DatabaseSchema `json:"databases,omitempty"`
}

// ProjectInfo holds metadata about the analyzed project.
type ProjectInfo struct {
	Root       string   `json:"root"`
	Name       string   `json:"name"`
	Languages  []string `json:"languages"`
	Frameworks []string `json:"frameworks"`
}

// FileInfo holds the analysis result for a single file.
type FileInfo struct {
	ID              string          `json:"id"`
	Path            string          `json:"path"`
	Language        string          `json:"language"`
	SizeBytes       int64           `json:"size_bytes"`
	Exports         []Export        `json:"exports,omitempty"`
	Imports         []Import        `json:"imports,omitempty"`
	Declarations    []Declaration   `json:"declarations,omitempty"`
	Calls           []Call          `json:"calls,omitempty"`
	ModuleCalls     []ModuleCall    `json:"module_calls,omitempty"`
	Metadata        map[string]any  `json:"metadata,omitempty"`
	DatabaseQueries []DatabaseQuery `json:"database_queries,omitempty"`
}

// ExternalDep records an external package and which files reference it.
type ExternalDep struct {
	Package string   `json:"package"`
	UsedBy  []string `json:"used_by"`
}
