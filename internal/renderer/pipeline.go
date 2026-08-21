package renderer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultViewport = "800x600"

// Options configures a single Render call.
type Options struct {
	Output   string    // output PNG path; default .forge/renders/<Name>.png
	Viewport string    // "WxH"; default "800x600"
	Verbose  bool      // log each pipeline stage to Stdout
	Stdout   io.Writer // verbose stage log destination; default io.Discard
}

// Render runs the 4-stage pipeline (analyze, generate, bundle, render) for a
// single component and returns the absolute path to the produced PNG.
func Render(componentPath string, opts Options) (string, error) {
	out := opts.Stdout
	if out == nil {
		out = io.Discard
	}
	log := func(format string, args ...any) {
		if opts.Verbose {
			fmt.Fprintf(out, format+"\n", args...)
		}
	}

	absComponent, err := filepath.Abs(componentPath)
	if err != nil {
		return "", fmt.Errorf("resolving component path: %w", err)
	}
	if _, err := os.Stat(absComponent); err != nil {
		return "", fmt.Errorf("component not found: %w", err)
	}

	viewportW, viewportH, err := parseViewport(opts.Viewport)
	if err != nil {
		return "", err
	}

	absOutput, err := filepath.Abs(resolveOutputPath(opts.Output, absComponent))
	if err != nil {
		return "", fmt.Errorf("resolving output path: %w", err)
	}

	cacheDir, err := scriptsCacheDir()
	if err != nil {
		return "", err
	}
	log("preparing renderer scripts in %s", cacheDir)
	if err := ensureScripts(cacheDir); err != nil {
		return "", err
	}

	log("checking prerequisites")
	if err := CheckPrerequisites(cacheDir); err != nil {
		return "", err
	}

	workDir, err := os.MkdirTemp("", "forge-render-*")
	if err != nil {
		return "", fmt.Errorf("creating work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	log("analyzing %s", absComponent)
	extraction, err := extractTypes(cacheDir, absComponent)
	if err != nil {
		return "", fmt.Errorf("analyzing component: %w", err)
	}
	for _, w := range extraction.Warnings {
		log("warning: %s", w)
	}

	log("generating synthetic props")
	props := GenerateProps(extraction.Props)
	propsPath := filepath.Join(workDir, "props.json")
	propsJSON, err := json.Marshal(props)
	if err != nil {
		return "", fmt.Errorf("marshaling props: %w", err)
	}
	if err := os.WriteFile(propsPath, propsJSON, 0o644); err != nil {
		return "", fmt.Errorf("writing props.json: %w", err)
	}

	log("bundling component")
	bundlePath := filepath.Join(workDir, "bundle.js")
	if err := runNodeScript(cacheDir, "bundle.js", absComponent, propsPath, bundlePath); err != nil {
		return "", fmt.Errorf("bundling component: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(absOutput), 0o755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	log("rendering screenshot")
	if err := runNodeScript(cacheDir, "render.js", bundlePath, absOutput, strconv.Itoa(viewportW), strconv.Itoa(viewportH)); err != nil {
		return "", fmt.Errorf("rendering screenshot: %w", err)
	}

	log("wrote %s", absOutput)
	return absOutput, nil
}

func resolveOutputPath(output, absComponent string) string {
	if output != "" {
		return output
	}
	name := strings.TrimSuffix(filepath.Base(absComponent), filepath.Ext(absComponent))
	return filepath.Join(".forge", "renders", name+".png")
}

func parseViewport(v string) (int, int, error) {
	if v == "" {
		v = defaultViewport
	}
	parts := strings.SplitN(v, "x", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid viewport %q; expected WxH (e.g. 800x600)", v)
	}
	w, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid viewport width %q: %w", parts[0], err)
	}
	h, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid viewport height %q: %w", parts[1], err)
	}
	return w, h, nil
}

// extractTypes runs scripts/types.js against componentPath and parses its
// JSON stdout into a TypeExtraction.
func extractTypes(cacheDir, componentPath string) (*TypeExtraction, error) {
	cmd := exec.Command("node", filepath.Join(cacheDir, "types.js"), componentPath)
	cmd.Dir = cacheDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w\n%s", err, stderr.String())
	}
	var extraction TypeExtraction
	if err := json.Unmarshal(stdout.Bytes(), &extraction); err != nil {
		return nil, fmt.Errorf("parsing type extraction output: %w", err)
	}
	return &extraction, nil
}

// runNodeScript invokes an embedded script from cacheDir with args, capturing
// stderr so failures carry the Node/esbuild/Playwright error message.
func runNodeScript(cacheDir, script string, args ...string) error {
	cmdArgs := append([]string{filepath.Join(cacheDir, script)}, args...)
	cmd := exec.Command("node", cmdArgs...)
	cmd.Dir = cacheDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w\n%s", err, stderr.String())
	}
	return nil
}
