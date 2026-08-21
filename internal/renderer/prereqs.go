package renderer

import (
	"fmt"
	"os/exec"
)

// CheckPrerequisites verifies node is on PATH and that playwright resolves
// from cacheDir's node_modules. It does NOT verify a browser is downloaded —
// npx/node paths differ per OS, so that check is left to Playwright itself,
// whose own launch error already names the exact install command. That
// failure surfaces through the render stage's captured stderr instead.
func CheckPrerequisites(cacheDir string) error {
	if _, err := exec.LookPath("node"); err != nil {
		return fmt.Errorf("forge render: node.js not found on PATH; install Node.js 18+ from https://nodejs.org")
	}

	cmd := exec.Command("npx", "--yes", "playwright", "--version")
	cmd.Dir = cacheDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("forge render: playwright not available in %s: %w\n%s", cacheDir, err, out)
	}
	return nil
}
