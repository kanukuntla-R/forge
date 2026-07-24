package analyzer_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

func detectDrizzle(t *testing.T, dir string) *analyzer.DatabaseSchema {
	t.Helper()
	analysis, _, err := analyzer.AnalyzeProject(dir)
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}
	for _, db := range analysis.Databases {
		if db.Type == "drizzle" {
			return &db
		}
	}
	return nil
}

// ── Schema parsing ──────────────────────────────────────────────────────────

func TestParsePgTable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "db/schema.ts", `import { pgTable, serial, text } from "drizzle-orm/pg-core"

export const users = pgTable("users", {
  id: serial("id").primaryKey(),
  email: text("email").notNull(),
})
`)
	schema := detectDrizzle(t, dir)
	if schema == nil {
		t.Fatal("expected schema, got nil")
	}
	if len(schema.Tables) != 1 || schema.Tables[0].Name != "users" {
		t.Fatalf("expected 1 table users, got %+v", schema.Tables)
	}
}

func TestParseSqliteTable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "db/schema.ts", `import { sqliteTable, integer, text } from "drizzle-orm/sqlite-core"

export const posts = sqliteTable("posts", {
  id: integer("id").primaryKey(),
})
`)
	schema := detectDrizzle(t, dir)
	if schema == nil || schema.Provider != "sqlite" {
		t.Fatalf("expected provider sqlite, got %+v", schema)
	}
}

func TestParseMysqlTable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "db/schema.ts", `import { mysqlTable, int, varchar } from "drizzle-orm/mysql-core"

export const products = mysqlTable("products", {
  id: int("id").primaryKey(),
})
`)
	schema := detectDrizzle(t, dir)
	if schema == nil || schema.Provider != "mysql" {
		t.Fatalf("expected provider mysql, got %+v", schema)
	}
}

func TestParseMultipleTables(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "db/schema.ts", `import { pgTable, serial } from "drizzle-orm/pg-core"

export const users = pgTable("users", { id: serial("id").primaryKey() })
export const posts = pgTable("posts", { id: serial("id").primaryKey() })
export const comments = pgTable("comments", { id: serial("id").primaryKey() })
`)
	schema := detectDrizzle(t, dir)
	if schema == nil || len(schema.Tables) != 3 {
		t.Fatalf("expected 3 tables, got %+v", schema)
	}
}

func TestParseNoDrizzleSchema(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.ts", `export function hello() { return "hi" }`)
	schema := detectDrizzle(t, dir)
	if schema != nil {
		t.Fatalf("expected nil schema, got %+v", schema)
	}
}

func TestVariableNameMatching(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "db/schema.ts", `import { pgTable, serial } from "drizzle-orm/pg-core"

export const users = pgTable("users", { id: serial("id").primaryKey() })
`)
	schema := detectDrizzle(t, dir)
	if schema == nil || len(schema.Tables) != 1 {
		t.Fatalf("expected 1 table, got %+v", schema)
	}
	if schema.Tables[0].Name != "users" {
		t.Errorf("expected table name users, got %q", schema.Tables[0].Name)
	}
}

func TestVariableAndTableNamesDiffer(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "db/schema.ts", `import { pgTable, serial } from "drizzle-orm/pg-core"

export const usersTable = pgTable("app_users", { id: serial("id").primaryKey() })
`)
	schema := detectDrizzle(t, dir)
	if schema == nil || len(schema.Tables) != 1 {
		t.Fatalf("expected 1 table, got %+v", schema)
	}
	if schema.Tables[0].Name != "app_users" {
		t.Errorf("expected table name app_users (the literal), got %q", schema.Tables[0].Name)
	}

	// Query matching resolves through the variable name (usersTable), not the
	// table string ("app_users") — verified end-to-end via query detection.
	writeFile(t, dir, "app/page.tsx", `import { db, usersTable } from "@/db"
const rows = await db.select().from(usersTable)`)
	analysis, _, err := analyzer.AnalyzeProject(dir)
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Table != "app_users" {
		t.Errorf("expected query resolved to table app_users via variable name, got %+v", q)
	}
}

// ── Client / schema-presence detection ──────────────────────────────────────

func TestDetectDrizzleClientInAnyFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "db/schema.ts", `import { pgTable, serial } from "drizzle-orm/pg-core"
export const users = pgTable("users", { id: serial("id").primaryKey() })`)
	writeFile(t, dir, "app/page.tsx", `const rows = await db.select().from(users)`)
	analysis, _, err := analyzer.AnalyzeProject(dir)
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}
	f := fileByPath(analysis.Files, "app/page.tsx")
	if f == nil || len(f.DatabaseQueries) != 1 {
		t.Fatalf("expected 1 query, got %+v", f)
	}
}

