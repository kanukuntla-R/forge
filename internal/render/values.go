package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kanukuntla-r/forge/internal/manifest"
)

// Resolve produces the final variable map for a blueprint scaffolding run.
// Resolution order — last wins:
//  1. Defaults from the manifest
//  2. jsonVars (decoded from --json stdin)
//  3. flagVars (parsed from --var KEY=VALUE flags)
//  4. Interactive prompts — not yet implemented; see TODO below.
//
// If interactive is true and a variable still has no value after steps 1–3,
// Resolve returns an error until prompt support is wired in.
func Resolve(
	m *manifest.Manifest,
	jsonVars map[string]any,
	flagVars []string,
	interactive bool,
) (map[string]any, error) {
	byName := make(map[string]manifest.Variable, len(m.Variables))
	for _, v := range m.Variables {
		byName[v.Name] = v
	}

	result := make(map[string]any)

	// 1. Defaults.
	for _, v := range m.Variables {
		if v.Default != nil {
			result[v.Name] = v.Default
		}
	}

	// 2. JSON map.
	for k, val := range jsonVars {
		if _, ok := byName[k]; !ok {
			return nil, fmt.Errorf("unknown variable %q in JSON input", k)
		}
		result[k] = val
	}

	// 3. --var flag strings.
	for _, kv := range flagVars {
		k, val, err := parseFlagVar(kv, byName)
		if err != nil {
			return nil, fmt.Errorf("--var %s: %w", kv, err)
		}
		result[k] = val
	}

	// 4. Type validation and coercion.
	for _, v := range m.Variables {
		val, ok := result[v.Name]
		if !ok {
			continue
		}
		coerced, err := coerce(val, v)
		if err != nil {
			return nil, fmt.Errorf("variable %q: %w", v.Name, err)
		}
		result[v.Name] = coerced
	}

	// 5. Check for missing values.
	for _, v := range m.Variables {
		if _, ok := result[v.Name]; ok {
			continue
		}
		if interactive {
			// TODO: collect missing variables and prompt via charmbracelet/huh.
			// Until huh is wired in (next chunk), treat as an error.
			return nil, fmt.Errorf("variable %q has no value and no default; interactive prompts not yet implemented", v.Name)
		}
		return nil, fmt.Errorf("variable %q is required but was not provided (use --var %s=<value> or --json)", v.Name, v.Name)
	}

	return result, nil
}

// ToTemplateContext returns a copy of values with keys converted from
// snake_case to PascalCase for use in text/template rendering.
// e.g., with_ai → WithAi, package_manager → PackageManager.
func ToTemplateContext(values map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for k, v := range values {
		out[snakeToPascal(k)] = v
	}
	return out
}

// parseFlagVar parses a KEY=VALUE string. The value is converted to the Go
// type expected by the variable's declared type so it passes coerce cleanly.
func parseFlagVar(kv string, byName map[string]manifest.Variable) (string, any, error) {
	idx := strings.IndexByte(kv, '=')
	if idx < 0 {
		return "", nil, fmt.Errorf("missing '='; expected KEY=VALUE format")
	}
	k, raw := kv[:idx], kv[idx+1:]
	v, ok := byName[k]
	if !ok {
		return "", nil, fmt.Errorf("unknown variable %q", k)
	}
	val, err := parseRaw(raw, v.Type)
	if err != nil {
		return "", nil, err
	}
	return k, val, nil
}

// parseRaw converts a raw flag string to the Go type expected for varType.
// string/path/choice come through as-is; bool and int are parsed strictly.
func parseRaw(raw string, t manifest.VarType) (any, error) {
	switch t {
	case manifest.VarTypeBool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("expected bool (true/false), got %q", raw)
		}
		return b, nil
	case manifest.VarTypeInt:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("expected integer, got %q", raw)
		}
		return n, nil
	default:
		return raw, nil
	}
}

// coerce ensures val is the correct Go type for variable v.
// For int variables it also handles JSON's float64 → int conversion.
// For choice variables it validates membership.
func coerce(val any, v manifest.Variable) (any, error) {
	switch v.Type {
	case manifest.VarTypeBool:
		if _, ok := val.(bool); !ok {
			return nil, fmt.Errorf("expected bool, got %T", val)
		}
	case manifest.VarTypeInt:
		switch n := val.(type) {
		case int:
			// already the right type
		case float64:
			// JSON decodes all numbers as float64; convert losslessly.
			if n != float64(int(n)) {
				return nil, fmt.Errorf("expected integer, got float %v", n)
			}
			return int(n), nil
		default:
			return nil, fmt.Errorf("expected int, got %T", val)
		}
	case manifest.VarTypeChoice:
		s, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("expected string for choice, got %T", val)
		}
		for _, c := range v.Choices {
			if c == s {
				return val, nil
			}
		}
		return nil, fmt.Errorf("%q is not an allowed choice; must be one of: %s",
			s, strings.Join(v.Choices, ", "))
	}
	return val, nil
}

// snakeToPascal converts a snake_case identifier to PascalCase.
// e.g., with_ai → WithAi, package_manager → PackageManager.
func snakeToPascal(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}
