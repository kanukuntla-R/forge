// Package renderer implements the M13.1 component rendering pipeline:
// extract prop types, generate synthetic props, bundle with esbuild, and
// screenshot with Playwright.
package renderer

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
)

//go:embed scripts
var scriptsFS embed.FS

// scriptsCacheDir returns the persistent directory where renderer scripts
// and their npm dependencies live, so `npm install` only runs once per
// machine rather than on every `forge render` invocation.
func scriptsCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolving cache dir: %w", err)
	}
	return filepath.Join(base, "forge", "renderer"), nil
}

// ensureScripts writes the embedded scripts into dir (overwriting any
// previous copy, so the cache always matches the running binary) and
// installs npm dependencies if node_modules is missing.
func ensureScripts(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating scripts cache dir: %w", err)
	}

	entries, err := scriptsFS.ReadDir("scripts")
	if err != nil {
		return fmt.Errorf("reading embedded scripts: %w", err)
	}
	for _, e := range entries {
		content, err := scriptsFS.ReadFile(path.Join("scripts", e.Name()))
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), content, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", e.Name(), err)
		}
	}

	if _, err := os.Stat(filepath.Join(dir, "node_modules")); err == nil {
		return nil
	}

	cmd := exec.Command("npm", "install")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("installing renderer script dependencies in %s: %w\n%s", dir, err, out)
	}
	return nil
}
