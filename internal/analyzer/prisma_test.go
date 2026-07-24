package analyzer_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

func detectPrisma(t *testing.T, dir string) *analyzer.DatabaseSchema {
	t.Helper()
	return analyzer.NewPrismaDetector().Detect(&analyzer.ProjectAnalysis{
		Project: analyzer.ProjectInfo{Root: dir},
	})
}

func fileByPath(files []analyzer.FileInfo, path string) *analyzer.FileInfo {
	for i := range files {
		if files[i].Path == path {
			return &files[i]
		}
	}
	return nil
}

// ── Schema parsing ──────────────────────────────────────────────────────────

func TestParseSimpleSchema(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prisma/schema.prisma", `
model User {
  id    Int    @id
  email String
}

model Post {
  id    Int    @id
  title String
}
`)
	schema := detectPrisma(t, dir)
	if schema == nil {
		t.Fatal("expected schema, got nil")
	}
	if len(schema.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d: %+v", len(schema.Tables), schema.Tables)
	}
	if schema.Tables[0].Name != "User" || schema.Tables[1].Name != "Post" {
		t.Errorf("unexpected table names: %+v", schema.Tables)
	}
}

func TestParseEmptySchema(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prisma/schema.prisma", `
generator client {
  provider = "prisma-client-js"
}
`)
	schema := detectPrisma(t, dir)
	if schema == nil {
		t.Fatal("expected schema (file exists), got nil")
	}
	if len(schema.Tables) != 0 {
		t.Errorf("expected empty tables list, got %+v", schema.Tables)
	}
}

func TestParseModelWithRelations(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prisma/schema.prisma", `
model Post {
  id       Int    @id
  author   User   @relation(fields: [authorId], references: [id])
  authorId Int
}
`)
	schema := detectPrisma(t, dir)
	if schema == nil || len(schema.Tables) != 1 || schema.Tables[0].Name != "Post" {
		t.Fatalf("expected 1 table Post, got %+v", schema)
	}
}

func TestParseSchemaWithComments(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prisma/schema.prisma", `
// This is the user model
model User {
  id Int @id // primary key
}
`)
	schema := detectPrisma(t, dir)
	if schema == nil || len(schema.Tables) != 1 || schema.Tables[0].Name != "User" {
		t.Fatalf("expected 1 table User, got %+v", schema)
	}
}

// ── Schema location ─────────────────────────────────────────────────────────

func TestFindSchemaInPrismaDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prisma/schema.prisma", `model User { id Int @id }`)
	schema := detectPrisma(t, dir)
	if schema == nil || schema.SchemaFile != "prisma/schema.prisma" {
		t.Fatalf("expected schema found at prisma/schema.prisma, got %+v", schema)
	}
}

func TestFindSchemaAtRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "schema.prisma", `model User { id Int @id }`)
	schema := detectPrisma(t, dir)
	if schema == nil || schema.SchemaFile != "schema.prisma" {
		t.Fatalf("expected schema found at root schema.prisma, got %+v", schema)
	}
}

func TestNoSchemaFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "no prisma here")
	schema := detectPrisma(t, dir)
	if schema != nil {
		t.Fatalf("expected nil schema, got %+v", schema)
	}
}

// ── Query detection ──────────────────────────────────────────────────────────

const userPostSchema = `
model User {
  id    Int    @id
  email String
}

model Post {
  id    Int    @id
  title String
}
`

func analyzeWithPrismaSchema(t *testing.T, dir string) *analyzer.ProjectAnalysis {
	t.Helper()
	writeFile(t, dir, "prisma/schema.prisma", userPostSchema)
	analysis, _, err := analyzer.AnalyzeProject(dir)
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}
	return analysis
}

func TestDetectFindMany(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `export default async function Page() {
  const users = await prisma.user.findMany()
  return users
}`)
	analysis := analyzeWithPrismaSchema(t, dir)
	f := fileByPath(analysis.Files, "app/page.tsx")
	if f == nil || len(f.DatabaseQueries) != 1 {
		t.Fatalf("expected 1 query, got %+v", f)
	}
	q := f.DatabaseQueries[0]
	if q.Table != "User" || q.Operation != "select" {
		t.Errorf("unexpected query: %+v", q)
	}
}

func TestDetectCreate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const u = await prisma.user.create({ data: {} })`)
	analysis := analyzeWithPrismaSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Operation != "insert" {
		t.Errorf("expected insert, got %+v", q)
	}
}

