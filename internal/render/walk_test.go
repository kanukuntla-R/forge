package render_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/kanukuntla-r/forge/internal/blueprint"
	"github.com/kanukuntla-r/forge/internal/manifest"
	"github.com/kanukuntla-r/forge/internal/render"
)

func newSyntheticBlueprint(files fstest.MapFS) *blueprint.Blueprint {
	return &blueprint.Blueprint{
		Manifest: &manifest.Manifest{Name: "test", Kind: manifest.KindApp},
		FS:       files,
		Source:   blueprint.SourceEmbedded,
	}
}

func TestWriteBlueprintRendersTemplateFiles(t *testing.T) {
	bp := newSyntheticBlueprint(fstest.MapFS{
		"template/README.md.tmpl": &fstest.MapFile{
			Data: []byte("# {{ .Name }}"),
			Mode: 0644,
		},
	})

	target := filepath.Join(t.TempDir(), "output")
	files, err := render.WriteBlueprint(bp, map[string]any{"Name": "hello"}, target)
	if err != nil {
		t.Fatalf("WriteBlueprint() error: %v", err)
	}

	if want := []string{"README.md"}; !reflect.DeepEqual(files, want) {
		t.Errorf("files = %v, want %v", files, want)
	}

	content, err := os.ReadFile(filepath.Join(target, "README.md"))
	if err != nil {
		t.Fatalf("reading rendered file: %v", err)
	}
	if string(content) != "# hello" {
		t.Errorf("content = %q, want %q", content, "# hello")
	}
}

func TestWriteBlueprintCopiesVerbatimFiles(t *testing.T) {
	data := []byte{0x00, 0xFF, 0x42, 0x7F}
	bp := newSyntheticBlueprint(fstest.MapFS{
		"template/data.bin": &fstest.MapFile{Data: data, Mode: 0644},
	})

	target := filepath.Join(t.TempDir(), "output")
	files, err := render.WriteBlueprint(bp, map[string]any{}, target)
	if err != nil {
		t.Fatalf("WriteBlueprint() error: %v", err)
	}

	if want := []string{"data.bin"}; !reflect.DeepEqual(files, want) {
		t.Errorf("files = %v, want %v", files, want)
	}

	got, err := os.ReadFile(filepath.Join(target, "data.bin"))
	if err != nil {
		t.Fatalf("reading verbatim file: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("verbatim file content changed: got %v, want %v", got, data)
	}
}

func TestWriteBlueprintReturnsSortedFileList(t *testing.T) {
	bp := newSyntheticBlueprint(fstest.MapFS{
		"template/z.txt": &fstest.MapFile{Data: []byte("z"), Mode: 0644},
		"template/a.txt": &fstest.MapFile{Data: []byte("a"), Mode: 0644},
		"template/m.txt": &fstest.MapFile{Data: []byte("m"), Mode: 0644},
	})

	target := filepath.Join(t.TempDir(), "output")
	files, err := render.WriteBlueprint(bp, map[string]any{}, target)
	if err != nil {
		t.Fatalf("WriteBlueprint() error: %v", err)
	}

	want := []string{"a.txt", "m.txt", "z.txt"}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files = %v, want %v (expected sorted)", files, want)
	}
}

func TestWriteBlueprintNoTemplateDirReturnsEmpty(t *testing.T) {
	bp := newSyntheticBlueprint(fstest.MapFS{
		"manifest.yaml": &fstest.MapFile{Data: []byte("name: test"), Mode: 0644},
	})

	target := filepath.Join(t.TempDir(), "output")
	files, err := render.WriteBlueprint(bp, map[string]any{}, target)
	if err != nil {
		t.Fatalf("WriteBlueprint() error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty file list, got %v", files)
	}
}

