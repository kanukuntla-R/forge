package render_test

import (
	"strings"
	"testing"

	"github.com/kanukuntla-r/forge/internal/manifest"
	"github.com/kanukuntla-r/forge/internal/render"
)

// --- helpers ---

// makeManifest builds a minimal manifest with the given variables.
func makeManifest(vars ...manifest.Variable) *manifest.Manifest {
	return &manifest.Manifest{Name: "test", Kind: manifest.KindApp, Variables: vars}
}

func boolVar(name string, def bool) manifest.Variable {
	return manifest.Variable{Name: name, Type: manifest.VarTypeBool, Default: def}
}

func stringVar(name string, def string) manifest.Variable {
	return manifest.Variable{Name: name, Type: manifest.VarTypeString, Default: def}
}

func intVar(name string, def int) manifest.Variable {
	return manifest.Variable{Name: name, Type: manifest.VarTypeInt, Default: def}
}

func choiceVar(name string, choices []string, def string) manifest.Variable {
	return manifest.Variable{Name: name, Type: manifest.VarTypeChoice, Choices: choices, Default: def}
}

func requiredStringVar(name string) manifest.Variable {
	return manifest.Variable{Name: name, Type: manifest.VarTypeString} // no default
}

// --- defaults ---

func TestResolveAppliesDefaults(t *testing.T) {
	m := makeManifest(
		stringVar("description", "A hackathon app"),
		boolVar("with_database", false),
		boolVar("with_ai", true),
	)
	got, err := render.Resolve(m, nil, nil, false)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got["description"] != "A hackathon app" {
		t.Errorf("description = %v, want %q", got["description"], "A hackathon app")
	}
	if got["with_database"] != false {
		t.Errorf("with_database = %v, want false", got["with_database"])
	}
	if got["with_ai"] != true {
		t.Errorf("with_ai = %v, want true", got["with_ai"])
	}
}

// --- resolution order ---

func TestResolveJSONOverridesDefault(t *testing.T) {
	m := makeManifest(boolVar("with_ai", true))
	got, err := render.Resolve(m, map[string]any{"with_ai": false}, nil, false)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got["with_ai"] != false {
		t.Errorf("with_ai = %v, want false", got["with_ai"])
	}
}

func TestResolveFlagOverridesJSONAndDefault(t *testing.T) {
	m := makeManifest(boolVar("with_ai", true))
	got, err := render.Resolve(m,
		map[string]any{"with_ai": false}, // JSON says false
		[]string{"with_ai=true"},         // --var says true — wins
		false,
	)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got["with_ai"] != true {
		t.Errorf("with_ai = %v, want true (flag should beat JSON)", got["with_ai"])
	}
}

// --- --var parsing ---

func TestResolveFlagParsesBool(t *testing.T) {
	m := makeManifest(boolVar("with_ai", false))
	got, err := render.Resolve(m, nil, []string{"with_ai=true"}, false)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got["with_ai"] != true {
		t.Errorf("with_ai = %v, want true", got["with_ai"])
	}
}

func TestResolveFlagParsesInt(t *testing.T) {
	m := makeManifest(intVar("count", 0))
	got, err := render.Resolve(m, nil, []string{"count=5"}, false)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got["count"] != 5 {
		t.Errorf("count = %v (%T), want 5 (int)", got["count"], got["count"])
	}
}

func TestResolveFlagValueWithEqualsSign(t *testing.T) {
	// VALUE portion may contain '=' — only the first one is the separator.
	m := makeManifest(stringVar("description", ""))
	got, err := render.Resolve(m, nil, []string{"description=foo=bar"}, false)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got["description"] != "foo=bar" {
		t.Errorf("description = %v, want %q", got["description"], "foo=bar")
	}
}

func TestResolveFlagMissingEquals(t *testing.T) {
	m := makeManifest(boolVar("with_ai", false))
	_, err := render.Resolve(m, nil, []string{"with_ai"}, false)
	if err == nil {
		t.Fatal("expected error for flag without '=', got nil")
	}
}

// --- choice validation ---

func TestResolveChoiceValid(t *testing.T) {
	m := makeManifest(choiceVar("pm", []string{"pnpm", "npm", "yarn"}, "pnpm"))
	got, err := render.Resolve(m, nil, []string{"pm=yarn"}, false)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got["pm"] != "yarn" {
		t.Errorf("pm = %v, want %q", got["pm"], "yarn")
	}
}

func TestResolveChoiceInvalid(t *testing.T) {
	m := makeManifest(choiceVar("pm", []string{"pnpm", "npm"}, "pnpm"))
	_, err := render.Resolve(m, nil, []string{"pm=pip"}, false)
	if err == nil {
		t.Fatal("expected error for choice not in allowed list, got nil")
	}
	if !strings.Contains(err.Error(), "pip") {
		t.Errorf("error should mention the bad value, got: %v", err)
	}
}

// --- type validation ---

func TestResolveWrongTypeBoolFromJSON(t *testing.T) {
	m := makeManifest(boolVar("with_ai", false))
	_, err := render.Resolve(m, map[string]any{"with_ai": "yes"}, nil, false)
	if err == nil {
		t.Fatal("expected error for string value on bool variable, got nil")
	}
}

