package renderer

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

// copyFixtureProject copies internal/renderer/testdata/batchproject into a
// fresh temp dir so RenderBatch's writes (.forge/analysis.json,
// .forge/renders/) never touch the checked-in fixture.
func copyFixtureProject(t *testing.T) string {
	t.Helper()
	return copyFixture(t, "testdata/batchproject")
}

// copyFixture copies the fixture project directory at src into a fresh
// temp dir so RenderBatch's writes never touch the checked-in fixture.
func copyFixture(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copying fixture project %s: %v", src, err)
	}
	return dst
}

func TestOutputName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"components/UserCard.tsx", "components-UserCard"},
		{"components/shared/Card.tsx", "components-shared-Card"},
		{"components/Header.jsx", "components-Header"},
	}
	for _, tt := range tests {
		if got := outputName(tt.path); got != tt.want {
			t.Errorf("outputName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestOutputNameAvoidsCollisions(t *testing.T) {
	a := outputName("components/Card.tsx")
	b := outputName("components/shared/Card.tsx")
	if a == b {
		t.Errorf("outputName produced colliding names for distinct paths: %q", a)
	}
}

func TestExtractNextjsComponentsTypedInProcess(t *testing.T) {
	analysis := &analyzer.ProjectAnalysis{
		Frameworks: map[string]any{
			"nextjs": &analyzer.NextjsInfo{
				Components: []analyzer.NextjsComponent{
					{ID: "components/Header.tsx", Name: "Header", File: "components/Header.tsx"},
				},
			},
		},
	}
	got := extractNextjsComponents(analysis)
	want := []analyzer.NextjsComponent{{ID: "components/Header.tsx", Name: "Header", File: "components/Header.tsx"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("extractNextjsComponents() = %#v, want %#v", got, want)
	}
}

func TestExtractNextjsComponentsAfterJSONRoundTrip(t *testing.T) {
	original := &analyzer.ProjectAnalysis{
		Frameworks: map[string]any{
			"nextjs": &analyzer.NextjsInfo{
				Components: []analyzer.NextjsComponent{
					{ID: "components/UserCard.tsx", Name: "UserCard", File: "components/UserCard.tsx"},
				},
			},
		},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded analyzer.ProjectAnalysis
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := extractNextjsComponents(&decoded)
	want := []analyzer.NextjsComponent{{ID: "components/UserCard.tsx", Name: "UserCard", File: "components/UserCard.tsx", UsedBy: nil}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("extractNextjsComponents() after round-trip = %#v, want %#v", got, want)
	}
}

func TestExtractNextjsComponentsNoFramework(t *testing.T) {
	analysis := &analyzer.ProjectAnalysis{Frameworks: map[string]any{}}
	got := extractNextjsComponents(analysis)
	if len(got) != 0 {
		t.Errorf("extractNextjsComponents() = %#v, want empty", got)
	}
}

func TestEnsureAnalysisUsesExistingFile(t *testing.T) {
	dir := t.TempDir()
	forgeDir := filepath.Join(dir, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fixture := analyzer.ProjectAnalysis{Project: analyzer.ProjectInfo{Name: "fixture-marker"}}
	data, _ := json.Marshal(fixture)
	if err := os.WriteFile(filepath.Join(forgeDir, "analysis.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ensureAnalysis(dir, false, func(string, ...any) {})
	if err != nil {
		t.Fatalf("ensureAnalysis: %v", err)
	}
	if got.Project.Name != "fixture-marker" {
		t.Errorf("ensureAnalysis returned Project.Name = %q, want %q (should have used existing file, not re-analyzed)", got.Project.Name, "fixture-marker")
	}
}

func TestEnsureAnalysisRunsWhenMissing(t *testing.T) {
	dir := t.TempDir()

	got, err := ensureAnalysis(dir, false, func(string, ...any) {})
	if err != nil {
		t.Fatalf("ensureAnalysis: %v", err)
	}
	if got == nil {
		t.Fatal("ensureAnalysis returned nil analysis")
	}
	if _, err := os.Stat(filepath.Join(dir, ".forge", "analysis.json")); err != nil {
		t.Errorf("ensureAnalysis did not write analysis.json: %v", err)
	}
}

func TestEnsureAnalysisRefreshIgnoresExistingFile(t *testing.T) {
	dir := t.TempDir()
	forgeDir := filepath.Join(dir, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fixture := analyzer.ProjectAnalysis{Project: analyzer.ProjectInfo{Name: "fixture-marker"}}
	data, _ := json.Marshal(fixture)
	if err := os.WriteFile(filepath.Join(forgeDir, "analysis.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ensureAnalysis(dir, true, func(string, ...any) {})
	if err != nil {
		t.Fatalf("ensureAnalysis: %v", err)
	}
	if got.Project.Name == "fixture-marker" {
		t.Errorf("ensureAnalysis with refresh=true used the stale existing file instead of re-analyzing")
	}
}

// TestRenderBatchIntegration proves the full batch pipeline end-to-end
// against a real fixture project: auto-analyze, tsconfig alias resolution,
// collision-safe naming for two same-named components, and graceful
// failure handling for a component with an unresolvable import. Requires
// Node.js/Playwright; skipped when node isn't on PATH.
func TestRenderBatchIntegration(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH, skipping renderer batch integration test")
	}

	dir := copyFixtureProject(t)
	var logOut bytes.Buffer
	result, err := RenderBatch(BatchOptions{
		ProjectRoot: dir,
		Parallelism: 4,
		Verbose:     true,
		Stdout:      &logOut,
	})
	if err != nil {
		t.Fatalf("RenderBatch: %v\nlog:\n%s", err, logOut.String())
	}

	if result.Summary.Components.Total != 3 {
		t.Fatalf("Summary.Components.Total = %d, want 3 (log:\n%s)", result.Summary.Components.Total, logOut.String())
	}
	if result.Summary.Components.Succeeded != 2 || result.Summary.Components.Failed != 1 {
		t.Errorf("Summary.Components = %+v, want 2 succeeded, 1 failed", result.Summary.Components)
	}

	byPath := make(map[string]ComponentRender, len(result.Components))
	for _, c := range result.Components {
		byPath[c.Path] = c
	}

	card, ok := byPath["components/Card.tsx"]
	if !ok {
		t.Fatal("missing components/Card.tsx in results")
	}
	if card.RenderStatus != "success" {
		t.Errorf("components/Card.tsx RenderStatus = %q, want success (error: %s)", card.RenderStatus, card.RenderError)
	} else {
		assertPNG(t, filepath.Join(dir, card.RenderPath))
	}
	if len(card.Props.Expected) == 0 || card.Props.Expected[0].Name != "title" {
		t.Errorf("components/Card.tsx Props.Expected = %+v, want a 'title' prop", card.Props.Expected)
	}

	sharedCard, ok := byPath["components/shared/Card.tsx"]
	if !ok {
		t.Fatal("missing components/shared/Card.tsx in results")
	}
	if sharedCard.RenderStatus != "success" {
		t.Errorf("components/shared/Card.tsx RenderStatus = %q, want success (error: %s)", sharedCard.RenderStatus, sharedCard.RenderError)
	}

	if card.RenderPath == sharedCard.RenderPath {
		t.Errorf("both Card components rendered to the same path %q — naming collision not avoided", card.RenderPath)
	}
	if card.RenderPath != ".forge/renders/components-Card.png" {
		t.Errorf("components/Card.tsx RenderPath = %q, want a project-relative path", card.RenderPath)
	}
	if sharedCard.RenderPath != ".forge/renders/components-shared-Card.png" {
		t.Errorf("components/shared/Card.tsx RenderPath = %q, want a project-relative path", sharedCard.RenderPath)
	}

	broken, ok := byPath["components/Broken.tsx"]
	if !ok {
		t.Fatal("missing components/Broken.tsx in results")
	}
	if broken.RenderStatus != "failed" {
		t.Errorf("components/Broken.tsx RenderStatus = %q, want failed", broken.RenderStatus)
	}
	if broken.RenderError == "" {
		t.Error("components/Broken.tsx RenderError is empty, want the bundler's unresolved-import message")
	}
	if broken.RenderPath != "" {
		t.Errorf("components/Broken.tsx RenderPath = %q, want empty for a failed render", broken.RenderPath)
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
	if onDisk.Summary.Components.Total != 3 {
		t.Errorf("metadata.json Summary.Components.Total = %d, want 3", onDisk.Summary.Components.Total)
	}

	if _, err := os.Stat(filepath.Join(dir, ".forge", "analysis.json")); err != nil {
		t.Errorf("RenderBatch did not auto-write analysis.json: %v", err)
	}
}

// TestRenderBatchNoComponents proves a project with no detected components
// (here: analysis.json exists but Frameworks has no "nextjs" key) still
// exits cleanly with an empty, non-error result — not a Node/Playwright
// invocation.
func TestRenderBatchNoComponents(t *testing.T) {
	dir := t.TempDir()
	forgeDir := filepath.Join(dir, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	analysis := analyzer.ProjectAnalysis{Frameworks: map[string]any{}}
	data, _ := json.Marshal(analysis)
	if err := os.WriteFile(filepath.Join(forgeDir, "analysis.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := RenderBatch(BatchOptions{ProjectRoot: dir, Stdout: io.Discard})
	if err != nil {
		t.Fatalf("RenderBatch on component-less project: unexpected error: %v", err)
	}
	if result.Summary.Components.Total != 0 || result.Summary.Pages.Total != 0 {
		t.Errorf("Summary = %+v, want 0 components and 0 pages", result.Summary)
	}
	if _, err := os.Stat(filepath.Join(forgeDir, "renders", "metadata.json")); err != nil {
		t.Errorf("expected metadata.json to still be written: %v", err)
	}
}