func TestWriteBlueprintPreservesSubdirectoryStructure(t *testing.T) {
	bp := newSyntheticBlueprint(fstest.MapFS{
		"template/app/page.tsx.tmpl": &fstest.MapFile{
			Data: []byte("export default function Page() { return <h1>{{ .Title }}</h1> }"),
			Mode: 0644,
		},
		"template/public/logo.svg": &fstest.MapFile{
			Data: []byte("<svg/>"),
			Mode: 0644,
		},
	})

	target := filepath.Join(t.TempDir(), "output")
	files, err := render.WriteBlueprint(bp, map[string]any{"Title": "Hello"}, target)
	if err != nil {
		t.Fatalf("WriteBlueprint() error: %v", err)
	}

	want := []string{"app/page.tsx", "public/logo.svg"}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files = %v, want %v", files, want)
	}

	got, err := os.ReadFile(filepath.Join(target, "app", "page.tsx"))
	if err != nil {
		t.Fatalf("reading nested rendered file: %v", err)
	}
	if !bytes.Contains(got, []byte("Hello")) {
		t.Errorf("rendered file should contain 'Hello', got %q", got)
	}

	// Files must be 0644 and directories 0755 regardless of source modes.
	checkMode(t, filepath.Join(target, "app", "page.tsx"), 0o644)
	checkMode(t, filepath.Join(target, "public", "logo.svg"), 0o644)
	checkMode(t, filepath.Join(target, "app"), 0o755)
	checkMode(t, filepath.Join(target, "public"), 0o755)
}

// TestWriteBlueprintIgnoresSourceModes confirms that unusual source modes in the
// embedded FS (e.g. 0000 or 0444 from //go:embed) are never propagated to disk.
func TestWriteBlueprintIgnoresSourceModes(t *testing.T) {
	bp := newSyntheticBlueprint(fstest.MapFS{
		"template/locked.txt":    &fstest.MapFile{Data: []byte("a"), Mode: 0000},
		"template/readonly.txt":  &fstest.MapFile{Data: []byte("b"), Mode: 0444},
		"template/sub/file.txt":  &fstest.MapFile{Data: []byte("c"), Mode: 0000},
	})

	target := filepath.Join(t.TempDir(), "output")
	if _, err := render.WriteBlueprint(bp, map[string]any{}, target); err != nil {
		t.Fatalf("WriteBlueprint() error: %v", err)
	}

	checkMode(t, filepath.Join(target, "locked.txt"), 0o644)
	checkMode(t, filepath.Join(target, "readonly.txt"), 0o644)
	checkMode(t, filepath.Join(target, "sub", "file.txt"), 0o644)
	checkMode(t, filepath.Join(target, "sub"), 0o755)
}

func checkMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s mode = %04o, want %04o", path, got, want)
	}
}

func TestWriteBlueprintSkipsEmptyRenderedTemplates(t *testing.T) {
	bp := newSyntheticBlueprint(fstest.MapFS{
		"template/always.md.tmpl": &fstest.MapFile{
			Data: []byte("always included"),
			Mode: 0644,
		},
		"template/conditional.md.tmpl": &fstest.MapFile{
			Data: []byte("{{ if .Show }}visible{{ end }}"),
			Mode: 0644,
		},
	})

	target := filepath.Join(t.TempDir(), "output")
	files, err := render.WriteBlueprint(bp, map[string]any{"Show": false}, target)
	if err != nil {
		t.Fatalf("WriteBlueprint() error: %v", err)
	}

	if want := []string{"always.md"}; !reflect.DeepEqual(files, want) {
		t.Errorf("files = %v, want %v", files, want)
	}

	if _, err := os.Stat(filepath.Join(target, "conditional.md")); !os.IsNotExist(err) {
		t.Error("conditional.md should not exist when template renders empty")
	}
}

func TestWriteBlueprintTemplatedFilename(t *testing.T) {
	bp := newSyntheticBlueprint(fstest.MapFS{
		"template/components/{{ .ComponentName }}.tsx.tmpl": &fstest.MapFile{
			Data: []byte("export const {{ .ComponentName }} = () => null"),
			Mode: 0644,
		},
	})

	target := filepath.Join(t.TempDir(), "output")
	files, err := render.WriteBlueprint(bp, map[string]any{"ComponentName": "UserCard"}, target)
	if err != nil {
		t.Fatalf("WriteBlueprint() error: %v", err)
	}

	want := []string{"components/UserCard.tsx"}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files = %v, want %v", files, want)
	}

	content, err := os.ReadFile(filepath.Join(target, "components", "UserCard.tsx"))
	if err != nil {
		t.Fatalf("reading rendered file: %v", err)
	}
	if string(content) != "export const UserCard = () => null" {
		t.Errorf("content = %q", content)
	}
}

