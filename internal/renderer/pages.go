package renderer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

// pageOutputName flattens a page's URL path into a filesystem-safe base
// name (no extension): "/" -> "home", "/posts/:id" -> "posts-id".
func pageOutputName(urlPath string) string {
	trimmed := strings.Trim(urlPath, "/")
	if trimmed == "" {
		return "home"
	}
	trimmed = strings.ReplaceAll(trimmed, "/", "-")
	trimmed = strings.ReplaceAll(trimmed, ":", "")
	return trimmed
}

var dynamicParamRe = regexp.MustCompile(`:(\w+)`)

// dynamicParamsFromPath extracts placeholder values for a dynamic route's
// :param segments (as already produced by appDirURL's [id] -> :id
// transform), e.g. "/posts/:id" -> {"id": "1"}. Returns nil for a static
// route.
func dynamicParamsFromPath(urlPath string) map[string]string {
	matches := dynamicParamRe.FindAllStringSubmatch(urlPath, -1)
	if len(matches) == 0 {
		return nil
	}
	params := make(map[string]string, len(matches))
	for _, m := range matches {
		params[m[1]] = "1"
	}
	return params
}

// supportedAliasPrefixes are the tsconfig path-alias patterns Level A
// resolves. Any other pattern fails clearly rather than being silently
// mishandled.
var supportedAliasPrefixes = map[string]bool{"@/*": true, "~/*": true, "#*": true}

type tsconfigCompilerOptions struct {
	Paths map[string][]string `json:"paths"`
}

type tsconfigFile struct {
	CompilerOptions tsconfigCompilerOptions `json:"compilerOptions"`
}

// validateTsconfigAliases reads tsconfig.json at path and confirms it
// defines at least one Level-A-supported alias pattern (@/*, ~/*, or #*).
// Returns the configured path keys (for error messages) and a nil error on
// success.
func validateTsconfigAliases(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Cannot resolve project aliases: tsconfig.json missing at project root")
	}
	var cfg tsconfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing tsconfig.json: %w", err)
	}
	if len(cfg.CompilerOptions.Paths) == 0 {
		return nil, fmt.Errorf("Cannot resolve project aliases: tsconfig.json has no paths configured")
	}
	keys := make([]string, 0, len(cfg.CompilerOptions.Paths))
	supported := false
	for key := range cfg.CompilerOptions.Paths {
		keys = append(keys, key)
		if supportedAliasPrefixes[key] {
			supported = true
		}
	}
	if !supported {
		return keys, fmt.Errorf("Path alias detection: Level A supports only @/, ~/, or # patterns, found: %s. Manual configuration required.", strings.Join(keys, ", "))
	}
	return keys, nil
}

// pageAlias is one data-access import a page needs aliased to a fixture
// stub, detected via M11's existing DatabaseQuery + Import data — no new
// per-project heuristics.
type pageAlias struct {
	Specifier string   `json:"specifier"`           // the import specifier the page actually wrote, e.g. "@/lib/prisma"
	Kind      string   `json:"kind"`                // "prisma" | "drizzle"
	TableVars []string `json:"tableVars,omitempty"` // drizzle only: schema export names the stub must also provide
}

var prismaAliasSpecifiers = []string{"@/lib/prisma", "@lib/prisma", "@/lib/db"}
var drizzleAliasSpecifiers = []string{"@/db", "@db", "@/lib/drizzle"}

