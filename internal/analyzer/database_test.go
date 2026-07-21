package analyzer_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

// stubDatabaseDetector is a test-only detector for exercising the database
// detection plumbing without real ORM detection logic.
type stubDatabaseDetector struct {
	schema  *analyzer.DatabaseSchema
	queries []analyzer.DatabaseQuery
}

func (s *stubDatabaseDetector) Name() string { return "stub" }

func (s *stubDatabaseDetector) Detect(_ *analyzer.ProjectAnalysis) *analyzer.DatabaseSchema {
	return s.schema
}

func (s *stubDatabaseDetector) MatchQueries(_ *analyzer.ProjectAnalysis, _ *analyzer.DatabaseSchema) []analyzer.DatabaseQuery {
	return s.queries
}

func TestDatabaseSchema_JSONSerialization(t *testing.T) {
	schema := analyzer.DatabaseSchema{
		Type:       "prisma",
		SchemaFile: "prisma/schema.prisma",
		Provider:   "postgresql",
		Tables:     []analyzer.DatabaseTable{{Name: "User", File: "prisma/schema.prisma", Line: 5}},
		Confidence: "high",
	}
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"type":"prisma"`) {
		t.Errorf("expected type field in JSON, got %s", data)
	}
}

func TestDatabaseTable_InferredFlag(t *testing.T) {
	inferred := analyzer.DatabaseTable{Name: "posts", IsInferred: true}
	data, err := json.Marshal(inferred)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"is_inferred":true`) {
		t.Errorf("expected is_inferred:true in JSON, got %s", data)
	}

	notInferred := analyzer.DatabaseTable{Name: "posts"}
	data, err = json.Marshal(notInferred)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "is_inferred") {
		t.Errorf("expected is_inferred omitted when false, got %s", data)
	}
}

func TestDetectDatabases_NoDetectors(t *testing.T) {
	analysis := &analyzer.ProjectAnalysis{}
	schemas := analyzer.DetectDatabases(analysis, nil)
	if len(schemas) != 0 {
		t.Errorf("expected no schemas, got %d", len(schemas))
	}
}

func TestDetectDatabases_StubReturnsNil(t *testing.T) {
	analysis := &analyzer.ProjectAnalysis{}
	detectors := []analyzer.DatabaseDetector{&stubDatabaseDetector{}}
	schemas := analyzer.DetectDatabases(analysis, detectors)
	if len(schemas) != 0 {
		t.Errorf("expected no schemas from nil-returning stub, got %d", len(schemas))
	}
}

func TestDetectDatabases_StubWithSchema(t *testing.T) {
	analysis := &analyzer.ProjectAnalysis{}
	schema := &analyzer.DatabaseSchema{Type: "stub", Confidence: "low"}
	detectors := []analyzer.DatabaseDetector{&stubDatabaseDetector{schema: schema}}
	schemas := analyzer.DetectDatabases(analysis, detectors)
	if len(schemas) != 1 || schemas[0].Type != "stub" {
		t.Errorf("expected one stub schema, got %+v", schemas)
	}
}

func TestDetectDatabases_QueriesDistributedToFiles(t *testing.T) {
	analysis := &analyzer.ProjectAnalysis{
		Files: []analyzer.FileInfo{
			{Path: "src/a.ts"},
			{Path: "src/b.ts"},
		},
	}
	schema := &analyzer.DatabaseSchema{Type: "stub"}
	queries := []analyzer.DatabaseQuery{
		{Table: "users", Operation: "select", File: "src/b.ts", Line: 3, ORM: "stub"},
	}
	detectors := []analyzer.DatabaseDetector{&stubDatabaseDetector{schema: schema, queries: queries}}

	analyzer.DetectDatabases(analysis, detectors)

	if len(analysis.Files[0].DatabaseQueries) != 0 {
		t.Errorf("expected src/a.ts to have no queries, got %+v", analysis.Files[0].DatabaseQueries)
	}
	if len(analysis.Files[1].DatabaseQueries) != 1 || analysis.Files[1].DatabaseQueries[0].Table != "users" {
		t.Errorf("expected src/b.ts to have the users query, got %+v", analysis.Files[1].DatabaseQueries)
	}
}

func TestFileAnalysis_QueriesOmittedWhenEmpty(t *testing.T) {
	fi := analyzer.FileInfo{Path: "src/a.ts"}
	data, err := json.Marshal(fi)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "database_queries") {
		t.Errorf("expected database_queries omitted when empty, got %s", data)
	}
}

func TestProjectAnalysis_DatabasesOmittedWhenEmpty(t *testing.T) {
	pa := analyzer.ProjectAnalysis{}
	data, err := json.Marshal(pa)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "databases") {
		t.Errorf("expected databases omitted when empty, got %s", data)
	}
}