func TestNoClientNoDrizzle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "db/schema.ts", `import { pgTable, serial } from "drizzle-orm/pg-core"
export const users = pgTable("users", { id: serial("id").primaryKey() })`)
	schema := detectDrizzle(t, dir)
	if schema == nil {
		t.Fatal("expected schema detected from schema file alone, got nil")
	}
}

// ── Query detection ──────────────────────────────────────────────────────────

const usersPostsDrizzleSchema = `import { pgTable, serial, text } from "drizzle-orm/pg-core"

export const users = pgTable("users", { id: serial("id").primaryKey() })
export const posts = pgTable("posts", { id: serial("id").primaryKey() })
`

func analyzeWithDrizzleSchema(t *testing.T, dir string) *analyzer.ProjectAnalysis {
	t.Helper()
	writeFile(t, dir, "db/schema.ts", usersPostsDrizzleSchema)
	analysis, _, err := analyzer.AnalyzeProject(dir)
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}
	return analysis
}

func TestDetectSelect(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const rows = await db.select().from(users)`)
	analysis := analyzeWithDrizzleSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Table != "users" || q.Operation != "select" {
		t.Errorf("unexpected query: %+v", q)
	}
}

func TestDetectInsert(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const row = await db.insert(users).values({ name: "x" })`)
	analysis := analyzeWithDrizzleSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Table != "users" || q.Operation != "insert" {
		t.Errorf("unexpected query: %+v", q)
	}
}

func TestDetectDrizzleUpdate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `await db.update(users).set({ name: "x" }).where(eq(users.id, 1))`)
	analysis := analyzeWithDrizzleSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Table != "users" || q.Operation != "update" {
		t.Errorf("unexpected query: %+v", q)
	}
}

func TestDetectDrizzleDelete(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `await db.delete(users).where(eq(users.id, 1))`)
	analysis := analyzeWithDrizzleSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Table != "users" || q.Operation != "delete" {
		t.Errorf("unexpected query: %+v", q)
	}
}

func TestDetectQueryPattern(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const rows = await db.query.users.findMany()`)
	analysis := analyzeWithDrizzleSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Table != "users" || q.Operation != "select" {
		t.Errorf("unexpected query: %+v", q)
	}
}

func TestUnknownDrizzleTableLowConfidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const rows = await db.select().from(unknownTable)`)
	analysis := analyzeWithDrizzleSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Confidence != "low" || q.Table != "unknownTable" {
		t.Errorf("expected low confidence with placeholder table, got %+v", q)
	}
}

func TestSelectWithWhere(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const rows = await db.select().from(users).where(eq(users.id, 1))`)
	analysis := analyzeWithDrizzleSchema(t, dir)
	f := fileByPath(analysis.Files, "app/page.tsx")
	if len(f.DatabaseQueries) != 1 || f.DatabaseQueries[0].Table != "users" {
		t.Errorf("expected 1 query on users despite trailing where(), got %+v", f.DatabaseQueries)
	}
}

func TestJoinIgnored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const rows = await db.select().from(users).leftJoin(posts, eq(users.id, posts.authorId))`)
	analysis := analyzeWithDrizzleSchema(t, dir)
	f := fileByPath(analysis.Files, "app/page.tsx")
	if len(f.DatabaseQueries) != 1 || f.DatabaseQueries[0].Table != "users" {
		t.Errorf("expected primary table users detected, joins ignored, got %+v", f.DatabaseQueries)
	}
}

func TestUncommonDrizzleClientMediumConfidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const rows = await myClient.select().from(users)`)
	analysis := analyzeWithDrizzleSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Table != "users" || q.Confidence != "medium" {
		t.Errorf("expected uncommon client name to be medium confidence, got %+v", q)
	}
}

func TestNonDrizzleCallNotDetected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const r = await axios.get("/api/users")`)
	analysis := analyzeWithDrizzleSchema(t, dir)
	f := fileByPath(analysis.Files, "app/page.tsx")
	if len(f.DatabaseQueries) != 0 {
		t.Errorf("expected no queries from a non-Drizzle call, got %+v", f.DatabaseQueries)
	}
}

func TestSkipDrizzleTestFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.test.ts", `const rows = await db.select().from(users)`)
	analysis := analyzeWithDrizzleSchema(t, dir)
	f := fileByPath(analysis.Files, "app/page.test.ts")
	if f != nil && len(f.DatabaseQueries) != 0 {
		t.Errorf("expected test file queries to be skipped, got %+v", f.DatabaseQueries)
	}
}

// ── Provider detection ───────────────────────────────────────────────────────

func TestProviderPostgres(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "db/schema.ts", `import { pgTable, serial } from "drizzle-orm/pg-core"
export const users = pgTable("users", { id: serial("id").primaryKey() })`)
	schema := detectDrizzle(t, dir)
	if schema == nil || schema.Provider != "postgresql" {
		t.Fatalf("expected provider postgresql, got %+v", schema)
	}
}