func TestDetectUpdate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const u = await prisma.user.update({ where: {}, data: {} })`)
	analysis := analyzeWithPrismaSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Operation != "update" {
		t.Errorf("expected update, got %+v", q)
	}
}

func TestDetectDelete(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const u = await prisma.user.delete({ where: {} })`)
	analysis := analyzeWithPrismaSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Operation != "delete" {
		t.Errorf("expected delete, got %+v", q)
	}
}

func TestDetectMultipleQueriesInFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `
const users = await prisma.user.findMany()
const post = await prisma.post.create({ data: {} })
`)
	analysis := analyzeWithPrismaSchema(t, dir)
	f := fileByPath(analysis.Files, "app/page.tsx")
	if len(f.DatabaseQueries) != 2 {
		t.Fatalf("expected 2 queries, got %+v", f.DatabaseQueries)
	}
}

func TestDetectQueriesAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/users/page.tsx", `const u = await prisma.user.findMany()`)
	writeFile(t, dir, "app/posts/page.tsx", `const p = await prisma.post.findMany()`)
	analysis := analyzeWithPrismaSchema(t, dir)
	if len(fileByPath(analysis.Files, "app/users/page.tsx").DatabaseQueries) != 1 {
		t.Error("expected 1 query in users/page.tsx")
	}
	if len(fileByPath(analysis.Files, "app/posts/page.tsx").DatabaseQueries) != 1 {
		t.Error("expected 1 query in posts/page.tsx")
	}
}

func TestClientVariableDb(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const u = await db.user.findMany()`)
	analysis := analyzeWithPrismaSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Table != "User" || q.Confidence != "high" {
		t.Errorf("expected db.user.findMany() (common client name) to be high confidence, got %+v", q)
	}
}

func TestClientVariableClient(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const u = await client.user.findMany()`)
	analysis := analyzeWithPrismaSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Table != "User" || q.Confidence != "high" {
		t.Errorf("expected client.user.findMany() (common client name) to be high confidence, got %+v", q)
	}
}

func TestCommonNamePrismaHighConfidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const u = await prisma.user.findMany()`)
	analysis := analyzeWithPrismaSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Confidence != "high" {
		t.Errorf("expected prisma.user.findMany() (common client name) to be high confidence, got %+v", q)
	}
}

func TestUncommonClientVarMediumConfidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const u = await myOrm.user.findMany()`)
	analysis := analyzeWithPrismaSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Table != "User" || q.Confidence != "medium" {
		t.Errorf("expected myOrm.user.findMany() (uncommon client name) to be medium confidence, got %+v", q)
	}
}

func TestUnknownTableLowConfidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const x = await prisma.unknownModel.findMany()`)
	analysis := analyzeWithPrismaSchema(t, dir)
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Confidence != "low" || q.Table != "unknownModel" {
		t.Errorf("expected low confidence with placeholder table, got %+v", q)
	}
}

func TestNonPrismaCallNotDetected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.tsx", `const r = await axios.get("/api/users")`)
	analysis := analyzeWithPrismaSchema(t, dir)
	f := fileByPath(analysis.Files, "app/page.tsx")
	if len(f.DatabaseQueries) != 0 {
		t.Errorf("expected no queries from a non-Prisma call, got %+v", f.DatabaseQueries)
	}
}

func TestSkipTestFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/page.test.ts", `const u = await prisma.user.findMany()`)
	analysis := analyzeWithPrismaSchema(t, dir)
	f := fileByPath(analysis.Files, "app/page.test.ts")
	if f != nil && len(f.DatabaseQueries) != 0 {
		t.Errorf("expected test file queries to be skipped, got %+v", f.DatabaseQueries)
	}
}

func TestCaseInsensitiveTableMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prisma/schema.prisma", `model BlogPost { id Int @id }`)
	writeFile(t, dir, "app/page.tsx", `const p = await prisma.blogPost.findMany()`)
	analysis, _, err := analyzer.AnalyzeProject(dir)
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}
	q := fileByPath(analysis.Files, "app/page.tsx").DatabaseQueries[0]
	if q.Table != "BlogPost" {
		t.Errorf("expected camelCase blogPost to match model BlogPost, got %+v", q)
	}
}

// ── Integration ──────────────────────────────────────────────────────────────

// TestRealDemoShop2 mirrors the demo-shop2 fixture (Prisma for users/posts,
// Drizzle for comments/tags) as a self-contained temp-dir project, since
// demo-shop2 itself is gitignored and not present in a fresh checkout.
func TestRealDemoShop2(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prisma/schema.prisma", `
datasource db {
  provider = "postgresql"
  url      = env("DATABASE_URL")
}

