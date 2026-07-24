package analyzer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// prismaDetector implements DatabaseDetector for Prisma ORM.
type prismaDetector struct{}

// NewPrismaDetector returns a DatabaseDetector for Prisma.
func NewPrismaDetector() DatabaseDetector {
	return &prismaDetector{}
}

func (p *prismaDetector) Name() string { return "prisma" }

// prismaSchemaPaths are checked in order; the first one found on disk wins.
var prismaSchemaPaths = []string{
	filepath.Join("prisma", "schema.prisma"),
	"schema.prisma",
}

// Detect looks for a Prisma schema file at the conventional locations and, if
// found, parses it via regex — schema.prisma has a predictable, line-oriented
// format that doesn't warrant a real parser.
func (p *prismaDetector) Detect(analysis *ProjectAnalysis) *DatabaseSchema {
	for _, rel := range prismaSchemaPaths {
		content, err := os.ReadFile(filepath.Join(analysis.Project.Root, rel))
		if err != nil {
			continue
		}
		return &DatabaseSchema{
			Type:       "prisma",
			SchemaFile: rel,
			Provider:   parsePrismaProvider(string(content)),
			Tables:     parsePrismaModels(string(content), rel),
			Confidence: "high",
		}
	}
	return nil
}

var prismaModelRe = regexp.MustCompile(`model\s+(\w+)\s*\{`)

// parsePrismaModels extracts model declarations from schema.prisma content.
func parsePrismaModels(content, file string) []DatabaseTable {
	tables := []DatabaseTable{}
	for _, m := range prismaModelRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := 1 + strings.Count(content[:m[0]], "\n")
		tables = append(tables, DatabaseTable{Name: name, File: file, Line: line})
	}
	return tables
}

var (
	prismaDatasourceRe = regexp.MustCompile(`datasource\s+\w+\s*\{([^}]*)\}`)
	prismaProviderRe   = regexp.MustCompile(`provider\s*=\s*"(\w+)"`)
)

// parsePrismaProvider extracts the provider from the datasource block —
// distinct from the generator block, which also has its own provider= line.
func parsePrismaProvider(content string) string {
	ds := prismaDatasourceRe.FindStringSubmatch(content)
	if ds == nil {
		return ""
	}
	m := prismaProviderRe.FindStringSubmatch(ds[1])
	if m == nil {
		return ""
	}
	return m[1]
}

// prismaReadMethods, prismaWriteMethods, and prismaOtherMethods partition the
// Prisma client method surface. isPrismaMethod checks membership across all
// three; categorizePrismaOperation maps a method to a DatabaseQuery operation.
var prismaReadMethods = map[string]bool{
	"findUnique": true, "findUniqueOrThrow": true, "findFirst": true, "findFirstOrThrow": true,
	"findMany": true, "count": true, "aggregate": true, "groupBy": true,
}

var prismaWriteMethods = map[string]bool{
	"create": true, "createMany": true, "createManyAndReturn": true,
	"update": true, "updateMany": true, "upsert": true,
	"delete": true, "deleteMany": true,
}

var prismaOtherMethods = map[string]bool{
	"executeRaw": true, "queryRaw": true, "findRaw": true, "aggregateRaw": true, "$transaction": true,
}

func isPrismaMethod(method string) bool {
	return prismaReadMethods[method] || prismaWriteMethods[method] || prismaOtherMethods[method]
}

func categorizePrismaOperation(method string) string {
	switch method {
	case "findUnique", "findUniqueOrThrow", "findFirst", "findFirstOrThrow", "findMany":
		return "select"
	case "count", "aggregate", "groupBy":
		return "aggregate"
	case "create", "createMany", "createManyAndReturn":
		return "insert"
	case "update", "updateMany", "upsert":
		return "update"
	case "delete", "deleteMany":
		return "delete"
	default:
		return "other"
	}
}

// commonPrismaClientNames are the variable names conventionally used to hold
// a PrismaClient instance (the official docs use "prisma"; "db"/"client" are
// common in the wild). A chained call through one of these names is high
// confidence without needing to trace the actual `new PrismaClient()` call.
//
// ponytail: name heuristic, not real instantiation tracing — a variable named
// "db" that isn't actually PrismaClient would false-positive. Upgrade to
// import/assignment tracing if that becomes a problem (M11.3+).
var commonPrismaClientNames = map[string]bool{
	"prisma": true,
	"db":     true,
	"client": true,
}

// isPrismaTestFile mirrors the exclusion used for HTTP call detection: skip
// *.test.ts(x)/*.spec.ts(x) and anything under __tests__/, test/, tests/.
func isPrismaTestFile(path string) bool {
	base := filepath.Base(path)
	for _, suf := range []string{".test.ts", ".test.tsx", ".spec.ts", ".spec.tsx"} {
		if strings.HasSuffix(base, suf) {
			return true
		}
	}
	slashed := "/" + filepath.ToSlash(path) + "/"
	for _, seg := range []string{"/__tests__/", "/test/", "/tests/"} {
		if strings.Contains(slashed, seg) {
			return true
		}
	}
	return false
}

// matchPrismaTable finds the schema table matching a call's model property.
// Prisma exposes PascalCase models as camelCase client properties (User ->
// prisma.user, BlogPost -> prisma.blogPost), so the match is case-insensitive.
// Falls back to the raw property name as a placeholder when nothing matches.
func matchPrismaTable(property string, tables []DatabaseTable) (name string, matched bool) {
	for _, t := range tables {
		if strings.EqualFold(t.Name, property) {
			return t.Name, true
		}
	}
	return property, false
}

// MatchQueries walks TypeScript files' pre-extracted ChainedCalls for
// Prisma-style client.model.method() calls and matches them against the
// schema's tables. Confidence: "high" when the client variable has a
// conventional PrismaClient name (prisma/db/client) and the table matches,
// "medium" when the table matches but the variable name is unrecognized,
// "low" when the method is a known Prisma method but the model property
// doesn't match any table.
func (p *prismaDetector) MatchQueries(analysis *ProjectAnalysis, _ map[string][]byte, schema *DatabaseSchema) []DatabaseQuery {
	var queries []DatabaseQuery

	for _, file := range analysis.Files {
		if file.Language != "typescript" || isPrismaTestFile(file.Path) {
			continue
		}
		for _, call := range file.ChainedCalls {
			if !isPrismaMethod(call.Method) {
				continue
			}

			table, matched := matchPrismaTable(call.Property, schema.Tables)

			var confidence string
			switch {
			case !matched:
				confidence = "low"
			case commonPrismaClientNames[call.Object]:
				confidence = "high"
			default:
				confidence = "medium"
			}

			queries = append(queries, DatabaseQuery{
				Table:      table,
				Operation:  categorizePrismaOperation(call.Method),
				File:       file.Path,
				Line:       call.Line,
				ORM:        "prisma",
				Confidence: confidence,
			})
		}
	}
	return queries
}
