package analyzer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// drizzleDetector implements DatabaseDetector for Drizzle ORM.
type drizzleDetector struct{}

// NewDrizzleDetector returns a DatabaseDetector for Drizzle.
func NewDrizzleDetector() DatabaseDetector {
	return &drizzleDetector{}
}

func (d *drizzleDetector) Name() string { return "drizzle" }

// drizzleTableFns maps a Drizzle table-definition call to the provider it
// implies.
var drizzleTableFns = map[string]string{
	"pgTable":     "postgresql",
	"sqliteTable": "sqlite",
	"mysqlTable":  "mysql",
}

// drizzleTableRe matches `export const <var> = <fn>("<table>", {` — Drizzle's
// schema definitions are unlike Prisma's (no separate schema file; tables are
// plain TypeScript exports), but the shape is just as regular, so regex over
// the raw source is enough — no need for the tree-sitter AST here.
var drizzleTableRe = regexp.MustCompile(`(?:export\s+)?const\s+(\w+)\s*=\s*(pgTable|sqliteTable|mysqlTable)\(\s*["'` + "`" + `](\w+)["'` + "`" + `]`)

// Detect scans every TypeScript file for pgTable/sqliteTable/mysqlTable
// declarations. Unlike Prisma there's no single schema file to check first —
// Drizzle schemas can live anywhere, so every file is a candidate.
func (d *drizzleDetector) Detect(analysis *ProjectAnalysis) *DatabaseSchema {
	var tables []DatabaseTable
	provider := ""

	for _, file := range analysis.Files {
		if file.Language != "typescript" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(analysis.Project.Root, file.Path))
		if err != nil {
			continue
		}
		text := string(content)
		for _, m := range drizzleTableRe.FindAllStringSubmatchIndex(text, -1) {
			variable := text[m[2]:m[3]]
			fn := text[m[4]:m[5]]
			tableName := text[m[6]:m[7]]
			line := 1 + strings.Count(text[:m[0]], "\n")

			if provider == "" {
				provider = drizzleTableFns[fn]
			}
			tables = append(tables, DatabaseTable{
				Name:         tableName,
				File:         file.Path,
				Line:         line,
				VariableName: variable,
			})
		}
	}

	if len(tables) == 0 {
		return nil
	}
	return &DatabaseSchema{
		Type:       "drizzle",
		Provider:   provider,
		Tables:     tables,
		Confidence: "high",
	}
}

// commonDrizzleClientNames are the variable names conventionally used to hold
// a Drizzle client (the official docs use "db"; "database"/"drizzle" show up
// too). A chain rooted at one of these is high confidence.
var commonDrizzleClientNames = map[string]bool{
	"db":       true,
	"database": true,
	"drizzle":  true,
}

var drizzleWriteOps = map[string]string{
	"insert": "insert",
	"update": "update",
	"delete": "delete",
}

var drizzleQueryMethods = map[string]bool{
	"findMany":  true,
	"findFirst": true,
}

// findDrizzleTable resolves a chain's table reference (a variable name) to
// the schema table it names. Falls back to the raw variable name as a
// placeholder when nothing matches.
func findDrizzleTable(variable string, tables []DatabaseTable) (name string, matched bool) {
	for _, t := range tables {
		if t.VariableName == variable {
			return t.Name, true
		}
	}
	return variable, false
}

func drizzleConfidence(root string, matched bool) string {
	switch {
	case !matched:
		return "low"
	case commonDrizzleClientNames[root]:
		return "high"
	default:
		return "medium"
	}
}

// MatchQueries walks TypeScript files' pre-extracted MethodChains for
// Drizzle's three query shapes:
//
//	db.select().from(<table>)             -> select
//	db.insert/update/delete(<table>)...   -> insert/update/delete
//	db.query.<table>.findMany/findFirst() -> select
//
// Joins, where clauses, and other trailing segments are ignored — only the
// primary table is recorded. Confidence mirrors Prisma's ladder: low when the
// table reference doesn't match a known schema table, otherwise high/medium
// based on whether the root variable has a conventional Drizzle client name.
func (d *drizzleDetector) MatchQueries(analysis *ProjectAnalysis, _ map[string][]byte, schema *DatabaseSchema) []DatabaseQuery {
	var queries []DatabaseQuery

	for _, file := range analysis.Files {
		if file.Language != "typescript" || isORMTestFile(file.Path) {
			continue
		}
		for _, chain := range file.MethodChains {
			table, op, ok := matchDrizzleChain(chain)
			if !ok {
				continue
			}
			tableName, matched := findDrizzleTable(table, schema.Tables)
			queries = append(queries, DatabaseQuery{
				Table:      tableName,
				Operation:  op,
				File:       file.Path,
				Line:       chain.Line,
				ORM:        "drizzle",
				Confidence: drizzleConfidence(chain.Root, matched),
			})
		}
	}
	return queries
}

// matchDrizzleChain recognizes Drizzle's query shapes over a flattened
// MethodChain and returns the referenced table variable and operation.
func matchDrizzleChain(chain MethodChain) (table, op string, ok bool) {
	calls := chain.Calls
	if len(calls) == 0 {
		return "", "", false
	}

	switch calls[0].Name {
	case "select":
		if len(calls) >= 2 && calls[1].Name == "from" && len(calls[1].Args) >= 1 {
			return calls[1].Args[0], "select", true
		}
	case "insert", "update", "delete":
		if len(calls[0].Args) >= 1 {
			return calls[0].Args[0], drizzleWriteOps[calls[0].Name], true
		}
	case "query":
		if len(calls) == 3 && drizzleQueryMethods[calls[2].Name] {
			return calls[1].Name, "select", true
		}
	}
	return "", "", false
}