model User {
  id    Int    @id @default(autoincrement())
  email String @unique
  posts Post[]
}

model Post {
  id       Int    @id @default(autoincrement())
  title    String
  authorId Int
  author   User   @relation(fields: [authorId], references: [id])
}
`)
	writeFile(t, dir, "lib/prisma.ts", `import { PrismaClient } from "@prisma/client"
export const prisma = globalForPrisma.prisma ?? new PrismaClient()`)
	writeFile(t, dir, "app/users/page.tsx", `import { prisma } from "@/lib/prisma"
export default async function UsersPage() {
  const users = await prisma.user.findMany({ orderBy: { createdAt: "desc" } })
}`)
	writeFile(t, dir, "app/posts/page.tsx", `import { prisma } from "@/lib/prisma"
export default async function PostsPage() {
  const posts = await prisma.post.findMany({ where: { published: true } })
}`)
	writeFile(t, dir, "app/posts/[id]/page.tsx", `import { prisma } from "@/lib/prisma"
import { db, comments } from "@/db"
import { eq } from "drizzle-orm"
export default async function PostDetailPage({ params }: { params: { id: string } }) {
  const post = await prisma.post.findUnique({ where: { id: parseInt(params.id) } })
  const postComments = await db.select().from(comments).where(eq(comments.postId, post.id))
}`)
	writeFile(t, dir, "app/api/users/route.ts", `import { prisma } from "@/lib/prisma"
export async function GET() {
  const users = await prisma.user.findMany()
}
export async function POST(request: Request) {
  const body = await request.json()
  const user = await prisma.user.create({ data: { email: body.email } })
}`)
	writeFile(t, dir, "app/api/posts/route.ts", `import { prisma } from "@/lib/prisma"
export async function GET() {
  const posts = await prisma.post.findMany({ where: { published: true } })
}
export async function POST(request: Request) {
  const body = await request.json()
  const post = await prisma.post.create({ data: { title: body.title } })
}`)
	writeFile(t, dir, "app/api/comments/route.ts", `import { db, comments } from "@/db"
import { eq } from "drizzle-orm"
export async function GET(request: Request) {
  const all = await db.select().from(comments)
}
export async function POST(request: Request) {
  const body = await request.json()
  const [comment] = await db.insert(comments).values({ content: body.content }).returning()
}`)
	writeFile(t, dir, "app/tags/page.tsx", `import { db, tags } from "@/db"
export default async function TagsPage() {
  const allTags = await db.select().from(tags)
}`)

	analysis, _, err := analyzer.AnalyzeProject(dir)
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}

	if len(analysis.Databases) != 1 {
		t.Fatalf("expected 1 database, got %d: %+v", len(analysis.Databases), analysis.Databases)
	}
	db := analysis.Databases[0]
	if db.Type != "prisma" || db.Provider != "postgresql" || len(db.Tables) != 2 {
		t.Fatalf("unexpected schema: %+v", db)
	}

	total := 0
	for _, f := range analysis.Files {
		total += len(f.DatabaseQueries)
	}
	if total != 7 {
		t.Errorf("expected 7 Prisma queries across the project, got %d", total)
	}

	for _, path := range []string{"app/api/comments/route.ts", "app/tags/page.tsx"} {
		if f := fileByPath(analysis.Files, path); f != nil && len(f.DatabaseQueries) != 0 {
			t.Errorf("expected no Prisma queries (Drizzle-only file) in %s, got %+v", path, f.DatabaseQueries)
		}
	}
}
