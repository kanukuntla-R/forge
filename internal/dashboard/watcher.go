package dashboard

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

// Watcher monitors a project directory for file changes and triggers a
// callback after debouncing. Thread-safe. Use Start to begin watching;
// Stop to clean up.
type Watcher struct {
	root      string
	debounce  time.Duration
	onChange  func()
	fsWatcher *fsnotify.Watcher
	timer     *time.Timer
	timerMu   sync.Mutex
	done      chan struct{}
	stopOnce  sync.Once
}

// NewWatcher creates a watcher rooted at the given project directory.
// onChange is invoked from a goroutine after 300ms of no file events.
func NewWatcher(root string, onChange func()) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating file watcher: %w", err)
	}
	return &Watcher{
		root:      root,
		debounce:  300 * time.Millisecond,
		onChange:  onChange,
		fsWatcher: fw,
		done:      make(chan struct{}),
	}, nil
}

// Start begins watching. Returns immediately; events are handled in a goroutine.
// Walks root and registers all non-ignored subdirectories with fsnotify.
func (w *Watcher) Start() error {
	if err := filepath.WalkDir(w.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !d.IsDir() {
			return nil
		}
		base := d.Name()
		if path != w.root {
			if analyzer.IsIgnoredDir(base) {
				return filepath.SkipDir
			}
			if strings.HasPrefix(base, ".") && base != ".forge" {
				return filepath.SkipDir
			}
		}
		return w.fsWatcher.Add(path)
	}); err != nil {
		w.fsWatcher.Close()
		return fmt.Errorf("registering directories: %w", err)
	}

	go w.run()
	return nil
}

// Stop stops watching and cleans up. Safe to call multiple times.
func (w *Watcher) Stop() {
	w.stopOnce.Do(func() {
		close(w.done)
		w.fsWatcher.Close()
		w.timerMu.Lock()
		if w.timer != nil {
			w.timer.Stop()
		}
		w.timerMu.Unlock()
	})
}

func (w *Watcher) run() {
	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(ev)
		case _, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) handleEvent(ev fsnotify.Event) {
	base := filepath.Base(ev.Name)
	// Skip editor temp/swap files.
	if strings.HasSuffix(base, ".swp") || strings.HasSuffix(base, "~") || strings.HasPrefix(base, ".#") {
		return
	}

	// Dynamically watch newly created subdirectories.
	if ev.Op&fsnotify.Create == fsnotify.Create {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			if !analyzer.IsIgnoredDir(base) && !(strings.HasPrefix(base, ".") && base != ".forge") {
				w.fsWatcher.Add(ev.Name) //nolint:errcheck
			}
		}
	}

	// Reset debounce timer.
	w.timerMu.Lock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(w.debounce, w.onChange)
	w.timerMu.Unlock()
}
