package renderer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

const batchMetadataVersion = "0.6.0"

// BatchOptions configures a RenderBatch call.
type BatchOptions struct {
	ProjectRoot    string    // project to render; required
	OutputDir      string    // default <ProjectRoot>/.forge/renders
	Parallelism    int       // default 4
	Refresh        bool      // force re-analysis even if analysis.json exists
	Viewport       string    // "WxH"; default "800x600"
	ComponentsOnly bool      // skip pages
	PagesOnly      bool      // skip components
	PagesLimit     int       // 0 = no limit; render only the first N pages
	Verbose        bool      // log each pipeline stage to Stdout
	Stdout         io.Writer // verbose stage log destination; default io.Discard
}

// RenderProps mirrors M13.1's TypeExtraction/GenerateProps shapes: the
// props the analyzer found on the component, and the synthetic values
// generated for them.
type RenderProps struct {
	Expected    []PropType     `json:"expected"`
	Synthesized map[string]any `json:"synthesized,omitempty"`
}

// ComponentRender is one component's entry in a batch render's metadata.
type ComponentRender struct {
	Name         string      `json:"name"`
	Path         string      `json:"path"`
	RenderPath   string      `json:"renderPath,omitempty"`
	RenderStatus string      `json:"renderStatus"` // "success" | "failed"
	RenderTimeMS int64       `json:"renderTime"`
	RenderError  string      `json:"renderError,omitempty"`
	Props        RenderProps `json:"props"`
}

// BatchCounts aggregates one render phase's (components, or pages) results.
type BatchCounts struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// BatchSummary aggregates a batch render's results.
type BatchSummary struct {
	Components  BatchCounts `json:"components"`
	Pages       BatchCounts `json:"pages"`
	TotalTimeMS int64       `json:"totalTime"`
}

// BatchResult is the full output of RenderBatch, also written to metadata.json.
type BatchResult struct {
	Version     string            `json:"version"`
	RenderedAt  string            `json:"renderedAt"`
	ProjectRoot string            `json:"projectRoot"`
	Components  []ComponentRender `json:"components"`
	Pages       []PageRender      `json:"pages"`
	Summary     BatchSummary      `json:"summary"`
}

// PageRender is one page's entry in a batch render's metadata.
type PageRender struct {
	Path          string            `json:"path"` // URL path with :param segments, e.g. "/posts/:id"
	File          string            `json:"file"`
	RenderPath    string            `json:"renderPath,omitempty"`
	RenderStatus  string            `json:"renderStatus"` // "success" | "failed"
	RenderTimeMS  int64             `json:"renderTime"`
	RenderError   string            `json:"renderError,omitempty"`
	Type          string            `json:"type"` // "static" | "async"
	UsedFixtures  []string          `json:"usedFixtures,omitempty"`
	DynamicParams map[string]string `json:"dynamicParams,omitempty"`
}

