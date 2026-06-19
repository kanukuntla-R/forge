package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// AliasMapping maps a wildcard pattern (e.g. "@/*") to its replacement (e.g. "./*").
// Both pattern and replacement must contain exactly one "*".
type AliasMapping struct {
	Pattern     string // e.g. "@/*"
	Replacement string // e.g. "./*"
}

// fallbackAliases are used when tsconfig.json is absent or unparseable.
// "@/" is the de facto Next.js convention; "~/" is used in some Vue/Nuxt setups.
var fallbackAliases = []AliasMapping{
	{Pattern: "@/*", Replacement: "./*"},
	{Pattern: "~/*", Replacement: "./*"},
}

// Resolver resolves local import sources to their actual file paths using
// the project's file index and tsconfig.json path aliases.
type Resolver struct {
	projectRoot  string
	fileSet      map[string]bool // relative paths of all files in the analysis
	aliases      []AliasMapping
	baseUrl      string   // tsconfig baseUrl, default "."
	tsExtensions []string // extensions to try when resolving
}

// NewResolver builds a Resolver from the project root and the completed walk analysis.
// It reads tsconfig.json for path aliases; falls back to hardcoded aliases on any error.
// The returned error is always nil; it exists for future extensibility.
func NewResolver(projectRoot string, analysis *ProjectAnalysis) (*Resolver, error) {
	fileSet := make(map[string]bool, len(analysis.Files))
	for _, f := range analysis.Files {
		fileSet[filepath.ToSlash(f.Path)] = true
	}
	aliases, baseUrl := loadTsconfig(projectRoot)
	return &Resolver{
		projectRoot:  projectRoot,
		fileSet:      fileSet,
		aliases:      aliases,
		baseUrl:      baseUrl,
		tsExtensions: []string{".ts", ".tsx", ".js", ".jsx"},
	}, nil
}

// Resolve updates Import.Resolved for every local import in the analysis.
// Imports marked External are skipped; their Resolved stays empty.
func (r *Resolver) Resolve(analysis *ProjectAnalysis) {
	for i := range analysis.Files {
		file := &analysis.Files[i]
		for j := range file.Imports {
			imp := &file.Imports[j]
			if imp.External || imp.Source == "" {
				continue
			}
			imp.Resolved = r.resolveImport(file.Path, imp.Source)
		}
	}
}

// resolveImport attempts to resolve a single import source relative to the file
// that contains it. Returns the relative path of the resolved file, or "" if
// no matching file was found in the project.
func (r *Resolver) resolveImport(fileRelPath, source string) string {
	transformed, wasAliased := r.applyAlias(source)

	var base string
	switch {
	case wasAliased:
		// Alias replacements are relative to tsconfig's baseUrl (usually project root).
		base = filepath.Join(r.projectRoot, r.baseUrl)
	case strings.HasPrefix(transformed, "./") || strings.HasPrefix(transformed, "../"):
		// Regular relative imports are relative to the file's directory.
		base = filepath.Join(r.projectRoot, filepath.Dir(fileRelPath))
	default:
		// Non-relative, non-aliased: should have been marked External.
		return ""
	}

	abs := filepath.Join(base, transformed)
	rel, err := filepath.Rel(r.projectRoot, abs)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)

	// Try: exact path, path + extension, path/index + extension.
	candidates := make([]string, 0, 1+len(r.tsExtensions)*2)
	candidates = append(candidates, rel)
	for _, ext := range r.tsExtensions {
		candidates = append(candidates, rel+ext)
	}
	for _, ext := range r.tsExtensions {
		candidates = append(candidates, rel+"/index"+ext)
	}

	for _, c := range candidates {
		if r.fileSet[c] {
			return c
		}
	}
	return ""
}

// applyAlias checks source against each alias pattern and returns the transformed
// source if matched. Only handles patterns with a single trailing wildcard (*).
func (r *Resolver) applyAlias(source string) (transformed string, matched bool) {
	for _, alias := range r.aliases {
		idx := strings.Index(alias.Pattern, "*")
		if idx < 0 {
			continue // non-wildcard alias, skip
		}
		prefix := alias.Pattern[:idx]
		if !strings.HasPrefix(source, prefix) || len(source) <= len(prefix) {
			continue
		}
		captured := source[len(prefix):]
		result := strings.Replace(alias.Replacement, "*", captured, 1)
		return result, true
	}
	return source, false
}

// tsconfigJSON is the minimal tsconfig.json structure we care about.
type tsconfigJSON struct {
	CompilerOptions struct {
		BaseUrl string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

// loadTsconfig reads tsconfig.json from projectRoot and extracts path aliases and baseUrl.
// On any error (file missing, JSON comments, invalid JSON) it returns fallbackAliases
// and baseUrl "." so resolution can still proceed.
func loadTsconfig(projectRoot string) (aliases []AliasMapping, baseUrl string) {
	baseUrl = "."

	data, err := os.ReadFile(filepath.Join(projectRoot, "tsconfig.json"))
	if err != nil {
		return fallbackAliases, baseUrl
	}

	var cfg tsconfigJSON
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Common cause: comments or trailing commas (JSON5 features not in encoding/json).
		return fallbackAliases, baseUrl
	}

	if cfg.CompilerOptions.BaseUrl != "" {
		baseUrl = cfg.CompilerOptions.BaseUrl
	}

	for pattern, replacements := range cfg.CompilerOptions.Paths {
		if !strings.Contains(pattern, "*") || len(replacements) == 0 {
			continue // skip non-wildcard or empty mappings
		}
		aliases = append(aliases, AliasMapping{
			Pattern:     pattern,
			Replacement: replacements[0], // use first alternative only
		})
	}

	if len(aliases) == 0 {
		return fallbackAliases, baseUrl
	}
	return aliases, baseUrl
}
