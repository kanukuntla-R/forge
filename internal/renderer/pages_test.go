package renderer

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

func TestPageOutputName(t *testing.T) {
	tests := []struct {
		urlPath string
		want    string
	}{
		{"/", "home"},
		{"/users", "users"},
		{"/posts/:id", "posts-id"},
		{"/tags", "tags"},
	}
	for _, tt := range tests {
		if got := pageOutputName(tt.urlPath); got != tt.want {
			t.Errorf("pageOutputName(%q) = %q, want %q", tt.urlPath, got, tt.want)
		}
	}
}

func TestDynamicParamsFromPath(t *testing.T) {
	tests := []struct {
		urlPath string
		want    map[string]string
	}{
		{"/", nil},
		{"/users", nil},
		{"/posts/:id", map[string]string{"id": "1"}},
		{"/users/:id/posts/:postId", map[string]string{"id": "1", "postId": "1"}},
	}
	for _, tt := range tests {
		got := dynamicParamsFromPath(tt.urlPath)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("dynamicParamsFromPath(%q) = %#v, want %#v", tt.urlPath, got, tt.want)
		}
	}
}

func TestExtractNextjsPages(t *testing.T) {
	analysis := &analyzer.ProjectAnalysis{
		Frameworks: map[string]any{
			"nextjs": &analyzer.NextjsInfo{
				Pages: []analyzer.NextjsPage{
					{ID: "app/page.tsx", Path: "/", File: "app/page.tsx"},
					{ID: "app/users/page.tsx", Path: "/users", File: "app/users/page.tsx", IsAsync: true},
				},
			},
		},
	}
	got := extractNextjsPages(analysis)
	if len(got) != 2 || !got[1].IsAsync {
		t.Errorf("extractNextjsPages() = %#v, want 2 pages with IsAsync preserved", got)
	}
}

func TestNewPageEntryPropsStaticPage(t *testing.T) {
	props := newPageEntryProps("/users")
	if len(props.Params) != 0 {
		t.Errorf("Params = %v, want empty for a static route", props.Params)
	}
}

func TestNewPageEntryPropsDynamicPage(t *testing.T) {
	props := newPageEntryProps("/posts/:id")
	if props.Params["id"] != "1" {
		t.Errorf("Params[id] = %q, want %q", props.Params["id"], "1")
	}
}

func TestValidateTsconfigAliasesMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := validateTsconfigAliases(filepath.Join(dir, "tsconfig.json"))
	if err == nil {
		t.Fatal("want error for missing tsconfig.json")
	}
}

func TestValidateTsconfigAliasesSupportedPattern(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(path, []byte(`{"compilerOptions":{"paths":{"@/*":["./*"]}}}`), 0o644)
	keys, err := validateTsconfigAliases(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 1 || keys[0] != "@/*" {
		t.Errorf("keys = %v, want [@/*]", keys)
	}
}

func TestValidateTsconfigAliasesUnsupportedPattern(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(path, []byte(`{"compilerOptions":{"paths":{"$app/*":["./src/*"]}}}`), 0o644)
	_, err := validateTsconfigAliases(path)
	if err == nil {
		t.Fatal("want error for unsupported alias pattern")
	}
}

func TestValidateTsconfigAliasesNoPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tsconfig.json")
	os.WriteFile(path, []byte(`{"compilerOptions":{"strict":true}}`), 0o644)
	_, err := validateTsconfigAliases(path)
	if err == nil {
		t.Fatal("want error when no paths are configured")
	}
}

func TestDetectPageAliasesNoORMUsage(t *testing.T) {
	file := analyzer.FileInfo{Path: "app/page.tsx"}
	aliases, err := detectPageAliases(file, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(aliases) != 0 {
		t.Errorf("aliases = %v, want none for a page with no DB queries", aliases)
	}
}

func TestDetectPageAliasesPrismaKnownPattern(t *testing.T) {
	file := analyzer.FileInfo{
		Path:            "app/users/page.tsx",
		Imports:         []analyzer.Import{{Source: "@/lib/prisma", Names: []string{"prisma"}}},
		DatabaseQueries: []analyzer.DatabaseQuery{{Table: "User", Operation: "select", ORM: "prisma"}},
	}
	aliases, err := detectPageAliases(file, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(aliases) != 1 || aliases[0].Kind != "prisma" || aliases[0].Specifier != "@/lib/prisma" {
		t.Errorf("aliases = %+v, want one prisma alias for @/lib/prisma", aliases)
	}
}

func TestDetectPageAliasesPrismaNonStandardPatternFailsClearly(t *testing.T) {
	file := analyzer.FileInfo{
		Path:            "app/users/page.tsx",
		Imports:         []analyzer.Import{{Source: "../../services/db-client", Names: []string{"prisma"}}},
		DatabaseQueries: []analyzer.DatabaseQuery{{Table: "User", Operation: "select", ORM: "prisma"}},
	}
	_, err := detectPageAliases(file, nil)
	if err == nil {
		t.Fatal("want error for a non-standard Prisma import path")
	}
	if !strings.Contains(err.Error(), "../../services/db-client") {
		t.Errorf("error should name the offending import, got: %v", err)
	}
}

func TestDetectPageAliasesDrizzleIncludesTableVars(t *testing.T) {
	file := analyzer.FileInfo{
		Path:            "app/tags/page.tsx",
		Imports:         []analyzer.Import{{Source: "@/db", Names: []string{"db", "tags"}}},
		DatabaseQueries: []analyzer.DatabaseQuery{{Table: "tags", Operation: "select", ORM: "drizzle"}},
	}
	databases := []analyzer.DatabaseSchema{{
		Type: "drizzle",
		Tables: []analyzer.DatabaseTable{
			{Name: "comments", VariableName: "comments"},
			{Name: "tags", VariableName: "tags"},
		},
	}}
	aliases, err := detectPageAliases(file, databases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(aliases) != 1 || aliases[0].Kind != "drizzle" {
		t.Fatalf("aliases = %+v, want one drizzle alias", aliases)
	}
	want := []string{"comments", "tags"}
	if !reflect.DeepEqual(aliases[0].TableVars, want) {
		t.Errorf("TableVars = %v, want %v", aliases[0].TableVars, want)
	}
}

func TestDetectPageAliasesBothORMs(t *testing.T) {
	file := analyzer.FileInfo{
		Path: "app/posts/[id]/page.tsx",
		Imports: []analyzer.Import{
			{Source: "@/lib/prisma", Names: []string{"prisma"}},
			{Source: "@/db", Names: []string{"db", "comments"}},
		},
		DatabaseQueries: []analyzer.DatabaseQuery{
			{Table: "Post", Operation: "select", ORM: "prisma"},
			{Table: "comments", Operation: "select", ORM: "drizzle"},
		},
	}
	aliases, err := detectPageAliases(file, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(aliases) != 2 {
		t.Fatalf("aliases = %+v, want 2 (deterministic prisma, then drizzle)", aliases)
	}
	if aliases[0].Kind != "prisma" || aliases[1].Kind != "drizzle" {
		t.Errorf("aliases order = [%s, %s], want [prisma, drizzle]", aliases[0].Kind, aliases[1].Kind)
	}
}