// RenderBatch renders every analyzer-detected Next.js component in a
// project, using one shared Playwright browser across Parallelism
// concurrent contexts, and writes a metadata.json describing the results.
func RenderBatch(opts BatchOptions) (*BatchResult, error) {
	if opts.Parallelism <= 0 {
		opts.Parallelism = 4
	}
	out := opts.Stdout
	if out == nil {
		out = io.Discard
	}
	log := func(format string, args ...any) {
		if opts.Verbose {
			fmt.Fprintf(out, format+"\n", args...)
		}
	}

	absRoot, err := filepath.Abs(opts.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolving project root: %w", err)
	}

	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(absRoot, ".forge", "renders")
	} else if outputDir, err = filepath.Abs(outputDir); err != nil {
		return nil, fmt.Errorf("resolving output dir: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	log("checking analysis")
	analysis, err := ensureAnalysis(absRoot, opts.Refresh, log)
	if err != nil {
		return nil, err
	}

	var components []analyzer.NextjsComponent
	if !opts.PagesOnly {
		components = extractNextjsComponents(analysis)
	}
	log("found %d component(s)", len(components))

	var pages []analyzer.NextjsPage
	if !opts.ComponentsOnly {
		pages = extractNextjsPages(analysis)
	}
	log("found %d page(s)", len(pages))

	result := &BatchResult{
		Version:     batchMetadataVersion,
		RenderedAt:  time.Now().UTC().Format(time.RFC3339),
		ProjectRoot: absRoot,
		Components:  []ComponentRender{},
		Pages:       []PageRender{},
	}
	if len(components) == 0 && len(pages) == 0 {
		if err := writeMetadata(result, outputDir); err != nil {
			return nil, err
		}
		return result, nil
	}

	cacheDir, err := scriptsCacheDir()
	if err != nil {
		return nil, err
	}
	log("preparing renderer scripts in %s", cacheDir)
	if err := ensureScripts(cacheDir); err != nil {
		return nil, err
	}
	log("checking prerequisites")
	if err := CheckPrerequisites(cacheDir); err != nil {
		return nil, err
	}

	viewportW, viewportH, err := parseViewport(opts.Viewport)
	if err != nil {
		return nil, err
	}

	tsconfigPath := ""
	if _, err := os.Stat(filepath.Join(absRoot, "tsconfig.json")); err == nil {
		tsconfigPath = filepath.Join(absRoot, "tsconfig.json")
	}

	workDir, err := os.MkdirTemp("", "forge-render-batch-*")
	if err != nil {
		return nil, fmt.Errorf("creating work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	results := make([]ComponentRender, len(components))
	outNameToIndex := make(map[string]int, len(components))
	for i, comp := range components {
		results[i] = ComponentRender{Name: comp.Name, Path: comp.File}
		outNameToIndex[outputName(comp.File)] = i
	}

	var manifest []batchManifestItem
	var manifestMu sync.Mutex
	sem := make(chan struct{}, opts.Parallelism)
	var wg sync.WaitGroup

	for i, comp := range components {
		i, comp := i, comp
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			start := time.Now()
			absComponent := filepath.Join(absRoot, comp.File)
			outName := outputName(comp.File)

			extraction, err := extractTypes(cacheDir, absComponent)
			if err != nil {
				results[i].RenderStatus = "failed"
				results[i].RenderError = err.Error()
				results[i].RenderTimeMS = time.Since(start).Milliseconds()
				return
			}
			props := GenerateProps(extraction.Props)
			results[i].Props = RenderProps{Expected: extraction.Props, Synthesized: props}

			propsPath := filepath.Join(workDir, outName+".props.json")
			propsJSON, err := json.Marshal(props)
			if err != nil {
				results[i].RenderStatus = "failed"
				results[i].RenderError = fmt.Sprintf("marshaling props: %v", err)
				results[i].RenderTimeMS = time.Since(start).Milliseconds()
				return
			}
			if err := os.WriteFile(propsPath, propsJSON, 0o644); err != nil {
				results[i].RenderStatus = "failed"
				results[i].RenderError = fmt.Sprintf("writing props: %v", err)
				results[i].RenderTimeMS = time.Since(start).Milliseconds()
				return
			}

			bundlePath := filepath.Join(workDir, outName+".bundle.js")
			bundleArgs := []string{absComponent, propsPath, bundlePath}
			if tsconfigPath != "" {
				bundleArgs = append(bundleArgs, tsconfigPath)
			}
			if err := runNodeScript(cacheDir, "bundle.js", bundleArgs...); err != nil {
				results[i].RenderStatus = "failed"
				results[i].RenderError = err.Error()
				results[i].RenderTimeMS = time.Since(start).Milliseconds()
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
		log("rendering %d bundled component(s)", len(manifest))
		renderResults, err := invokeBatchRenderer(cacheDir, workDir, manifest, opts.Parallelism, viewportW, viewportH)
		if err != nil {
			return nil, fmt.Errorf("batch rendering: %w", err)
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

	componentCounts := BatchCounts{}
	var totalTimeMS int64
	for _, r := range results {
		componentCounts.Total++
		if r.RenderStatus == "success" {
			componentCounts.Succeeded++
		} else {
			componentCounts.Failed++
		}
		totalTimeMS += r.RenderTimeMS
	}
	result.Components = results

	log("rendering %d page(s)", len(pages))
	pageResults, err := renderPages(cacheDir, workDir, outputDir, absRoot, pages, analysis, opts.Parallelism, viewportW, viewportH, opts.PagesLimit)
	if err != nil {
		return nil, err
	}
	pageCounts := BatchCounts{}
	for _, r := range pageResults {
		pageCounts.Total++
		if r.RenderStatus == "success" {
			pageCounts.Succeeded++
		} else {
			pageCounts.Failed++
		}
		totalTimeMS += r.RenderTimeMS
	}
	result.Pages = pageResults

	result.Summary = BatchSummary{Components: componentCounts, Pages: pageCounts, TotalTimeMS: totalTimeMS}

	if err := writeMetadata(result, outputDir); err != nil {
		return nil, err
	}
	log("wrote %s", filepath.Join(outputDir, "metadata.json"))

	return result, nil
}

func writeMetadata(result *BatchResult, outputDir string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "metadata.json"), data, 0o644); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}
	return nil
}

// outputName flattens a component's project-relative path into a
// collision-safe base name (no extension) by replacing path separators
// with dashes, e.g. "components/shared/Card.tsx" -> "components-shared-Card".
func outputName(relPath string) string {
	trimmed := strings.TrimSuffix(relPath, filepath.Ext(relPath))
	return strings.ReplaceAll(trimmed, "/", "-")
}

// extractNextjsComponents reads analysis.Frameworks["nextjs"].Components.
// Frameworks is map[string]any: a freshly-run in-process analysis stores a
// *analyzer.NextjsInfo directly, while one read back from analysis.json
// decodes it as a generic map — a JSON re-encode normalizes both cases.
func extractNextjsComponents(analysis *analyzer.ProjectAnalysis) []analyzer.NextjsComponent {
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
	return info.Components
}

// ensureAnalysis reads .forge/analysis.json under root, running the
// analyzer and writing it fresh if missing or refresh is requested.
func ensureAnalysis(root string, refresh bool, log func(string, ...any)) (*analyzer.ProjectAnalysis, error) {
	analysisPath := filepath.Join(root, ".forge", "analysis.json")
	if !refresh {
		if data, err := os.ReadFile(analysisPath); err == nil {
			var analysis analyzer.ProjectAnalysis
			if err := json.Unmarshal(data, &analysis); err != nil {
				return nil, fmt.Errorf("parsing existing analysis.json: %w", err)
			}
			log("using existing analysis at %s", analysisPath)
			return &analysis, nil
		}
	}

	log("running analyzer on %s", root)
	analysis, warnings, err := analyzer.AnalyzeProject(root)
	if err != nil {
		return nil, fmt.Errorf("analyzing project: %w", err)
	}
	for _, w := range warnings {
		log("warning: %v", w)
	}

	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling analysis: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(analysisPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating .forge directory: %w", err)
	}
	if err := os.WriteFile(analysisPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("writing analysis.json: %w", err)
	}
	return analysis, nil
}

type batchManifestItem struct {
	OutName    string `json:"outName"`
	BundlePath string `json:"bundlePath"`
	OutputPath string `json:"outputPath"`
}

type batchViewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type batchManifest struct {
	Parallel int                 `json:"parallel"`
	Viewport batchViewport       `json:"viewport"`
	Items    []batchManifestItem `json:"items"`
}

type batchRenderResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	TimeMS int64  `json:"time"`
	Error  string `json:"error,omitempty"`
}

// invokeBatchRenderer runs render-batch.js once against manifest, which
// launches a single browser and screenshots every item across parallelism
// concurrent contexts.
func invokeBatchRenderer(cacheDir, workDir string, manifest []batchManifestItem, parallelism, viewportW, viewportH int) ([]batchRenderResult, error) {
	payload := batchManifest{
		Parallel: parallelism,
		Viewport: batchViewport{Width: viewportW, Height: viewportH},
		Items:    manifest,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling batch manifest: %w", err)
	}
	manifestPath := filepath.Join(workDir, "batch-manifest.json")
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("writing batch manifest: %w", err)
	}

	cmd := exec.Command("node", filepath.Join(cacheDir, "render-batch.js"), manifestPath)
	cmd.Dir = cacheDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w\n%s", err, stderr.String())
	}

	var results []batchRenderResult
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		return nil, fmt.Errorf("parsing batch render output: %w", err)
	}
	return results, nil
}