func TestProviderSqlite(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "db/schema.ts", `import { sqliteTable, integer } from "drizzle-orm/sqlite-core"
export const users = sqliteTable("users", { id: integer("id").primaryKey() })`)
	schema := detectDrizzle(t, dir)
	if schema == nil || schema.Provider != "sqlite" {
		t.Fatalf("expected provider sqlite, got %+v", schema)
	}
}

func TestMultipleProviders(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "db/schema.ts", `import { pgTable, sqliteTable, serial, integer } from "drizzle-orm/pg-core"
export const users = pgTable("users", { id: serial("id").primaryKey() })
export const legacy = sqliteTable("legacy", { id: integer("id").primaryKey() })`)
	schema := detectDrizzle(t, dir)
	if schema == nil || schema.Provider != "postgresql" {
		t.Fatalf("expected first-detected provider postgresql, got %+v", schema)
	}
}

// ── Integration ──────────────────────────────────────────────────────────────

// TestRealDemoShop2Drizzle mirrors demo-shop2's Drizzle side (comments/tags,
// Prisma handles users/posts) as a self-contained temp-dir project, since
// demo-shop2 itself is gitignored and not present in a fresh checkout.
func TestRealDemoShop2Drizzle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prisma/schema.prisma", userPostSchema)
	writeFile(t, dir, "db/schema.ts", `import { pgTable, serial, text, integer, timestamp, boolean } from "drizzle-orm/pg-core"

export const comments = pgTable("comments", {
  id: serial("id").primaryKey(),
  postId: integer("post_id").notNull(),
  authorName: text("author_name").notNull(),
  content: text("content").notNull(),
  approved: boolean("approved").default(false),
  createdAt: timestamp("created_at").defaultNow().notNull(),
})

export const tags = pgTable("tags", {
  id: serial("id").primaryKey(),
  name: text("name").notNull().unique(),
  slug: text("slug").notNull().unique(),
  createdAt: timestamp("created_at").defaultNow().notNull(),
})
`)
	writeFile(t, dir, "db/index.ts", `import { drizzle } from "drizzle-orm/postgres-js"
import postgres from "postgres"
import * as schema from "./schema"

const client = postgres(process.env.DATABASE_URL ?? "")
export const db = drizzle(client, { schema })
export * from "./schema"
`)
	writeFile(t, dir, "app/api/comments/route.ts", `import { NextResponse } from "next/server"
import { db, comments } from "@/db"
import { eq } from "drizzle-orm"

export async function GET(request: Request) {
  const url = new URL(request.url)
  const postId = url.searchParams.get("postId")

  if (postId) {
    const result = await db
      .select()
      .from(comments)
      .where(eq(comments.postId, parseInt(postId)))
    return NextResponse.json(result)
  }

  const all = await db.select().from(comments)
  return NextResponse.json(all)
}

export async function POST(request: Request) {
  const body = await request.json()
  const [comment] = await db
    .insert(comments)
    .values({
      postId: body.postId,
      authorName: body.authorName,
      content: body.content,
    })
    .returning()
  return NextResponse.json(comment)
}
`)
	writeFile(t, dir, "app/tags/page.tsx", `import { db, tags } from "@/db"

export default async function TagsPage() {
  const allTags = await db.select().from(tags)
  return null
}
`)
	writeFile(t, dir, "app/posts/[id]/page.tsx", `import { prisma } from "@/lib/prisma"
import { db, comments } from "@/db"
import { eq } from "drizzle-orm"

export default async function PostDetailPage({ params }: { params: { id: string } }) {
  const postId = parseInt(params.id)
  const post = await prisma.post.findUnique({ where: { id: postId } })
  const postComments = await db.select().from(comments).where(eq(comments.postId, postId))
  return null
}
`)

	analysis, _, err := analyzer.AnalyzeProject(dir)
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}

	if len(analysis.Databases) != 2 {
		t.Fatalf("expected 2 databases (prisma + drizzle), got %d: %+v", len(analysis.Databases), analysis.Databases)
	}

	var drizzleDB *analyzer.DatabaseSchema
	for i := range analysis.Databases {
		if analysis.Databases[i].Type == "drizzle" {
			drizzleDB = &analysis.Databases[i]
		}
	}
	if drizzleDB == nil || len(drizzleDB.Tables) != 2 {
		t.Fatalf("expected drizzle schema with 2 tables, got %+v", drizzleDB)
	}

	total := 0
	drizzleTotal := 0
	for _, f := range analysis.Files {
		total += len(f.DatabaseQueries)
		for _, q := range f.DatabaseQueries {
			if q.ORM == "drizzle" {
				drizzleTotal++
			}
		}
	}
	if drizzleTotal != 5 {
		t.Errorf("expected 5 Drizzle queries, got %d", drizzleTotal)
	}
	// posts/[id]/page.tsx also calls prisma.post.findUnique(...), one Prisma query.
	if total != 6 {
		t.Errorf("expected 6 total queries (5 drizzle + 1 prisma), got %d", total)
	}
}