func TestResolveWrongTypeBoolFromFlag(t *testing.T) {
	m := makeManifest(boolVar("with_ai", false))
	_, err := render.Resolve(m, nil, []string{"with_ai=maybe"}, false)
	if err == nil {
		t.Fatal("expected error for non-bool flag value on bool variable, got nil")
	}
}

func TestResolveWrongTypeIntFromFlag(t *testing.T) {
	m := makeManifest(intVar("count", 0))
	_, err := render.Resolve(m, nil, []string{"count=three"}, false)
	if err == nil {
		t.Fatal("expected error for non-integer flag value on int variable, got nil")
	}
}

func TestResolveJSONFloat64ToInt(t *testing.T) {
	// JSON numbers arrive as float64; they should be converted losslessly.
	m := makeManifest(intVar("count", 0))
	got, err := render.Resolve(m, map[string]any{"count": float64(3)}, nil, false)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got["count"] != 3 {
		t.Errorf("count = %v (%T), want 3 (int)", got["count"], got["count"])
	}
}

func TestResolveJSONFloat64NonIntegerRejected(t *testing.T) {
	m := makeManifest(intVar("count", 0))
	_, err := render.Resolve(m, map[string]any{"count": float64(3.5)}, nil, false)
	if err == nil {
		t.Fatal("expected error for non-integer float64 on int variable, got nil")
	}
}

// --- unknown variables ---

func TestResolveUnknownJSONVar(t *testing.T) {
	m := makeManifest(boolVar("with_ai", false))
	_, err := render.Resolve(m, map[string]any{"typo_variable": true}, nil, false)
	if err == nil {
		t.Fatal("expected error for unknown JSON variable, got nil")
	}
}

func TestResolveUnknownFlagVar(t *testing.T) {
	m := makeManifest(boolVar("with_ai", false))
	_, err := render.Resolve(m, nil, []string{"typo_variable=true"}, false)
	if err == nil {
		t.Fatal("expected error for unknown --var key, got nil")
	}
}

// --- missing required values ---

func TestResolveMissingRequiredNonInteractive(t *testing.T) {
	m := makeManifest(requiredStringVar("description"))
	_, err := render.Resolve(m, nil, nil, false)
	if err == nil {
		t.Fatal("expected error for missing required variable (non-interactive), got nil")
	}
}

func TestResolveMissingRequiredInteractive(t *testing.T) {
	// interactive=true but prompts not yet implemented → error for now
	m := makeManifest(requiredStringVar("description"))
	_, err := render.Resolve(m, nil, nil, true)
	if err == nil {
		t.Fatal("expected error (interactive prompts not yet implemented), got nil")
	}
}

// --- ToTemplateContext ---

func TestToTemplateContextPascalCase(t *testing.T) {
	values := map[string]any{
		"with_ai":         true,
		"with_database":   false,
		"package_manager": "pnpm",
		"description":     "My app",
		"with_dark_mode":  true,
	}
	got := render.ToTemplateContext(values)

	cases := []struct {
		key  string
		want any
	}{
		{"WithAi", true},
		{"WithDatabase", false},
		{"PackageManager", "pnpm"},
		{"Description", "My app"},
		{"WithDarkMode", true},
	}
	for _, tc := range cases {
		v, ok := got[tc.key]
		if !ok {
			t.Errorf("key %q missing from template context", tc.key)
			continue
		}
		if v != tc.want {
			t.Errorf("%q = %v, want %v", tc.key, v, tc.want)
		}
	}
	// snake_case keys must not appear in the output
	for _, k := range []string{"with_ai", "with_database", "package_manager"} {
		if _, ok := got[k]; ok {
			t.Errorf("snake_case key %q should not appear in template context", k)
		}
	}
}

// --- hackathon-app integration ---

func TestResolveHackathonAppAllDefaults(t *testing.T) {
	m, err := manifest.ParseFile("../manifest/testdata/hackathon-app.yaml")
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}
	got, err := render.Resolve(m, nil, nil, false)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if len(got) != len(m.Variables) {
		t.Errorf("got %d variables, want %d", len(got), len(m.Variables))
	}
	if got["with_ai"] != true {
		t.Errorf("with_ai = %v, want true", got["with_ai"])
	}
	if got["with_database"] != false {
		t.Errorf("with_database = %v, want false", got["with_database"])
	}
	if got["package_manager"] != "pnpm" {
		t.Errorf("package_manager = %v, want %q", got["package_manager"], "pnpm")
	}
}

func TestResolveHackathonAppWithOverrides(t *testing.T) {
	m, err := manifest.ParseFile("../manifest/testdata/hackathon-app.yaml")
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}
	// JSON turns off AI; --var overrides package manager (should beat JSON).
	got, err := render.Resolve(m,
		map[string]any{"with_ai": false},
		[]string{"package_manager=npm"},
		false,
	)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got["with_ai"] != false {
		t.Errorf("with_ai = %v, want false (JSON override)", got["with_ai"])
	}
	if got["package_manager"] != "npm" {
		t.Errorf("package_manager = %v, want %q (flag override)", got["package_manager"], "npm")
	}
	// Unchanged defaults still present.
	if got["with_database"] != false {
		t.Errorf("with_database = %v, want false (default)", got["with_database"])
	}
}
