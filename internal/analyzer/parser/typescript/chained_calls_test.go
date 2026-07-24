package typescript_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer/parser/typescript"
)

func parseChainedCalls(t *testing.T, src string) []typescript.ChainedCall {
	t.Helper()
	p := typescript.NewParser()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return tree.ChainedCalls()
}

func TestChainedCalls_ThreeLevel(t *testing.T) {
	calls := parseChainedCalls(t, `const users = await prisma.user.findMany()`)
	if len(calls) != 1 {
		t.Fatalf("expected 1 chained call, got %d: %+v", len(calls), calls)
	}
	c := calls[0]
	if c.Object != "prisma" || c.Property != "user" || c.Method != "findMany" {
		t.Errorf("unexpected call: %+v", c)
	}
}

func TestChainedCalls_TwoLevelSkipped(t *testing.T) {
	calls := parseChainedCalls(t, `db.select()`)
	if len(calls) != 0 {
		t.Errorf("expected two-level call to be skipped, got %+v", calls)
	}
}

func TestChainedCalls_ChainedResultCallsSkipped(t *testing.T) {
	calls := parseChainedCalls(t, `db.select().from(comments).where(eq(comments.postId, id))`)
	if len(calls) != 0 {
		t.Errorf("expected calls on a call-expression result to be skipped, got %+v", calls)
	}
}

func TestChainedCalls_LineNumber(t *testing.T) {
	calls := parseChainedCalls(t, "const x = 1\nprisma.post.create({})")
	if len(calls) != 1 || calls[0].Line != 2 {
		t.Fatalf("expected line 2, got %+v", calls)
	}
}

func TestChainedCalls_MultipleInFile(t *testing.T) {
	src := "prisma.user.findMany()\nprisma.post.create({})\n"
	calls := parseChainedCalls(t, src)
	if len(calls) != 2 {
		t.Fatalf("expected 2 chained calls, got %d: %+v", len(calls), calls)
	}
}
