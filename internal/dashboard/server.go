package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/kanukuntla-r/forge/internal/analyzer"
)

const (
	portRangeStart     = 5050
	portRangeEnd       = 5100
	defaultIdleTimeout = 30 * time.Second
)

// Server represents a running dashboard HTTP server.
type Server struct {
	projectRoot       string
	analysisPath      string
	listener          net.Listener
	port              int
	httpServer        *http.Server
	hub               *Hub
	watcher           *Watcher
	analyzing         atomic.Bool
	needsRerun        atomic.Bool
	idleShutdownCh    chan struct{}
	idleTimeout       time.Duration
	idleCheckInterval time.Duration // default 5s; lower in tests
}

// NewServer binds to the first available port in [5050, 5100) and registers
// HTTP routes. The caller must call Start to begin serving.
func NewServer(projectRoot, analysisPath string) (*Server, error) {
	if _, err := os.Stat(analysisPath); err != nil {
		return nil, fmt.Errorf("analysis file not found at %q: %w", analysisPath, err)
	}

	port, ln, err := findAvailablePort()
	if err != nil {
		return nil, err
	}

	hub := NewHub()
	s := &Server{
		projectRoot:    projectRoot,
		analysisPath:   analysisPath,
		listener:       ln,
		port:           port,
		hub:            hub,
		idleShutdownCh: make(chan struct{}, 1),
		idleTimeout:    defaultIdleTimeout,
	}

	watcher, err := NewWatcher(projectRoot, s.runAnalysis)
	if err != nil {
		ln.Close()
		return nil, fmt.Errorf("creating file watcher: %w", err)
	}
	s.watcher = watcher

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/analysis", s.handleAnalysis)
	mux.HandleFunc("/ws", hub.ServeWS)
	mux.Handle("/", http.FileServer(staticFileSystem()))

	s.httpServer = &http.Server{Handler: mux}
	return s, nil
}

// Start serves requests until ctx is cancelled, then shuts down gracefully.
func (s *Server) Start(ctx context.Context) error {
	if err := s.watcher.Start(); err != nil {
		return fmt.Errorf("starting file watcher: %w", err)
	}
	defer s.watcher.Stop()

	go s.runIdleMonitor(ctx)

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		return s.httpServer.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

// IdleShutdownCh returns a channel that receives when the idle timer expires.
func (s *Server) IdleShutdownCh() <-chan struct{} {
	return s.idleShutdownCh
}

// URL returns the address the server is listening on.
func (s *Server) URL() string {
	return fmt.Sprintf("http://localhost:%d", s.port)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
}

func (s *Server) handleAnalysis(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(s.analysisPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("reading analysis: %v", err), http.StatusInternalServerError)
		return
	}
	if !json.Valid(data) {
		http.Error(w, "analysis.json is not valid JSON", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data) //nolint:errcheck
}

// runAnalysis triggers a re-analysis using skip-in-flight + rerun flag.
func (s *Server) runAnalysis() {
	if !s.analyzing.CompareAndSwap(false, true) {
		s.needsRerun.Store(true)
		return
	}

	go func() {
		defer func() {
			s.analyzing.Store(false)
			if s.needsRerun.CompareAndSwap(true, false) {
				s.runAnalysis()
			}
		}()

		if err := s.analyzeAndWrite(); err != nil {
			log.Printf("dashboard: re-analysis failed: %v", err)
			return
		}

		s.hub.BroadcastAnalysisUpdate()
	}()
}

// analyzeAndWrite runs the full analysis pipeline and writes analysis.json.
func (s *Server) analyzeAndWrite() error {
	analysis, warnings, err := analyzer.AnalyzeProject(s.projectRoot)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		log.Printf("dashboard: warning: %v", w)
	}
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling analysis: %w", err)
	}
	return os.WriteFile(s.analysisPath, data, 0644)
}

// runIdleMonitor signals idleShutdownCh when no clients have been connected
// for at least s.idleTimeout.
func (s *Server) runIdleMonitor(ctx context.Context) {
	interval := s.idleCheckInterval
	if interval == 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.hub.idleDuration() >= s.idleTimeout {
				log.Println("dashboard: no active connections, shutting down")
				select {
				case s.idleShutdownCh <- struct{}{}:
				default:
				}
				return
			}
		}
	}
}

// findAvailablePort tries ports starting from portRangeStart up to portRangeEnd.
// Returns the port number and an open listener; caller owns the listener.
func findAvailablePort() (int, net.Listener, error) {
	for port := portRangeStart; port < portRangeEnd; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			return port, ln, nil
		}
	}
	return 0, nil, fmt.Errorf("no available port in range %d–%d", portRangeStart, portRangeEnd)
}

// OpenBrowser opens url in the system default browser.
// Failure is non-fatal; the caller should print the URL for manual access.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default: // linux, bsd, etc.
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// staticFileSystem returns an fs.FS rooted at the embedded static/ directory.
func staticFileSystem() http.FileSystem {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// staticFS is always valid since it's embedded at compile time.
		panic(fmt.Sprintf("dashboard: bad embed: %v", err))
	}
	return http.FS(sub)
}
