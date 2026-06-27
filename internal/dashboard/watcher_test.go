package dashboard

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWatcherDetectsFileChange(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	count := 0
	w, err := NewWatcher(dir, func() {
		mu.Lock()
		count++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(filepath.Join(dir, "test.ts"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	mu.Lock()
	got := count
	mu.Unlock()
	if got == 0 {
		t.Error("callback was not called after file change")
	}
}

func TestWatcherDebounces(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	count := 0
	w, err := NewWatcher(dir, func() {
		mu.Lock()
		count++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	f := filepath.Join(dir, "test.ts")
	os.WriteFile(f, []byte("v1"), 0644) //nolint:errcheck
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(f, []byte("v2"), 0644) //nolint:errcheck

	// Debounce is 300ms; wait well past it.
	time.Sleep(700 * time.Millisecond)

	mu.Lock()
	got := count
	mu.Unlock()
	if got != 1 {
		t.Errorf("callback called %d times, want 1 (debounce failed)", got)
	}
}

func TestWatcherIgnoresNodeModules(t *testing.T) {
	dir := t.TempDir()
	nmDir := filepath.Join(dir, "node_modules")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var mu sync.Mutex
	count := 0
	w, err := NewWatcher(dir, func() {
		mu.Lock()
		count++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	os.WriteFile(filepath.Join(nmDir, "something.js"), []byte("hello"), 0644) //nolint:errcheck
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	got := count
	mu.Unlock()
	if got != 0 {
		t.Errorf("callback fired %d times for node_modules change, want 0", got)
	}
}

func TestWatcherWatchesSubdirectories(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "app", "api")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var mu sync.Mutex
	count := 0
	w, err := NewWatcher(dir, func() {
		mu.Lock()
		count++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	os.WriteFile(filepath.Join(subDir, "route.ts"), []byte("hello"), 0644) //nolint:errcheck
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	got := count
	mu.Unlock()
	if got == 0 {
		t.Error("callback not called for subdirectory file change")
	}
}

func TestWatcherDetectsNewSubdirectory(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	count := 0
	w, err := NewWatcher(dir, func() {
		mu.Lock()
		count++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	// Create a new directory, then write a file inside it.
	newDir := filepath.Join(dir, "newdir")
	os.Mkdir(newDir, 0755) //nolint:errcheck
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(filepath.Join(newDir, "file.ts"), []byte("hello"), 0644) //nolint:errcheck

	time.Sleep(600 * time.Millisecond)

	mu.Lock()
	got := count
	mu.Unlock()
	if got == 0 {
		t.Error("callback not called after file created in new subdirectory")
	}
}