// detectPageAliases decides which data-access imports (if any) a page
// needs stubbed. It trusts M11's existing DatabaseQueries to know whether
// the page performs ORM calls at all, then checks the file's own Imports
// against Level A's fixed allow-list to find the specifier to alias. A
// page that DatabaseQueries says uses an ORM, but whose imports don't
// match the allow-list, fails clearly rather than guessing.
func detectPageAliases(file analyzer.FileInfo, databases []analyzer.DatabaseSchema) ([]pageAlias, error) {
	used := map[string]bool{}
	for _, q := range file.DatabaseQueries {
		used[q.ORM] = true
	}

	var aliases []pageAlias
	for _, orm := range []string{"prisma", "drizzle"} { // fixed order: deterministic output
		if !used[orm] {
			continue
		}
		candidates := prismaAliasSpecifiers
		if orm == "drizzle" {
			candidates = drizzleAliasSpecifiers
		}

		specifier, found := matchImportSpecifier(file.Imports, candidates)
		if !found {
			return nil, fmt.Errorf("%s detection: expected import from %s, found: %s. Update your import or use Level A patterns.",
				strings.ToUpper(orm[:1])+orm[1:], strings.Join(candidates, " or "), strings.Join(importSources(file.Imports), ", "))
		}

		alias := pageAlias{Specifier: specifier, Kind: orm}
		if orm == "drizzle" {
			alias.TableVars = drizzleTableVars(databases)
		}
		aliases = append(aliases, alias)
	}
	return aliases, nil
}

func matchImportSpecifier(imports []analyzer.Import, candidates []string) (string, bool) {
	for _, imp := range imports {
		for _, c := range candidates {
			if imp.Source == c {
				return imp.Source, true
			}
		}
	}
	return "", false
}

func importSources(imports []analyzer.Import) []string {
	sources := make([]string, len(imports))
	for i, imp := range imports {
		sources[i] = imp.Source
	}
	return sources
}

// extractNextjsPages reads analysis.Frameworks["nextjs"].Pages using the
// same JSON-roundtrip normalization as extractNextjsComponents.
func extractNextjsPages(analysis *analyzer.ProjectAnalysis) []analyzer.NextjsPage {
	raw, ok := analysis.Frameworks["nextjs"]
	if !ok {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var info analyzer.NextjsInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil
	}
	return info.Pages
}

// pageEntryProps is the JSON payload written for bundle-page.js's generated
// entry file to pass into the page function, mirroring the Next.js App
// Router page-prop contract: Page({ params, searchParams }).
type pageEntryProps struct {
	Params       map[string]string `json:"params"`
	SearchParams map[string]string `json:"searchParams"`
}

func newPageEntryProps(urlPath string) pageEntryProps {
	params := dynamicParamsFromPath(urlPath)
	if params == nil {
		params = map[string]string{}
	}
	return pageEntryProps{Params: params, SearchParams: map[string]string{}}
}

