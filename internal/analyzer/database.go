package analyzer

import (
	"path/filepath"
	"strings"
)

// isORMTestFile reports whether rel looks like a TS/JS test file: it's
// excluded from query detection because test traffic isn't a real query.
// Shared by every DatabaseDetector's MatchQueries implementation.
func isORMTestFile(rel string) bool {
	base := filepath.Base(rel)
	for _, suf := range []string{".test.ts", ".test.tsx", ".spec.ts", ".spec.tsx"} {
		if strings.HasSuffix(base, suf) {
			return true
		}
	}
	slashed := "/" + filepath.ToSlash(rel) + "/"
	for _, seg := range []string{"/__tests__/", "/test/", "/tests/"} {
		if strings.Contains(slashed, seg) {
			return true
		}
	}
	return false
}

// DatabaseSchema represents a detected database in the project.
type DatabaseSchema struct {
	Type       string          `json:"type"`        // "prisma", "drizzle", "sqlalchemy", "supabase", "firebase", "sqlite"
	SchemaFile string          `json:"schema_file"` // path to schema definition (may be empty for inferred schemas)
	Provider   string          `json:"provider"`    // "postgresql", "mysql", "sqlite", "firestore", etc.
	Tables     []DatabaseTable `json:"tables"`
	Confidence string          `json:"confidence"` // "high", "medium", "low"
}

// DatabaseTable represents a single table/collection in the schema.
type DatabaseTable struct {
	Name       string   `json:"name"`
	File       string   `json:"file"` // where the table is defined
	Line       int      `json:"line"`
	Fields     []string `json:"fields,omitempty"`      // optional: field names for future use
	IsInferred bool     `json:"is_inferred,omitempty"` // true if inferred from usage (Supabase, Firebase)

	// VariableName is the identifier query code refers to this table by,
	// e.g. "users" in `export const users = pgTable("app_users", ...)`. Not
	// always equal to Name — detector-internal, not serialized.
	VariableName string `json:"-"`
}

// DatabaseQuery represents a query call from application code to a table.
type DatabaseQuery struct {
	Table      string `json:"table"`
	Operation  string `json:"operation"` // "select", "insert", "update", "delete", "count", "aggregate"
	File       string `json:"file"`
	Line       int    `json:"line"`
	ORM        string `json:"orm"` // "prisma", "drizzle", "sqlalchemy", etc.
	Confidence string `json:"confidence"`
}

// DatabaseDetector detects database schema and query patterns for a specific ORM/tool.
type DatabaseDetector interface {
	// Name returns the detector type identifier (e.g., "prisma", "drizzle").
	Name() string

	// Detect examines the project analysis and returns a DatabaseSchema if this
	// detector's ORM/tool is present. Returns nil if not detected.
	Detect(analysis *ProjectAnalysis) *DatabaseSchema

	// MatchQueries examines file analyses for query calls to tables in the schema.
	// Returns queries matched to specific tables. This runs after Detect when
	// we know which tables exist. files holds each analyzed file's raw source,
	// keyed by the same relative path used in FileInfo.Path, for detectors that
	// need to inspect source text beyond what FileAnalysis captured.
	MatchQueries(analysis *ProjectAnalysis, files map[string][]byte, schema *DatabaseSchema) []DatabaseQuery
}

// DetectDatabases runs the given detectors against analysis and returns the
// detected schemas. Queries returned by each detector's MatchQueries are
// distributed back onto the matching FileInfo entries in analysis.Files.
func DetectDatabases(analysis *ProjectAnalysis, files map[string][]byte, detectors []DatabaseDetector) []DatabaseSchema {
	var schemas []DatabaseSchema

	for _, detector := range detectors {
		schema := detector.Detect(analysis)
		if schema == nil {
			continue
		}

		queries := detector.MatchQueries(analysis, files, schema)
		for _, query := range queries {
			for i := range analysis.Files {
				if analysis.Files[i].Path == query.File {
					analysis.Files[i].DatabaseQueries = append(analysis.Files[i].DatabaseQueries, query)
					break
				}
			}
		}

		schemas = append(schemas, *schema)
	}

	return schemas
}