func TestWriteBlueprintTemplatedDirectory(t *testing.T) {
	bp := newSyntheticBlueprint(fstest.MapFS{
		"template/app/api/{{ .RouteName }}/route.ts.tmpl": &fstest.MapFile{
			Data: []byte("export function GET() {}"),
			Mode: 0644,
		},
	})

	target := filepath.Join(t.TempDir(), "output")
	files, err := render.WriteBlueprint(bp, map[string]any{"RouteName": "users"}, target)
	if err != nil {
		t.Fatalf("WriteBlueprint() error: %v", err)
	}

	want := []string{"app/api/users/route.ts"}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files = %v, want %v", files, want)
	}

	if _, err := os.Stat(filepath.Join(target, "app", "api", "users", "route.ts")); err != nil {
		t.Errorf("expected file at app/api/users/route.ts: %v", err)
	}
}

func TestWriteBlueprintConditionalPathSegmentTrue(t *testing.T) {
	bp := newSyntheticBlueprint(fstest.MapFS{
		"template/{{ if .WithFeature }}feature{{ end }}/data.ts.tmpl": &fstest.MapFile{
			Data: []byte("// data"),
			Mode: 0644,
		},
	})

	target := filepath.Join(t.TempDir(), "output")
	files, err := render.WriteBlueprint(bp, map[string]any{"WithFeature": true}, target)
	if err != nil {
		t.Fatalf("WriteBlueprint() error: %v", err)
	}

	want := []string{"feature/data.ts"}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files = %v, want %v", files, want)
	}
}

func TestWriteBlueprintConditionalPathSegmentFalse(t *testing.T) {
	bp := newSyntheticBlueprint(fstest.MapFS{
		"template/{{ if .WithFeature }}feature{{ end }}/data.ts.tmpl": &fstest.MapFile{
			Data: []byte("// data"),
			Mode: 0644,
		},
	})

	target := filepath.Join(t.TempDir(), "output")
	files, err := render.WriteBlueprint(bp, map[string]any{"WithFeature": false}, target)
	if err != nil {
		t.Fatalf("WriteBlueprint() error: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected no files written, got %v", files)
	}
	if _, err := os.Stat(filepath.Join(target, "feature")); !os.IsNotExist(err) {
		t.Error("feature/ directory should not exist when path segment conditional is false")
	}
}

func TestWriteBlueprintMissingKeyInPath(t *testing.T) {
	bp := newSyntheticBlueprint(fstest.MapFS{
		"template/app/{{ .UndefinedKey }}/file.ts.tmpl": &fstest.MapFile{
			Data: []byte("content"),
			Mode: 0644,
		},
	})

	target := filepath.Join(t.TempDir(), "output")
	_, err := render.WriteBlueprint(bp, map[string]any{}, target)
	if err == nil {
		t.Fatal("expected error for missing key in path segment, got nil")
	}
}

func TestWriteBlueprintMultipleTemplatedSegments(t *testing.T) {
	bp := newSyntheticBlueprint(fstest.MapFS{
		"template/a/{{ .X }}/b/{{ .Y }}/file.ts.tmpl": &fstest.MapFile{
			Data: []byte("ok"),
			Mode: 0644,
		},
	})

	target := filepath.Join(t.TempDir(), "output")
	files, err := render.WriteBlueprint(bp, map[string]any{"X": "alpha", "Y": "beta"}, target)
	if err != nil {
		t.Fatalf("WriteBlueprint() error: %v", err)
	}

	want := []string{"a/alpha/b/beta/file.ts"}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files = %v, want %v", files, want)
	}

	if _, err := os.Stat(filepath.Join(target, "a", "alpha", "b", "beta", "file.ts")); err != nil {
		t.Errorf("expected file at a/alpha/b/beta/file.ts: %v", err)
	}
}

func TestWriteBlueprintHackathonAppRealBlueprint(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}
	bp, err := r.Find("hackathon-app")
	if err != nil {
		t.Fatalf("Find() error: %v", err)
	}

	values, err := render.Resolve(bp.Manifest, nil, nil, false)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	values["name"] = "test-project"
	ctx := render.ToTemplateContext(values)

	target := filepath.Join(t.TempDir(), "output")
	files, err := render.WriteBlueprint(bp, ctx, target)
	if err != nil {
		t.Fatalf("WriteBlueprint() error: %v", err)
	}
	if len(files) == 0 {
		t.Error("expected at least one file written for blueprint with template/")
	}
}
