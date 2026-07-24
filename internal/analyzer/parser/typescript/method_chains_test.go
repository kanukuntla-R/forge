package typescript_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer/parser/typescript"
)

func parseMethodChains(t *testing.T, src string) []typescript.MethodChain {
	t.Helper()
	p := typescript.NewParser()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return tree.MethodChains()
}

func TestMethodChains_SelectFrom(t *testing.T) {
	chains := parseMethodChains(t, `const rows = await db.select().from(comments)`)
	if len(chains) != 1 {
		t.Fatalf("expected 1 chain, got %d: %+v", len(chains), chains)
	}
	c := chains[0]
	if c.Root != "db" {
		t.Errorf("expected root db, got %q", c.Root)
	}
	if len(c.Calls) != 2 || c.Calls[0].Name != "select" || c.Calls[1].Name != "from" {
		t.Fatalf("unexpected calls: %+v", c.Calls)
	}
	if len(c.Calls[1].Args) != 1 || c.Calls[1].Args[0] != "comments" {
		t.Errorf("expected from(comments), got args %+v", c.Calls[1].Args)
	}
	// db.select() itself must not also be captured as a standalone chain.
	if len(chains) != 1 {
		t.Errorf("db.select() inner call leaked as its own chain: %+v", chains)
	}
}

func TestMethodChains_SelectFromWhere(t *testing.T) {
	chains := parseMethodChains(t, `db.select().from(comments).where(eq(comments.postId, id))`)
	if len(chains) != 1 {
		t.Fatalf("expected 1 chain, got %d: %+v", len(chains), chains)
	}
	c := chains[0]
	if len(c.Calls) != 3 {
		t.Fatalf("expected 3 segments (select, from, where), got %+v", c.Calls)
	}
	if c.Calls[0].Name != "select" || c.Calls[1].Name != "from" || c.Calls[2].Name != "where" {
		t.Errorf("unexpected segment order: %+v", c.Calls)
	}
}

func TestMethodChains_InsertValuesReturning(t *testing.T) {
	chains := parseMethodChains(t, `db.insert(comments).values({ content: "hi" }).returning()`)
	if len(chains) != 1 {
		t.Fatalf("expected 1 chain, got %d: %+v", len(chains), chains)
	}
	c := chains[0]
	if len(c.Calls) != 3 || c.Calls[0].Name != "insert" || c.Calls[1].Name != "values" || c.Calls[2].Name != "returning" {
		t.Fatalf("unexpected calls: %+v", c.Calls)
	}
	if len(c.Calls[0].Args) != 1 || c.Calls[0].Args[0] != "comments" {
		t.Errorf("expected insert(comments), got args %+v", c.Calls[0].Args)
	}
	if c.Calls[2].Args != nil {
		t.Errorf("expected returning() to have nil args, got %+v", c.Calls[2].Args)
	}
}

func TestMethodChains_UpdateSet(t *testing.T) {
	chains := parseMethodChains(t, `db.update(users).set({ name: "x" }).where(eq(users.id, 1))`)
	if len(chains) != 1 {
		t.Fatalf("expected 1 chain, got %d: %+v", len(chains), chains)
	}
	if chains[0].Calls[0].Name != "update" || chains[0].Calls[0].Args[0] != "users" {
		t.Errorf("unexpected update call: %+v", chains[0].Calls[0])
	}
}

func TestMethodChains_Delete(t *testing.T) {
	chains := parseMethodChains(t, `db.delete(users).where(eq(users.id, 1))`)
	if len(chains) != 1 || chains[0].Calls[0].Name != "delete" || chains[0].Calls[0].Args[0] != "users" {
		t.Fatalf("unexpected chain: %+v", chains)
	}
}

func TestMethodChains_QueryPropertyChain(t *testing.T) {
	chains := parseMethodChains(t, `db.query.users.findMany()`)
	if len(chains) != 1 {
		t.Fatalf("expected 1 chain, got %d: %+v", len(chains), chains)
	}
	c := chains[0]
	if c.Root != "db" {
		t.Errorf("expected root db, got %q", c.Root)
	}
	if len(c.Calls) != 3 || c.Calls[0].Name != "query" || c.Calls[1].Name != "users" || c.Calls[2].Name != "findMany" {
		t.Fatalf("unexpected calls: %+v", c.Calls)
	}
	if c.Calls[0].Args != nil || c.Calls[1].Args != nil {
		t.Errorf("expected property-only segments to have nil args, got %+v", c.Calls)
	}
}

func TestMethodChains_ThreeLevelChainedCallShapeAlsoCaptured(t *testing.T) {
	// prisma.user.findMany() is a property chain ending in a call — MethodChains
	// captures it too (Drizzle's matcher just won't recognize the shape).
	chains := parseMethodChains(t, `prisma.user.findMany()`)
	if len(chains) != 1 {
		t.Fatalf("expected 1 chain, got %d: %+v", len(chains), chains)
	}
	c := chains[0]
	if c.Root != "prisma" || len(c.Calls) != 2 || c.Calls[0].Name != "user" || c.Calls[1].Name != "findMany" {
		t.Fatalf("unexpected chain: %+v", c)
	}
}

func TestMethodChains_NonMemberCallSkipped(t *testing.T) {
	chains := parseMethodChains(t, `eq(comments.postId, id)`)
	if len(chains) != 0 {
		t.Errorf("expected bare identifier call to be skipped, got %+v", chains)
	}
}

func TestMethodChains_LineNumber(t *testing.T) {
	chains := parseMethodChains(t, "const x = 1\ndb.select().from(users)")
	if len(chains) != 1 || chains[0].Line != 2 {
		t.Fatalf("expected line 2, got %+v", chains)
	}
}

func TestMethodChains_MultipleInFile(t *testing.T) {
	src := "db.select().from(users)\ndb.insert(posts).values({})\n"
	chains := parseMethodChains(t, src)
	if len(chains) != 2 {
		t.Fatalf("expected 2 chains, got %d: %+v", len(chains), chains)
	}
}
