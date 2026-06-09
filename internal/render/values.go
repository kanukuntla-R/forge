package render

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/kanukuntla-r/forge/internal/manifest"
)

// FormOption configures the huh form before Resolve runs it.
// This exists primarily to allow tests to inject scripted input via
// WithInput/WithAccessible. Production callers should omit it and let
// huh use the real terminal.
type FormOption func(*huh.Form) *huh.Form

// Resolve produces the final variable map for a blueprint scaffolding run.
// Resolution order — last wins:
//  1. Defaults from the manifest
//  2. jsonVars (decoded from --json stdin)
//  3. flagVars (parsed from --var KEY=VALUE flags)
//  4. Type validation and coercion.
//
// When interactive=true (walkthrough mode), ALL variables not explicitly
// supplied via jsonVars or flagVars are prompted — defaults appear pre-filled,
// user hits Enter to accept or types to change.
// When interactive=false, variables with no value at all return an error.
//
// formOpts are applied to the huh form before it runs; production callers
// omit them.
func Resolve(
	m *manifest.Manifest,
	jsonVars map[string]any,
	flagVars []string,
	interactive bool,
	formOpts ...FormOption,
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

	// 5. Track which variables were explicitly provided by the caller so we
	//    know which ones to skip during interactive walkthrough.
	explicitlySet := make(map[string]bool, len(jsonVars)+len(flagVars))
	for k := range jsonVars {
		explicitlySet[k] = true
	}
	for _, kv := range flagVars {
		if idx := strings.IndexByte(kv, '='); idx > 0 {
			explicitlySet[kv[:idx]] = true
		}
	}

	// 6. Collect variables to prompt for (or error on if non-interactive).
	//    Walkthrough mode: prompt for everything not explicitly supplied (defaults pre-filled).
	//    Non-interactive: only error on variables with no value at all.
	var missing []manifest.Variable
	for _, v := range m.Variables {
		if explicitlySet[v.Name] {
			continue
		}
		_, hasValue := result[v.Name]
		if interactive || !hasValue {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		if !interactive {
			return nil, fmt.Errorf("variable %q is required but was not provided (use --var %s=<value> or --json)",
				missing[0].Name, missing[0].Name)
		}
		if err := promptMissing(missing, result, formOpts); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// promptMissing builds a single huh form for all missing variables, runs it,
// and stores the answers in result. All fields are grouped into one form so
// the user sees the whole set of questions at once.
func promptMissing(missing []manifest.Variable, result map[string]any, opts []FormOption) error {
	type capture struct {
		name    string
		vtype   manifest.VarType
		strVal  string
		boolVal bool
	}

	caps := make([]capture, len(missing))
	fields := make([]huh.Field, len(missing))

	for i, v := range missing {
		cap := &caps[i]
		cap.name = v.Name
		cap.vtype = v.Type

		switch v.Type {
		case manifest.VarTypeString, manifest.VarTypePath:
			if def, ok := v.Default.(string); ok {
				cap.strVal = def
			}
			f := huh.NewInput().Title(v.Prompt).Value(&cap.strVal)
			if v.Pattern != "" {
				re := regexp.MustCompile(v.Pattern)
				f = f.Validate(func(s string) error {
					if !re.MatchString(s) {
						return fmt.Errorf("must match pattern %s", v.Pattern)
					}
					return nil
				})
			}
			fields[i] = f

		case manifest.VarTypeBool:
			if def, ok := v.Default.(bool); ok {
				cap.boolVal = def
			}
			fields[i] = huh.NewConfirm().Title(v.Prompt).Value(&cap.boolVal)

		case manifest.VarTypeInt:
			if def, ok := v.Default.(int); ok {
				cap.strVal = strconv.Itoa(def)
			}
			// Capture min/max before the closure to avoid loop variable capture issues.
			min, max := v.Min, v.Max
			fields[i] = huh.NewInput().Title(v.Prompt).Value(&cap.strVal).
				Validate(func(s string) error {
					n, err := strconv.Atoi(s)
					if err != nil {
						return fmt.Errorf("expected integer, got %q", s)
					}
					if min != nil && n < *min {
						return fmt.Errorf("must be at least %d", *min)
					}
					if max != nil && n > *max {
						return fmt.Errorf("must be at most %d", *max)
					}
					return nil
				})

		case manifest.VarTypeChoice:
			if def, ok := v.Default.(string); ok {
				cap.strVal = def
			}
			options := make([]huh.Option[string], len(v.Choices))
			for j, c := range v.Choices {
				options[j] = huh.NewOption(c, c)
			}
			fields[i] = huh.NewSelect[string]().Title(v.Prompt).Options(options...).Value(&cap.strVal)
		}
	}

	form := huh.NewForm(huh.NewGroup(fields...))
	for _, opt := range opts {
		form = opt(form)
	}
	if err := form.Run(); err != nil {
		return fmt.Errorf("prompt: %w", err)
	}

	for _, cap := range caps {
		switch cap.vtype {
		case manifest.VarTypeString, manifest.VarTypePath, manifest.VarTypeChoice:
			result[cap.name] = cap.strVal
		case manifest.VarTypeBool:
			result[cap.name] = cap.boolVal
		case manifest.VarTypeInt:
			n, _ := strconv.Atoi(cap.strVal) // already validated by huh
			result[cap.name] = n
		}
	}
	return nil
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
