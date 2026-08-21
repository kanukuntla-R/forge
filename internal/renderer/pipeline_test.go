package renderer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseViewport(t *testing.T) {
	tests := []struct {
		in      string
		wantW   int
		wantH   int
		wantErr bool
	}{
		{in: "", wantW: 800, wantH: 600},
		{in: "800x600", wantW: 800, wantH: 600},
		{in: "1024x768", wantW: 1024, wantH: 768},
		{in: "bad", wantErr: true},
		{in: "800xNaN", wantErr: true},
	}
	for _, tt := range tests {
		w, h, err := parseViewport(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseViewport(%q): want error, got none", tt.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseViewport(%q): unexpected error: %v", tt.in, err)
		}
		if w != tt.wantW || h != tt.wantH {
			t.Errorf("parseViewport(%q) = %d,%d, want %d,%d", tt.in, w, h, tt.wantW, tt.wantH)
		}
	}
}

// TestRenderIntegration proves the full pipeline (analyze → generate →
// bundle → render) end-to-end for each M13.1 fixture. Requires Node.js,
// npm, and Playwright's Chromium browser; skipped when node isn't on PATH.
func TestRenderIntegration(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH, skipping renderer integration test")
	}

	fixtures := []string{"SimpleCard.tsx", "PropCard.tsx", "CardWithButton.tsx"}
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			outPath := filepath.Join(t.TempDir(), "out.png")
			got, err := Render(filepath.Join("testdata", fixture), Options{Output: outPath})
			if err != nil {
				t.Fatalf("Render(%s) failed: %v", fixture, err)
			}
			if got != outPath {
				t.Errorf("Render returned %q, want %q", got, outPath)
			}
			assertPNG(t, outPath)
		})
	}
}

func assertPNG(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading output PNG: %v", err)
	}
	if len(data) < 8 {
		t.Fatalf("output file too small to be a PNG: %d bytes", len(data))
	}
	pngMagic := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	for i, b := range pngMagic {
		if data[i] != b {
			t.Fatalf("output file is not a valid PNG (bad magic bytes)")
		}
	}
}
