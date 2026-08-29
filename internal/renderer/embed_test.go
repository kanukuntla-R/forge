package renderer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNeedsInstall(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(dir string)
		hash       string
		wantResult bool
	}{
		{
			name:       "fresh dir, nothing installed",
			setup:      func(dir string) {},
			hash:       "abc123",
			wantResult: true,
		},
		{
			name: "node_modules present but no hash file (pre-existing cache dir)",
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755)
			},
			hash:       "abc123",
			wantResult: true,
		},
		{
			name: "node_modules present, hash matches",
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755)
				os.WriteFile(filepath.Join(dir, ".package-hash"), []byte("abc123"), 0o644)
			},
			hash:       "abc123",
			wantResult: false,
		},
		{
			name: "node_modules present, hash stale",
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755)
				os.WriteFile(filepath.Join(dir, ".package-hash"), []byte("old-hash"), 0o644)
			},
			hash:       "abc123",
			wantResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)
			got := needsInstall(dir, tt.hash)
			if got != tt.wantResult {
				t.Errorf("needsInstall() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}
