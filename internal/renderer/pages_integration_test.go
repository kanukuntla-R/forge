package renderer

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestRenderBatchPagesIntegration proves the full page-rendering pipeline
// end-to-end against a real fixture project: a static page, an async page
// doing a real Prisma query (aliased to the generic fixture stub), a
// dynamic-route page with a mocked :id param, a page chaining Drizzle's
// .from().where() (the exact shape demo-shop2's real posts/[id] page uses),
// and a page whose Prisma import path isn't on the Level A allow-list
// (must fail clearly, not silently). Requires Node.js/Playwright; skipped
// when node isn't on PATH.
func TestRenderBatchPagesIntegration(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH, skipping renderer pages integration test")
	}

	dir := copyFixture(t, "testdata/pages-project")
	result, err := RenderBatch(BatchOptions{
		ProjectRoot: dir,
		Parallelism: 4,
		PagesOnly:   true,
	})
	if err != nil {
		t.Fatalf("RenderBatch: %v", err)
	}

	if result.Summary.Pages.Total != 5 {
		t.Fatalf("Summary.Pages.Total = %d, want 5", result.Summary.Pages.Total)
	}

	byFile := make(map[string]PageRender, len(result.Pages))
	for _, p := range result.Pages {
		byFile[p.File] = p
	}

	home, ok := byFile["app/page.tsx"]
	if !ok {
		t.Fatal("missing app/page.tsx in results")
	}
	if home.Type != "static" {
		t.Errorf("app/page.tsx Type = %q, want static", home.Type)
	}
	if home.RenderStatus != "success" {
		t.Errorf("app/page.tsx RenderStatus = %q, want success (error: %s)", home.RenderStatus, home.RenderError)
	} else {
		assertPNG(t, filepath.Join(dir, home.RenderPath))
	}

	users, ok := byFile["app/users/page.tsx"]
	if !ok {
		t.Fatal("missing app/users/page.tsx in results")
	}
	if users.Type != "async" {
		t.Errorf("app/users/page.tsx Type = %q, want async", users.Type)
	}
	if users.RenderStatus != "success" {
		t.Errorf("app/users/page.tsx RenderStatus = %q, want success (error: %s)", users.RenderStatus, users.RenderError)
	}
	if len(users.UsedFixtures) != 1 || users.UsedFixtures[0] != "prisma" {
		t.Errorf("app/users/page.tsx UsedFixtures = %v, want [prisma]", users.UsedFixtures)
	}

	postDetail, ok := byFile["app/posts/[id]/page.tsx"]
	if !ok {
		t.Fatal("missing app/posts/[id]/page.tsx in results")
	}
	if postDetail.RenderStatus != "success" {
		t.Errorf("app/posts/[id]/page.tsx RenderStatus = %q, want success (error: %s)", postDetail.RenderStatus, postDetail.RenderError)
	}
	if postDetail.DynamicParams["id"] != "1" {
		t.Errorf("app/posts/[id]/page.tsx DynamicParams = %v, want id=1", postDetail.DynamicParams)
	}

	comments, ok := byFile["app/comments/page.tsx"]
	if !ok {
		t.Fatal("missing app/comments/page.tsx in results")
	}
	if comments.RenderStatus != "success" {
		t.Errorf("app/comments/page.tsx RenderStatus = %q, want success (error: %s)", comments.RenderStatus, comments.RenderError)
	} else {
		assertPNG(t, filepath.Join(dir, comments.RenderPath))
	}
	if len(comments.UsedFixtures) != 1 || comments.UsedFixtures[0] != "drizzle" {
		t.Errorf("app/comments/page.tsx UsedFixtures = %v, want [drizzle]", comments.UsedFixtures)
	}

	broken, ok := byFile["app/broken/page.tsx"]
	if !ok {
		t.Fatal("missing app/broken/page.tsx in results")
	}
	if broken.RenderStatus != "failed" {
		t.Errorf("app/broken/page.tsx RenderStatus = %q, want failed (non-standard Prisma import)", broken.RenderStatus)
	}
	if broken.RenderError == "" {
		t.Error("app/broken/page.tsx RenderError is empty, want the Level-A fail-clear message")
	}

	metadataPath := filepath.Join(dir, ".forge", "renders", "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("reading metadata.json: %v", err)
	}
	var onDisk BatchResult
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatalf("parsing metadata.json: %v", err)
	}
	if len(onDisk.Pages) != 5 {
		t.Errorf("metadata.json has %d pages, want 5", len(onDisk.Pages))
	}
}

// TestRenderBatchPagesMissingTsconfigFailsAllClearly proves a missing
// tsconfig.json fails every page up front with one clear message, instead
// of N separate cryptic unresolved-import errors from esbuild.
func TestRenderBatchPagesMissingTsconfigFailsAllClearly(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH, skipping renderer pages integration test")
	}

	dir := copyFixture(t, "testdata/pages-project")
	tsconfigPath := filepath.Join(dir, "tsconfig.json")
	if err := os.Remove(tsconfigPath); err != nil {
		t.Fatal(err)
	}

	result, err := RenderBatch(BatchOptions{ProjectRoot: dir, PagesOnly: true})
	if err != nil {
		t.Fatalf("RenderBatch: %v", err)
	}
	if result.Summary.Pages.Failed != result.Summary.Pages.Total || result.Summary.Pages.Total == 0 {
		t.Fatalf("Summary.Pages = %+v, want every page failed", result.Summary.Pages)
	}
	for _, p := range result.Pages {
		if p.RenderError == "" {
			t.Errorf("page %s has no RenderError", p.File)
		}
	}
}
