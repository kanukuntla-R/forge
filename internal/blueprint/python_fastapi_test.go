package blueprint_test

import (
	"path/filepath"
	"testing"

	"github.com/kanukuntla-r/forge/internal/blueprint"
	"github.com/kanukuntla-r/forge/internal/manifest"
	"github.com/kanukuntla-r/forge/internal/render"
)

func TestPythonFastapiManifest(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("python-fastapi")
	if err != nil {
		t.Fatalf("Find(python-fastapi): %v", err)
	}
	if bp.Manifest.Name != "python-fastapi" {
		t.Errorf("Name = %q, want python-fastapi", bp.Manifest.Name)
	}
	if bp.Manifest.Kind != manifest.KindApp {
		t.Errorf("Kind = %q, want app", bp.Manifest.Kind)
	}
	varNames := map[string]bool{}
	for _, v := range bp.Manifest.Variables {
		varNames[v.Name] = true
	}
	for _, want := range []string{"description", "with_database", "with_auth", "with_docker", "with_openai", "with_type_check"} {
		if !varNames[want] {
			t.Errorf("manifest missing variable %q", want)
		}
	}
}

func TestPythonFastapiScaffoldsBase(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("python-fastapi")
	if err != nil {
		t.Fatalf("Find(python-fastapi): %v", err)
	}

	dir := filepath.Join(t.TempDir(), "output")
	ctx := map[string]any{
		"Name":          "my-api",
		"Description":   "A test API",
		"WithDatabase":  false,
		"WithAuth":      false,
		"WithDocker":    false,
		"WithOpenai":    false,
		"WithTypeCheck": false,
	}
	written, err := render.WriteBlueprint(bp, ctx, dir)
	if err != nil {
		t.Fatalf("WriteBlueprint: %v", err)
	}

	got := map[string]bool{}
	for _, f := range written {
		got[f] = true
	}

	for _, w := range []string{
		"pyproject.toml", "README.md", ".python-version", ".gitignore",
		"src/__init__.py", "src/main.py", "src/config.py",
		"src/routes/__init__.py", "src/routes/health.py",
		"tests/__init__.py", "tests/test_health.py",
	} {
		if !got[w] {
			t.Errorf("base file %q not written; written: %v", w, written)
		}
	}

	// Conditional files absent when all features off
	for _, absent := range []string{
		"src/database.py", "src/models/user.py", "src/schemas/user.py",
		"src/routes/users.py", "src/routes/auth.py", "src/routes/chat.py",
		"src/auth/jwt.py", "src/auth/password.py",
		"Dockerfile", "docker-compose.yml", ".dockerignore", "mypy.ini",
		"alembic.ini", "migrations/env.py", "migrations/script.py.mako",
		"tests/conftest.py", "tests/test_users.py", "tests/test_auth.py",
	} {
		if got[absent] {
			t.Errorf("conditional file %q present when all features off", absent)
		}
	}
}

func TestPythonFastapiScaffoldsWithDatabase(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("python-fastapi")
	if err != nil {
		t.Fatalf("Find(python-fastapi): %v", err)
	}

	dir := filepath.Join(t.TempDir(), "output")
	ctx := map[string]any{
		"Name":          "my-api",
		"Description":   "A test API",
		"WithDatabase":  true,
		"WithAuth":      false,
		"WithDocker":    false,
		"WithOpenai":    false,
		"WithTypeCheck": false,
	}
	written, err := render.WriteBlueprint(bp, ctx, dir)
	if err != nil {
		t.Fatalf("WriteBlueprint: %v", err)
	}

	got := map[string]bool{}
	for _, f := range written {
		got[f] = true
	}

	for _, w := range []string{
		"src/database.py", "src/models/__init__.py", "src/models/user.py",
		"src/schemas/__init__.py", "src/schemas/user.py",
		"src/routes/users.py", "tests/conftest.py", "tests/test_users.py",
		"alembic.ini", "migrations/env.py", "migrations/script.py.mako",
	} {
		if !got[w] {
			t.Errorf("database file %q not written; written: %v", w, written)
		}
	}
}

func TestPythonFastapiScaffoldsWithAllFeatures(t *testing.T) {
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("python-fastapi")
	if err != nil {
		t.Fatalf("Find(python-fastapi): %v", err)
	}

	dir := filepath.Join(t.TempDir(), "output")
	ctx := map[string]any{
		"Name":          "my-api",
		"Description":   "A test API",
		"WithDatabase":  true,
		"WithAuth":      true,
		"WithDocker":    true,
		"WithOpenai":    true,
		"WithTypeCheck": true,
	}
	written, err := render.WriteBlueprint(bp, ctx, dir)
	if err != nil {
		t.Fatalf("WriteBlueprint: %v", err)
	}

	got := map[string]bool{}
	for _, f := range written {
		got[f] = true
	}

	for _, w := range []string{
		"src/database.py", "src/models/user.py", "src/schemas/user.py",
		"src/routes/users.py", "src/routes/auth.py", "src/routes/chat.py",
		"src/auth/jwt.py", "src/auth/password.py",
		"Dockerfile", "docker-compose.yml", ".dockerignore",
		"mypy.ini", "alembic.ini", "migrations/env.py", "migrations/script.py.mako",
		"tests/conftest.py", "tests/test_users.py", "tests/test_auth.py",
	} {
		if !got[w] {
			t.Errorf("file %q not written; written: %v", w, written)
		}
	}
}

func TestPythonFastapiAuthOnlyImpliesDatabase(t *testing.T) {
	// Documents: with_auth=true without with_database=true still creates database
	// infrastructure (database.py, models, schemas, alembic) because auth needs user records.
	// The /users CRUD endpoint and tests are NOT created — only auth infrastructure.
	r, err := blueprint.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	bp, err := r.Find("python-fastapi")
	if err != nil {
		t.Fatalf("Find(python-fastapi): %v", err)
	}

	dir := filepath.Join(t.TempDir(), "output")
	ctx := map[string]any{
		"Name":          "my-api",
		"Description":   "A test API",
		"WithDatabase":  false,
		"WithAuth":      true,
		"WithDocker":    false,
		"WithOpenai":    false,
		"WithTypeCheck": false,
	}
	written, err := render.WriteBlueprint(bp, ctx, dir)
	if err != nil {
		t.Fatalf("WriteBlueprint: %v", err)
	}

	got := map[string]bool{}
	for _, f := range written {
		got[f] = true
	}

	// Auth-only still creates database infrastructure
	for _, present := range []string{
		"src/database.py", "src/models/user.py", "src/schemas/user.py",
		"src/routes/auth.py", "src/auth/jwt.py", "src/auth/password.py",
		"tests/conftest.py", "tests/test_auth.py",
		"alembic.ini", "migrations/env.py",
	} {
		if !got[present] {
			t.Errorf("expected %q present when only auth enabled; written: %v", present, written)
		}
	}

	// /users CRUD is NOT created without explicit with_database
	for _, absent := range []string{"src/routes/users.py", "tests/test_users.py"} {
		if got[absent] {
			t.Errorf("%q should not be created when only auth (not database) is enabled", absent)
		}
	}
}