// renderPages bundles and screenshots every page (capped at pagesLimit if
// positive), reusing the same worker-pool + shared-browser pattern as
// component rendering (invokeBatchRenderer, batchManifestItem — both
// defined in batch.go). A missing or unsupported tsconfig alias
// configuration fails every page up front with one clear message, rather
// than letting each page's esbuild bundle fail separately with a cryptic
// unresolved-import error.
func renderPages(cacheDir, workDir, outputDir, absRoot string, pages []analyzer.NextjsPage, analysis *analyzer.ProjectAnalysis, parallelism, viewportW, viewportH, pagesLimit int) ([]PageRender, error) {
	if pagesLimit > 0 && len(pages) > pagesLimit {
		pages = pages[:pagesLimit]
	}
	if len(pages) == 0 {
		return []PageRender{}, nil
	}

	results := make([]PageRender, len(pages))
	outNameToIndex := make(map[string]int, len(pages))
	for i, p := range pages {
		pageType := "static"
		if p.IsAsync {
			pageType = "async"
		}
		results[i] = PageRender{
			Path:          p.Path,
			File:          p.File,
			Type:          pageType,
			DynamicParams: dynamicParamsFromPath(p.Path),
		}
		outNameToIndex[pageOutputName(p.Path)] = i
	}

	tsconfigPath := filepath.Join(absRoot, "tsconfig.json")
	if _, err := validateTsconfigAliases(tsconfigPath); err != nil {
		for i := range results {
			results[i].RenderStatus = "failed"
			results[i].RenderError = err.Error()
		}
		return results, nil
	}

	filesByPath := make(map[string]analyzer.FileInfo, len(analysis.Files))
	for _, f := range analysis.Files {
		filesByPath[f.Path] = f
	}

	var manifest []batchManifestItem
	var manifestMu sync.Mutex
	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup

	for i, p := range pages {
		i, p := i, p
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			start := time.Now()
			outName := pageOutputName(p.Path)
			absPage := filepath.Join(absRoot, p.File)
			fail := func(err error) {
				results[i].RenderStatus = "failed"
				results[i].RenderError = err.Error()
				results[i].RenderTimeMS = time.Since(start).Milliseconds()
			}

			aliases, err := detectPageAliases(filesByPath[p.File], analysis.Databases)
			if err != nil {
				fail(err)
				return
			}
			for _, a := range aliases {
				results[i].UsedFixtures = append(results[i].UsedFixtures, a.Kind)
			}

			propsPath := filepath.Join(workDir, outName+".page-props.json")
			propsJSON, err := json.Marshal(newPageEntryProps(p.Path))
			if err != nil {
				fail(fmt.Errorf("marshaling page props: %w", err))
				return
			}
			if err := os.WriteFile(propsPath, propsJSON, 0o644); err != nil {
				fail(fmt.Errorf("writing page props: %w", err))
				return
			}

			bundlePath := filepath.Join(workDir, outName+".page-bundle.js")
			bundleArgs := []string{absPage, propsPath, bundlePath, tsconfigPath}
			if len(aliases) > 0 {
				aliasesPath := filepath.Join(workDir, outName+".aliases.json")
				aliasesJSON, err := json.Marshal(aliases)
				if err != nil {
					fail(fmt.Errorf("marshaling data-access aliases: %w", err))
					return
				}
				if err := os.WriteFile(aliasesPath, aliasesJSON, 0o644); err != nil {
					fail(fmt.Errorf("writing data-access aliases: %w", err))
					return
				}
				bundleArgs = append(bundleArgs, aliasesPath)
			}

			if err := runNodeScript(cacheDir, "bundle-page.js", bundleArgs...); err != nil {
				fail(err)
				return
			}

			results[i].RenderTimeMS = time.Since(start).Milliseconds()
			manifestMu.Lock()
			manifest = append(manifest, batchManifestItem{
				OutName:    outName,
				BundlePath: bundlePath,
				OutputPath: filepath.Join(outputDir, outName+".png"),
			})
			manifestMu.Unlock()
		}()
	}
	wg.Wait()

	if len(manifest) > 0 {
		renderResults, err := invokeBatchRenderer(cacheDir, workDir, manifest, parallelism, viewportW, viewportH)
		if err != nil {
			return nil, fmt.Errorf("batch rendering pages: %w", err)
		}
		for _, rr := range renderResults {
			i, ok := outNameToIndex[rr.Name]
			if !ok {
				continue
			}
			results[i].RenderTimeMS += rr.TimeMS
			if rr.Status == "success" {
				results[i].RenderStatus = "success"
				absPNG := filepath.Join(outputDir, rr.Name+".png")
				if rel, err := filepath.Rel(absRoot, absPNG); err == nil {
					results[i].RenderPath = filepath.ToSlash(rel)
				} else {
					results[i].RenderPath = absPNG
				}
			} else {
				results[i].RenderStatus = "failed"
				results[i].RenderError = rr.Error
			}
		}
	}

	return results, nil
}

// drizzleTableVars returns the schema export variable names for the
// first-detected Drizzle schema (Level A: one database per project), so
// the generated stub module can re-export them alongside the client — a
// page importing "@/db" typically also imports its table variables from
// the same module (`import { db, comments } from "@/db"`).
func drizzleTableVars(databases []analyzer.DatabaseSchema) []string {
	for _, schema := range databases {
		if schema.Type != "drizzle" {
			continue
		}
		vars := make([]string, len(schema.Tables))
		for i, t := range schema.Tables {
			vars[i] = t.VariableName
		}
		return vars
	}
	return nil
}
