package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
)

const (
	portRangeStart = 5050
	portRangeEnd   = 5100
)

// Server represents a running dashboard HTTP server.
type Server struct {
	analysisPath string
	listener     net.Listener
	port         int
	httpServer   *http.Server
}

// NewServer binds to the first available port in [5050, 5100) and registers
// HTTP routes. The caller must call Start to begin serving.
func NewServer(analysisPath string) (*Server, error) {
	if _, err := os.Stat(analysisPath); err != nil {
		return nil, fmt.Errorf("analysis file not found at %q: %w", analysisPath, err)
	}

	port, ln, err := findAvailablePort()
	if err != nil {
		return nil, err
	}

	s := &Server{
		analysisPath: analysisPath,
		listener:     ln,
		port:         port,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/analysis", s.handleAnalysis)
	mux.Handle("/", http.FileServer(staticFileSystem()))

	s.httpServer = &http.Server{Handler: mux}
	return s, nil
}

// Start serves requests until ctx is cancelled, then shuts down gracefully.
func (s *Server) Start(ctx context.Context) error {
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
	// Validate JSON before serving — if the file is corrupt, return a clear error.
	if !json.Valid(data) {
		http.Error(w, "analysis.json is not valid JSON", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data) //nolint:errcheck
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
